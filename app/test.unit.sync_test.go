package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/slidebolt/plugin-asterisk/internal/hostapi"
	domain "github.com/slidebolt/sb-domain"
	messenger "github.com/slidebolt/sb-messenger-sdk"
	storage "github.com/slidebolt/sb-storage-sdk"
	testkit "github.com/slidebolt/sb-testkit"
)

// fakeHost implements the asterisk-host API the plugin uses.
type fakeHost struct {
	srv *httptest.Server

	mu       sync.Mutex
	snapshot hostapi.Snapshot
	events   []hostapi.Event
	seq      int64
	requests []string
	bodies   map[string]string
	wake     chan struct{}
	expired  bool
}

func newFakeHost(t *testing.T) *fakeHost {
	t.Helper()
	f := &fakeHost{bodies: map[string]string{}, wake: make(chan struct{})}
	f.snapshot = hostapi.Snapshot{
		APIVersion: 1, StreamID: "s1",
		Server: hostapi.Server{Connected: true, Version: "22.7.0"},
		Endpoints: []hostapi.Endpoint{
			{ID: "3000", Name: "Intercom", Kind: "extension", Role: "intercom", Status: "online",
				Features: hostapi.Features{AutoAnswer: true, Listen: "on_command"},
				Stream:   &hostapi.Stream{Status: "idle", URL: "http://media/x"}},
			{ID: "201", Name: "Kitchen Phone", Kind: "extension", Status: "offline", Features: hostapi.Features{Listen: "never"}},
			{ID: "float-10000", Name: "float-10000", Kind: "extension", Status: "offline", Features: hostapi.Features{Listen: "never"}},
			{ID: "trunk1", Name: "trunk1", Kind: "trunk", Status: "online", Features: hostapi.Features{Listen: "never"}},
		},
		Calls: []hostapi.Call{},
	}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(func() {
		f.mu.Lock()
		close(f.wake)
		f.wake = nil
		f.mu.Unlock()
		f.srv.CloseClientConnections()
		f.srv.Close()
	})
	return f
}

func (f *fakeHost) serve(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer test-token" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	switch {
	case r.URL.Path == "/v1/snapshot":
		f.mu.Lock()
		s := f.snapshot
		s.Sequence = f.seq
		f.mu.Unlock()
		json.NewEncoder(w).Encode(s)
	case r.URL.Path == "/v1/events":
		f.serveEvents(w, r)
	default:
		var body strings.Builder
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		body.Write(buf[:n])
		f.mu.Lock()
		call := r.Method + " " + r.URL.Path
		f.requests = append(f.requests, call)
		f.bodies[call] = body.String()
		f.mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
	}
}

func (f *fakeHost) serveEvents(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	expired := f.expired
	f.expired = false
	f.mu.Unlock()
	if expired {
		w.WriteHeader(http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusOK)
	rc := http.NewResponseController(w)
	rc.Flush()
	var after int64
	fmt.Sscan(r.URL.Query().Get("after"), &after)
	for {
		f.mu.Lock()
		wake := f.wake
		var send []hostapi.Event
		for _, ev := range f.events {
			if ev.ID > after {
				send = append(send, ev)
			}
		}
		f.mu.Unlock()
		for _, ev := range send {
			data, _ := json.Marshal(ev)
			fmt.Fprintf(w, "id: %d\ndata: %s\n\n", ev.ID, data)
			after = ev.ID
		}
		rc.Flush()
		if wake == nil {
			return
		}
		select {
		case <-wake:
		case <-r.Context().Done():
			return
		}
	}
}

func (f *fakeHost) emit(typ string, data any) {
	raw, _ := json.Marshal(data)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seq++
	f.events = append(f.events, hostapi.Event{ID: f.seq, StreamID: "s1", Type: typ, Data: raw})
	close(f.wake)
	f.wake = make(chan struct{})
}

func (f *fakeHost) called(call string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.requests {
		if c == call {
			return true
		}
	}
	return false
}

func (f *fakeHost) body(call string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.bodies[call]
}

type syncHarness struct {
	host  *fakeHost
	store storage.Storage
	cmds  *messenger.Commands
}

func startSync(t *testing.T, seed func(storage.Storage)) *syncHarness {
	t.Helper()
	env := testkit.NewTestEnv(t)
	env.Start("messenger")
	env.Start("storage")
	if seed != nil {
		seed(env.Storage())
	}
	f := newFakeHost(t)
	t.Setenv("ASTERISK_HOST_URL", f.srv.URL)
	t.Setenv("ASTERISK_HOST_TOKEN", "test-token")
	a := New()
	if _, err := a.OnStart(map[string]json.RawMessage{"messenger": env.MessengerPayload()}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.OnShutdown() })
	h := &syncHarness{host: f, store: env.Storage(), cmds: messenger.NewCommands(env.Messenger(), domain.LookupCommand)}
	soon(t, "first snapshot", func() bool { return h.exists("trunk_trunk1", "registration") })
	return h
}

func soon(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func (h *syncHarness) exists(device, entity string) bool {
	_, err := h.store.Get(domain.EntityKey{Plugin: PluginID, DeviceID: device, ID: entity})
	return err == nil
}

func (h *syncHarness) get(t *testing.T, device, entity string) domain.Entity {
	t.Helper()
	raw, err := h.store.Get(domain.EntityKey{Plugin: PluginID, DeviceID: device, ID: entity})
	if err != nil {
		t.Fatalf("get %s.%s: %v", device, entity, err)
	}
	var e domain.Entity
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatal(err)
	}
	return e
}

func (h *syncHarness) send(t *testing.T, device, entity string, cmd messenger.Action) {
	t.Helper()
	if err := h.cmds.Send(domain.EntityKey{Plugin: PluginID, DeviceID: device, ID: entity}, cmd); err != nil {
		t.Fatal(err)
	}
}

func TestSync_DiscoveredDevicesBecomeEntities(t *testing.T) {
	h := startSync(t, nil)

	intercom := h.get(t, "ext_3000", "registration")
	state := intercom.State.(domain.SIPEndpoint)
	if intercom.Type != "sip_endpoint" || intercom.Name != "Intercom" || !state.Registered || state.Role != "intercom" {
		t.Fatalf("intercom = %+v", intercom)
	}
	if !h.exists("ext_3000", "availability") || !h.exists("ext_3000", "call-state") {
		t.Fatal("intercom is missing entities")
	}
	audio := h.get(t, "ext_3000", "audio").State.(domain.MediaStream)
	if audio.URL != "http://media/x" || audio.Status != "idle" {
		t.Fatalf("audio = %+v", audio)
	}
	if h.exists("ext_201", "audio") {
		t.Fatal("an endpoint that forbids listening must not get an audio entity")
	}
	if !h.exists("ext_float_10000", "registration") {
		t.Fatal("unassigned extensions are real and should come through")
	}
	trunk := h.get(t, "trunk_trunk1", "registration")
	if trunk.Type != "sip_trunk" || !trunk.State.(domain.SIPTrunk).Registered {
		t.Fatalf("trunk = %+v", trunk)
	}
	if status := h.get(t, "server", "status").State.(domain.PBX); status.Version != "22.7.0" || !status.Connected {
		t.Fatalf("server = %+v", status)
	}
}

func TestSync_SnapshotRemovesDevicesTheHostDoesNotReport(t *testing.T) {
	h := startSync(t, func(store storage.Storage) {
		store.Save(domain.Device{ID: "ext_old", Plugin: PluginID, Name: "Old"})
		store.Save(domain.Entity{ID: "registration", Plugin: PluginID, DeviceID: "ext_old", Type: "sip_endpoint", State: domain.SIPEndpoint{}})
	})
	soon(t, "stale device removal", func() bool { return !h.exists("ext_old", "registration") })
}

func TestSync_EventsCreateUpdateAndRemoveCalls(t *testing.T) {
	h := startSync(t, nil)
	call := hostapi.Call{ID: "front-door", Status: "active", Participants: []hostapi.Participant{
		{ID: "201", Target: "201", Status: "ringing"},
	}}
	h.host.emit(hostapi.EventCall, call)
	soon(t, "participant", func() bool { return h.exists("call_front-door", "p_201") })
	p := h.get(t, "call_front-door", "p_201")
	if p.Type != "sip_participant" || p.State.(domain.SIPParticipant).Status != "ringing" {
		t.Fatalf("participant = %+v", p)
	}
	if c := h.get(t, "call_front-door", "call"); c.Type != "sip_call" {
		t.Fatalf("call = %+v", c)
	}

	call.Participants[0].Status = "joined"
	h.host.emit(hostapi.EventCall, call)
	soon(t, "joined", func() bool {
		return h.get(t, "call_front-door", "p_201").State.(domain.SIPParticipant).Status == "joined"
	})

	// Scripts cannot see a deletion, so an ended call must stay visible first.
	call.Status = "ended"
	call.Reason = "everyone left"
	h.host.emit(hostapi.EventCall, call)
	soon(t, "ended", func() bool {
		s := h.get(t, "call_front-door", "call").State.(domain.SIPCall)
		return s.Status == "ended" && s.Reason == "everyone left"
	})

	h.host.emit(hostapi.EventRemoved, map[string]string{"id": "front-door"})
	soon(t, "removal", func() bool { return !h.exists("call_front-door", "call") && !h.exists("call_front-door", "p_201") })
}

func TestSync_TranscriptReachesEntity(t *testing.T) {
	h := startSync(t, nil)
	h.host.emit(hostapi.EventCall, hostapi.Call{ID: "voice", Status: "active", Transcript: &hostapi.Transcript{Status: "listening", Seq: 1, Text: "turn the lights on"}})
	soon(t, "transcript", func() bool {
		if !h.exists("call_voice", "transcript") {
			return false
		}
		s := h.get(t, "call_voice", "transcript").State.(domain.Transcript)
		return s.Seq == 1 && s.Text == "turn the lights on"
	})
}

func TestSync_SpeechReachesEntity(t *testing.T) {
	h := startSync(t, nil)
	h.host.emit(hostapi.EventCall, hostapi.Call{ID: "agent", Status: "active", Speech: &hostapi.Speech{Status: "speaking", Seq: 1, Text: "hello"}})
	soon(t, "speech", func() bool {
		if !h.exists("call_agent", "speech") {
			return false
		}
		s := h.get(t, "call_agent", "speech").State.(domain.Speech)
		return s.Status == "speaking" && s.Seq == 1
	})
}

func TestSync_EndpointEventUpdatesState(t *testing.T) {
	h := startSync(t, nil)
	h.host.emit(hostapi.EventEndpoint, hostapi.Endpoint{ID: "201", Name: "Kitchen Phone", Kind: "extension", Status: "online", Call: "c"})
	soon(t, "update", func() bool {
		s := h.get(t, "ext_201", "registration").State.(domain.SIPEndpoint)
		return s.Registered && s.InCall && s.Call == "c"
	})
	if v := h.get(t, "ext_201", "call-state").State.(domain.Sensor).Value; v != "in_call" {
		t.Fatalf("call-state = %v", v)
	}
}

func TestCommands_RouteToHost(t *testing.T) {
	cases := []struct {
		name           string
		device, entity string
		cmd            messenger.Action
		want, wantBody string
	}{
		{"dial endpoint", "ext_201", "registration", domain.SIPDial{Call: "front-door", RingSeconds: 30},
			"POST /v1/calls/front-door/participants", `{"target":"201","ringSeconds":30}`},
		{"dial uses real endpoint id", "ext_float_10000", "registration", domain.SIPDial{Call: "c"},
			"POST /v1/calls/c/participants", `{"target":"float-10000"}`},
		{"dial a number", "ext_201", "registration", domain.SIPDial{Call: "c", Target: "number:+15551234567"},
			"POST /v1/calls/c/participants", `{"target":"number:+15551234567"}`},
		{"hangup endpoint", "ext_201", "registration", domain.SIPHangup{}, "POST /v1/endpoints/201/hangup", ""},
		{"listen", "ext_3000", "audio", domain.MediaStreamStart{}, "POST /v1/endpoints/3000/listen", ""},
		{"stop listening", "ext_3000", "audio", domain.MediaStreamStop{}, "DELETE /v1/endpoints/3000/listen", ""},
		{"reload", "server", "status", domain.PBXReload{}, "POST /v1/server/reload", ""},
		{"end call", "call_x", "call", domain.SIPHangup{}, "DELETE /v1/calls/x", ""},
		{"add to call", "call_x", "call", domain.SIPDial{Target: "202"}, "POST /v1/calls/x/participants", `{"target":"202"}`},
		{"play", "call_x", "call", domain.SIPPlay{Sound: "closed", ThenHangup: true},
			"POST /v1/calls/x/playbacks", `{"sound":"closed","thenHangup":true}`},
		{"transcribe", "call_x", "call", domain.SIPTranscribeStart{}, "POST /v1/calls/x/transcription", ""},
		{"stop transcribing", "call_x", "transcript", domain.SIPTranscribeStop{}, "DELETE /v1/calls/x/transcription", ""},
		{"say", "call_x", "call", domain.SIPSay{Text: "eight lights are on"}, "POST /v1/calls/x/speech", `{"text":"eight lights are on"}`},
		{"say on speech entity", "call_x", "speech", domain.SIPSay{Text: "hi"}, "POST /v1/calls/x/speech", `{"text":"hi"}`},
		{"stream call", "call_x", "audio", domain.MediaStreamStart{}, "POST /v1/calls/x/streams", ""},
		{"stop call stream", "call_x", "audio", domain.MediaStreamStop{}, "DELETE /v1/calls/x/streams", ""},
		{"mute", "call_x", "p_201", domain.SIPMute{}, "POST /v1/calls/x/participants/201/commands", `{"action":"mute"}`},
		{"unmute", "call_x", "p_201", domain.SIPUnmute{}, "POST /v1/calls/x/participants/201/commands", `{"action":"unmute"}`},
		{"hold", "call_x", "p_201", domain.SIPHold{}, "POST /v1/calls/x/participants/201/commands", `{"action":"hold"}`},
		{"unhold", "call_x", "p_201", domain.SIPUnhold{}, "POST /v1/calls/x/participants/201/commands", `{"action":"unhold"}`},
		{"dtmf", "call_x", "p_201", domain.SIPDTMF{Digits: "#"}, "POST /v1/calls/x/participants/201/commands", `{"action":"dtmf","parameters":{"digits":"#"}}`},
		{"remove", "call_x", "p_201", domain.SIPRemove{}, "DELETE /v1/calls/x/participants/201", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := startSync(t, nil)
			h.send(t, tc.device, tc.entity, tc.cmd)
			soon(t, tc.want, func() bool { return h.host.called(tc.want) })
			if got := strings.TrimSpace(h.host.body(tc.want)); got != tc.wantBody {
				t.Fatalf("body = %q, want %q", got, tc.wantBody)
			}
		})
	}
}

func TestSync_NoRepeatSaveWhenNothingChanged(t *testing.T) {
	env := testkit.NewTestEnv(t)
	env.Start("messenger")
	env.Start("storage")
	a := New()
	a.store = env.Storage()
	item := hostapi.Endpoint{ID: "201", Name: "n", Kind: "extension", Status: "online"}
	a.applyEndpoint(item)
	if a.changed("plugin-asterisk.ext_201.availability", "different") != true {
		t.Fatal("a different value should count as changed")
	}
	if a.changed("plugin-asterisk.ext_201.availability", "different") != false {
		t.Fatal("the same value should not count as changed")
	}
}
