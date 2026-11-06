package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/slidebolt/plugin-asterisk/internal/hostapi"
	"github.com/slidebolt/plugin-asterisk/internal/mapping"
	contract "github.com/slidebolt/sb-contract"
	domain "github.com/slidebolt/sb-domain"
	messenger "github.com/slidebolt/sb-messenger-sdk"
	storage "github.com/slidebolt/sb-storage-sdk"
)

const PluginID = mapping.PluginID

const commandTimeout = 15 * time.Second

// App is the importable runtime for the plugin-asterisk binary. It is a thin
// translator between asterisk-host and SlideBolt: every device comes from the
// host, and it holds no call state or behavior of its own.
type App struct {
	msg    messenger.Messenger
	store  storage.Storage
	cmds   *messenger.Commands
	subs   []messenger.Subscription
	host   *hostapi.Client
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu        sync.Mutex
	endpoints map[string]hostapi.Endpoint // by device ID
	saved     map[string]string           // last saved JSON by key
	server    hostapi.Server
}

func New() *App {
	return &App{endpoints: map[string]hostapi.Endpoint{}, saved: map[string]string{}}
}

func (a *App) Hello() contract.HelloResponse {
	return contract.HelloResponse{
		ID:              PluginID,
		Kind:            contract.KindPlugin,
		ContractVersion: contract.ContractVersion,
		DependsOn:       []string{"messenger", "storage"},
	}
}

// OnStart stays up when the host is unreachable or unconfigured. A failed start
// aborts sb-manager's startAll, which would stop every other binary.
func (a *App) OnStart(deps map[string]json.RawMessage) (json.RawMessage, error) {
	msg, err := messenger.Connect(deps)
	if err != nil {
		return nil, fmt.Errorf("connect messenger: %w", err)
	}
	a.msg = msg

	store, err := storage.Connect(deps)
	if err != nil {
		return nil, fmt.Errorf("connect storage: %w", err)
	}
	a.store = store

	a.cmds = messenger.NewCommands(msg, lookupCommand)
	sub, err := a.cmds.Receive(PluginID+".>", a.handleCommand)
	if err != nil {
		return nil, fmt.Errorf("subscribe commands: %w", err)
	}
	a.subs = append(a.subs, sub)

	a.ctx, a.cancel = context.WithCancel(context.Background())
	host, err := hostapi.New(os.Getenv("ASTERISK_HOST_URL"), os.Getenv("ASTERISK_HOST_TOKEN"))
	if err != nil {
		log.Printf("plugin-asterisk: disabled: %v", err)
		a.applyServer(hostapi.Server{Error: err.Error()})
		return nil, nil
	}
	a.host = host
	a.wg.Add(1)
	go a.syncLoop()

	log.Println("plugin-asterisk: started")
	return nil, nil
}

func (a *App) OnShutdown() error {
	if a.cancel != nil {
		a.cancel()
	}
	a.wg.Wait()
	for _, sub := range a.subs {
		sub.Unsubscribe()
	}
	if a.store != nil {
		a.store.Close()
	}
	if a.msg != nil {
		a.msg.Close()
	}
	return nil
}

// syncLoop mirrors the host: load a snapshot, then follow events, and start
// again from a snapshot if the event stream cannot be resumed.
func (a *App) syncLoop() {
	defer a.wg.Done()
	backoff := time.Second
	for a.ctx.Err() == nil {
		snapshot, err := a.host.Snapshot(a.ctx)
		if err != nil {
			a.markHostOffline(err)
			if !wait(a.ctx, backoff) {
				return
			}
			backoff = min(backoff*2, 30*time.Second)
			continue
		}
		backoff = time.Second
		a.applySnapshot(snapshot)
		cursor := snapshot.Sequence
		err = a.host.Events(a.ctx, snapshot.StreamID, cursor, func(event hostapi.Event) error {
			if event.StreamID != "" && event.StreamID != snapshot.StreamID {
				return hostapi.ErrCursorExpired
			}
			if event.ID <= cursor {
				return nil
			}
			a.applyEvent(event)
			cursor = event.ID
			return nil
		})
		if a.ctx.Err() != nil {
			return
		}
		if !errors.Is(err, hostapi.ErrCursorExpired) {
			a.markHostOffline(err)
			if !wait(a.ctx, backoff) {
				return
			}
		}
	}
}

func wait(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}

func (a *App) markHostOffline(err error) {
	log.Printf("plugin-asterisk: host unavailable: %v", err)
	a.mu.Lock()
	server := a.server
	a.mu.Unlock()
	server.Connected = false
	server.Error = "asterisk-host unavailable"
	a.applyServer(server)
}

func (a *App) applySnapshot(s hostapi.Snapshot) {
	current := map[string]bool{}
	a.applyServer(s.Server)
	current[mapping.ServerDevice] = true
	for _, e := range s.Endpoints {
		a.applyEndpoint(e)
		current[mapping.EndpointDevice(e)] = true
	}
	for _, c := range s.Calls {
		a.applyCall(c)
		current[mapping.CallDevice(c.ID)] = true
	}
	a.removeStale(current)
}

func (a *App) applyEvent(ev hostapi.Event) {
	var err error
	switch ev.Type {
	case hostapi.EventServer:
		var s hostapi.Server
		if err = json.Unmarshal(ev.Data, &s); err == nil {
			a.applyServer(s)
		}
	case hostapi.EventEndpoint:
		var e hostapi.Endpoint
		if err = json.Unmarshal(ev.Data, &e); err == nil {
			a.applyEndpoint(e)
		}
	case hostapi.EventCall:
		var c hostapi.Call
		if err = json.Unmarshal(ev.Data, &c); err == nil {
			a.applyCall(c)
		}
	case hostapi.EventRemoved:
		var r struct {
			ID string `json:"id"`
		}
		if err = json.Unmarshal(ev.Data, &r); err == nil {
			a.removeDevice(mapping.CallDevice(r.ID))
		}
	}
	if err != nil {
		log.Printf("plugin-asterisk: decode %s: %v", ev.Type, err)
	}
}

func (a *App) applyServer(s hostapi.Server) {
	a.mu.Lock()
	a.server = s
	a.mu.Unlock()
	a.saveItem(mapping.Server(s))
}

func (a *App) applyEndpoint(e hostapi.Endpoint) {
	a.mu.Lock()
	a.endpoints[mapping.EndpointDevice(e)] = e
	a.mu.Unlock()
	a.saveItem(mapping.Endpoint(e))
}

func (a *App) applyCall(c hostapi.Call) {
	a.saveItem(mapping.Call(c))
}

// saveItem stores what changed. Saving only changes matters because automations
// trigger on every save.
func (a *App) saveItem(item mapping.Item) {
	if a.changed(item.Device.Key(), item.Device) {
		if err := a.store.Save(item.Device); err != nil {
			log.Printf("plugin-asterisk: save device %s: %v", item.Device.Key(), err)
		}
	}
	for _, e := range item.Entities {
		if !a.changed(e.Key(), e) {
			continue
		}
		if err := a.store.Save(e); err != nil {
			log.Printf("plugin-asterisk: save entity %s: %v", e.Key(), err)
		}
	}
}

func (a *App) changed(key string, v any) bool {
	data, err := json.Marshal(v)
	if err != nil {
		return true
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.saved[key] == string(data) {
		return false
	}
	a.saved[key] = string(data)
	return true
}

// removeStale deletes devices the host no longer reports, including anything an
// earlier version of this plugin stored.
func (a *App) removeStale(current map[string]bool) {
	entries, err := a.store.Search(PluginID + ".>")
	if err != nil {
		log.Printf("plugin-asterisk: search: %v", err)
		return
	}
	stale := map[string]bool{}
	for _, entry := range entries {
		parts := strings.Split(entry.Key, ".")
		if len(parts) >= 2 && !current[parts[1]] {
			stale[parts[1]] = true
		}
	}
	for device := range stale {
		a.removeDevice(device)
	}
}

func (a *App) removeDevice(device string) {
	entries, err := a.store.Search(PluginID + "." + device + ".>")
	if err != nil {
		log.Printf("plugin-asterisk: search %s: %v", device, err)
		return
	}
	for _, entry := range entries {
		parts := strings.SplitN(entry.Key, ".", 3)
		if len(parts) != 3 {
			continue
		}
		key := domain.EntityKey{Plugin: PluginID, DeviceID: device, ID: parts[2]}
		if err := a.store.Delete(key); err != nil {
			log.Printf("plugin-asterisk: delete %s: %v", entry.Key, err)
		}
		a.forget(entry.Key)
	}
	key := domain.DeviceKey{Plugin: PluginID, ID: device}
	if err := a.store.Delete(key); err != nil {
		log.Printf("plugin-asterisk: delete device %s: %v", device, err)
	}
	a.forget(key.Key())
	a.mu.Lock()
	delete(a.endpoints, device)
	a.mu.Unlock()
}

func (a *App) forget(key string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.saved, key)
}
