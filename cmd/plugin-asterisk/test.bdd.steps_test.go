//go:build bdd

package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cucumber/godog"
	domain "github.com/slidebolt/sb-domain"
	messenger "github.com/slidebolt/sb-messenger-sdk"
	storage "github.com/slidebolt/sb-storage-sdk"
	testkit "github.com/slidebolt/sb-testkit"

	"github.com/slidebolt/plugin-asterisk/app"
	translate "github.com/slidebolt/plugin-asterisk/internal/translate"
)

// ---------------------------------------------------------------------------
// Scenario context — one per scenario, reset in BeforeScenario
// ---------------------------------------------------------------------------

type bddCtx struct {
	t     *testing.T
	env   *testkit.TestEnv
	store storage.Storage
	cmds  *messenger.Commands

	lastEntity       domain.Entity
	lastGetErr       error
	lastEntries      []storage.Entry
	lastInternalData json.RawMessage
	lastWirePayload  json.RawMessage

	cmdReceived chan string
	cmdSub      messenger.Subscription
}

func newBDDCtx(t *testing.T) *bddCtx {
	t.Helper()
	env := testkit.NewTestEnv(t)
	env.Start("messenger")
	env.Start("storage")
	c := &bddCtx{
		t:           t,
		env:         env,
		store:       env.Storage(),
		cmds:        messenger.NewCommands(env.Messenger(), domain.LookupCommand),
		cmdReceived: make(chan string, 1),
	}
	return c
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func parseKey(key string) (plugin, device, id string, err error) {
	parts := strings.SplitN(key, ".", 3)
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("key %q must have 3 dot-separated segments", key)
	}
	return parts[0], parts[1], parts[2], nil
}

func (c *bddCtx) saveEntity(e domain.Entity) error {
	return c.store.Save(e)
}

func (c *bddCtx) getEntity(key string) (domain.Entity, error) {
	plug, dev, id, err := parseKey(key)
	if err != nil {
		return domain.Entity{}, err
	}
	raw, err := c.store.Get(domain.EntityKey{Plugin: plug, DeviceID: dev, ID: id})
	if err != nil {
		return domain.Entity{}, err
	}
	var e domain.Entity
	return e, json.Unmarshal(raw, &e)
}

// ---------------------------------------------------------------------------
// Entity creation steps
// ---------------------------------------------------------------------------

// PBX

func (c *bddCtx) aPBXEntity(key, name string, connected string) error {
	plug, dev, id, err := parseKey(key)
	if err != nil {
		return err
	}
	return c.saveEntity(domain.Entity{
		ID: id, Plugin: plug, DeviceID: dev,
		Type: "pbx", Name: name,
		State: app.PBX{Connected: connected == "true"},
	})
}

func (c *bddCtx) aPBXEntityFull(key, name string, connected string, version string, uptime int) error {
	plug, dev, id, err := parseKey(key)
	if err != nil {
		return err
	}
	return c.saveEntity(domain.Entity{
		ID: id, Plugin: plug, DeviceID: dev,
		Type: "pbx", Name: name,
		State: app.PBX{Connected: connected == "true", Version: version, Uptime: int64(uptime)},
	})
}

// SIP Trunk

func (c *bddCtx) aSIPTrunkEntity(key, name string, registered string, host string) error {
	plug, dev, id, err := parseKey(key)
	if err != nil {
		return err
	}
	return c.saveEntity(domain.Entity{
		ID: id, Plugin: plug, DeviceID: dev,
		Type: "sip_trunk", Name: name,
		State: app.SIPTrunk{Registered: registered == "true", Host: host},
	})
}

func (c *bddCtx) aSIPTrunkEntityFull(key, name string, registered string, host string, port, latency int) error {
	plug, dev, id, err := parseKey(key)
	if err != nil {
		return err
	}
	return c.saveEntity(domain.Entity{
		ID: id, Plugin: plug, DeviceID: dev,
		Type: "sip_trunk", Name: name,
		State: app.SIPTrunk{Registered: registered == "true", Host: host, Port: port, Latency: latency},
	})
}

// SIP Endpoint

func (c *bddCtx) aSIPEndpointEntity(key, name string, registered string) error {
	plug, dev, id, err := parseKey(key)
	if err != nil {
		return err
	}
	return c.saveEntity(domain.Entity{
		ID: id, Plugin: plug, DeviceID: dev,
		Type: "sip_endpoint", Name: name,
		State: app.SIPEndpoint{Registered: registered == "true"},
	})
}

func (c *bddCtx) aSIPEndpointEntityFull(key, name string, registered, inCall string, ip string) error {
	plug, dev, id, err := parseKey(key)
	if err != nil {
		return err
	}
	return c.saveEntity(domain.Entity{
		ID: id, Plugin: plug, DeviceID: dev,
		Type: "sip_endpoint", Name: name,
		State: app.SIPEndpoint{Registered: registered == "true", InCall: inCall == "true", IP: ip},
	})
}

// SIP Call

func (c *bddCtx) aSIPCallEntity(key, name, state, caller, callee string, duration int) error {
	plug, dev, id, err := parseKey(key)
	if err != nil {
		return err
	}
	return c.saveEntity(domain.Entity{
		ID: id, Plugin: plug, DeviceID: dev,
		Type: "sip_call", Name: name,
		State: app.SIPCall{State: state, Caller: caller, Callee: callee, Duration: duration},
	})
}

// Voicemail

func (c *bddCtx) aVoicemailEntity(key, name string, newMsgs, oldMsgs int, mailbox string) error {
	plug, dev, id, err := parseKey(key)
	if err != nil {
		return err
	}
	return c.saveEntity(domain.Entity{
		ID: id, Plugin: plug, DeviceID: dev,
		Type: "voicemail", Name: name,
		State: app.Voicemail{NewMessages: newMsgs, OldMessages: oldMsgs, Mailbox: mailbox},
	})
}

// Call Queue

func (c *bddCtx) aCallQueueEntity(key, name string, callers, available int, strategy string) error {
	plug, dev, id, err := parseKey(key)
	if err != nil {
		return err
	}
	return c.saveEntity(domain.Entity{
		ID: id, Plugin: plug, DeviceID: dev,
		Type: "call_queue", Name: name,
		State: app.CallQueue{Callers: callers, Available: available, Strategy: strategy},
	})
}

// ---------------------------------------------------------------------------
// Update steps
// ---------------------------------------------------------------------------

func (c *bddCtx) updatePBXConnected(key string, connected string) error {
	plug, dev, id, err := parseKey(key)
	if err != nil {
		return err
	}
	return c.saveEntity(domain.Entity{
		ID: id, Plugin: plug, DeviceID: dev,
		Type:  "pbx",
		State: app.PBX{Connected: connected == "true"},
	})
}

func (c *bddCtx) updateTrunkRegistered(key string, registered string) error {
	plug, dev, id, err := parseKey(key)
	if err != nil {
		return err
	}
	return c.saveEntity(domain.Entity{
		ID: id, Plugin: plug, DeviceID: dev,
		Type:  "sip_trunk",
		State: app.SIPTrunk{Registered: registered == "true"},
	})
}

func (c *bddCtx) updateEndpointInCall(key string, inCall string) error {
	plug, dev, id, err := parseKey(key)
	if err != nil {
		return err
	}
	return c.saveEntity(domain.Entity{
		ID: id, Plugin: plug, DeviceID: dev,
		Type:  "sip_endpoint",
		State: app.SIPEndpoint{Registered: true, InCall: inCall == "true"},
	})
}

func (c *bddCtx) updateVoicemailMessages(key string, newMsgs, oldMsgs int) error {
	plug, dev, id, err := parseKey(key)
	if err != nil {
		return err
	}
	return c.saveEntity(domain.Entity{
		ID: id, Plugin: plug, DeviceID: dev,
		Type:  "voicemail",
		State: app.Voicemail{NewMessages: newMsgs, OldMessages: oldMsgs},
	})
}

func (c *bddCtx) updateCallState(key, state string) error {
	plug, dev, id, err := parseKey(key)
	if err != nil {
		return err
	}
	return c.saveEntity(domain.Entity{
		ID: id, Plugin: plug, DeviceID: dev,
		Type:  "sip_call",
		State: app.SIPCall{State: state},
	})
}

// ---------------------------------------------------------------------------
// Entity Lifecycle steps
// ---------------------------------------------------------------------------

func (c *bddCtx) iRetrieve(key string) error {
	e, err := c.getEntity(key)
	c.lastEntity = e
	c.lastGetErr = err
	return nil
}

func (c *bddCtx) iDelete(key string) error {
	plug, dev, id, err := parseKey(key)
	if err != nil {
		return err
	}
	return c.store.Delete(domain.EntityKey{Plugin: plug, DeviceID: dev, ID: id})
}

func (c *bddCtx) retrievingKeyFails(key string) error {
	_, err := c.getEntity(key)
	if err == nil {
		return fmt.Errorf("expected retrieval of %q to fail, but it succeeded", key)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Assertion steps: generic
// ---------------------------------------------------------------------------

func (c *bddCtx) entityTypeIs(expected string) error {
	if c.lastGetErr != nil {
		return fmt.Errorf("retrieve failed: %w", c.lastGetErr)
	}
	if c.lastEntity.Type != expected {
		return fmt.Errorf("entity type: got %q, want %q", c.lastEntity.Type, expected)
	}
	return nil
}

func (c *bddCtx) entityNameIs(expected string) error {
	if c.lastGetErr != nil {
		return fmt.Errorf("retrieve failed: %w", c.lastGetErr)
	}
	if c.lastEntity.Name != expected {
		return fmt.Errorf("entity name: got %q, want %q", c.lastEntity.Name, expected)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Assertion steps: PBX
// ---------------------------------------------------------------------------

func (c *bddCtx) pbxConnectedIs(expected string) error {
	if c.lastGetErr != nil {
		return fmt.Errorf("retrieve failed: %w", c.lastGetErr)
	}
	st, ok := c.lastEntity.State.(app.PBX)
	if !ok {
		return fmt.Errorf("state type: got %T, want app.PBX", c.lastEntity.State)
	}
	want := expected == "true"
	if st.Connected != want {
		return fmt.Errorf("pbx.Connected: got %v, want %v", st.Connected, want)
	}
	return nil
}

func (c *bddCtx) pbxVersionIs(expected string) error {
	if c.lastGetErr != nil {
		return fmt.Errorf("retrieve failed: %w", c.lastGetErr)
	}
	st, ok := c.lastEntity.State.(app.PBX)
	if !ok {
		return fmt.Errorf("state type: got %T, want app.PBX", c.lastEntity.State)
	}
	if st.Version != expected {
		return fmt.Errorf("pbx.Version: got %q, want %q", st.Version, expected)
	}
	return nil
}

func (c *bddCtx) pbxUptimeIs(expected int) error {
	if c.lastGetErr != nil {
		return fmt.Errorf("retrieve failed: %w", c.lastGetErr)
	}
	st, ok := c.lastEntity.State.(app.PBX)
	if !ok {
		return fmt.Errorf("state type: got %T, want app.PBX", c.lastEntity.State)
	}
	if st.Uptime != int64(expected) {
		return fmt.Errorf("pbx.Uptime: got %d, want %d", st.Uptime, expected)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Assertion steps: SIP Trunk
// ---------------------------------------------------------------------------

func (c *bddCtx) trunkRegisteredIs(expected string) error {
	if c.lastGetErr != nil {
		return fmt.Errorf("retrieve failed: %w", c.lastGetErr)
	}
	st, ok := c.lastEntity.State.(app.SIPTrunk)
	if !ok {
		return fmt.Errorf("state type: got %T, want app.SIPTrunk", c.lastEntity.State)
	}
	want := expected == "true"
	if st.Registered != want {
		return fmt.Errorf("trunk.Registered: got %v, want %v", st.Registered, want)
	}
	return nil
}

func (c *bddCtx) trunkHostIs(expected string) error {
	if c.lastGetErr != nil {
		return fmt.Errorf("retrieve failed: %w", c.lastGetErr)
	}
	st, ok := c.lastEntity.State.(app.SIPTrunk)
	if !ok {
		return fmt.Errorf("state type: got %T, want app.SIPTrunk", c.lastEntity.State)
	}
	if st.Host != expected {
		return fmt.Errorf("trunk.Host: got %q, want %q", st.Host, expected)
	}
	return nil
}

func (c *bddCtx) trunkLatencyIs(expected int) error {
	if c.lastGetErr != nil {
		return fmt.Errorf("retrieve failed: %w", c.lastGetErr)
	}
	st, ok := c.lastEntity.State.(app.SIPTrunk)
	if !ok {
		return fmt.Errorf("state type: got %T, want app.SIPTrunk", c.lastEntity.State)
	}
	if st.Latency != expected {
		return fmt.Errorf("trunk.Latency: got %d, want %d", st.Latency, expected)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Assertion steps: SIP Endpoint
// ---------------------------------------------------------------------------

func (c *bddCtx) endpointRegisteredIs(expected string) error {
	if c.lastGetErr != nil {
		return fmt.Errorf("retrieve failed: %w", c.lastGetErr)
	}
	st, ok := c.lastEntity.State.(app.SIPEndpoint)
	if !ok {
		return fmt.Errorf("state type: got %T, want app.SIPEndpoint", c.lastEntity.State)
	}
	want := expected == "true"
	if st.Registered != want {
		return fmt.Errorf("endpoint.Registered: got %v, want %v", st.Registered, want)
	}
	return nil
}

func (c *bddCtx) endpointInCallIs(expected string) error {
	if c.lastGetErr != nil {
		return fmt.Errorf("retrieve failed: %w", c.lastGetErr)
	}
	st, ok := c.lastEntity.State.(app.SIPEndpoint)
	if !ok {
		return fmt.Errorf("state type: got %T, want app.SIPEndpoint", c.lastEntity.State)
	}
	want := expected == "true"
	if st.InCall != want {
		return fmt.Errorf("endpoint.InCall: got %v, want %v", st.InCall, want)
	}
	return nil
}

func (c *bddCtx) endpointIPIs(expected string) error {
	if c.lastGetErr != nil {
		return fmt.Errorf("retrieve failed: %w", c.lastGetErr)
	}
	st, ok := c.lastEntity.State.(app.SIPEndpoint)
	if !ok {
		return fmt.Errorf("state type: got %T, want app.SIPEndpoint", c.lastEntity.State)
	}
	if st.IP != expected {
		return fmt.Errorf("endpoint.IP: got %q, want %q", st.IP, expected)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Assertion steps: SIP Call
// ---------------------------------------------------------------------------

func (c *bddCtx) callStateIs(expected string) error {
	if c.lastGetErr != nil {
		return fmt.Errorf("retrieve failed: %w", c.lastGetErr)
	}
	st, ok := c.lastEntity.State.(app.SIPCall)
	if !ok {
		return fmt.Errorf("state type: got %T, want app.SIPCall", c.lastEntity.State)
	}
	if st.State != expected {
		return fmt.Errorf("call.State: got %q, want %q", st.State, expected)
	}
	return nil
}

func (c *bddCtx) callCallerIs(expected string) error {
	if c.lastGetErr != nil {
		return fmt.Errorf("retrieve failed: %w", c.lastGetErr)
	}
	st := c.lastEntity.State.(app.SIPCall)
	if st.Caller != expected {
		return fmt.Errorf("call.Caller: got %q, want %q", st.Caller, expected)
	}
	return nil
}

func (c *bddCtx) callCalleeIs(expected string) error {
	if c.lastGetErr != nil {
		return fmt.Errorf("retrieve failed: %w", c.lastGetErr)
	}
	st := c.lastEntity.State.(app.SIPCall)
	if st.Callee != expected {
		return fmt.Errorf("call.Callee: got %q, want %q", st.Callee, expected)
	}
	return nil
}

func (c *bddCtx) callDurationIs(expected int) error {
	if c.lastGetErr != nil {
		return fmt.Errorf("retrieve failed: %w", c.lastGetErr)
	}
	st := c.lastEntity.State.(app.SIPCall)
	if st.Duration != expected {
		return fmt.Errorf("call.Duration: got %d, want %d", st.Duration, expected)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Assertion steps: Voicemail
// ---------------------------------------------------------------------------

func (c *bddCtx) voicemailNewIs(expected int) error {
	if c.lastGetErr != nil {
		return fmt.Errorf("retrieve failed: %w", c.lastGetErr)
	}
	st, ok := c.lastEntity.State.(app.Voicemail)
	if !ok {
		return fmt.Errorf("state type: got %T, want app.Voicemail", c.lastEntity.State)
	}
	if st.NewMessages != expected {
		return fmt.Errorf("voicemail.NewMessages: got %d, want %d", st.NewMessages, expected)
	}
	return nil
}

func (c *bddCtx) voicemailOldIs(expected int) error {
	if c.lastGetErr != nil {
		return fmt.Errorf("retrieve failed: %w", c.lastGetErr)
	}
	st := c.lastEntity.State.(app.Voicemail)
	if st.OldMessages != expected {
		return fmt.Errorf("voicemail.OldMessages: got %d, want %d", st.OldMessages, expected)
	}
	return nil
}

func (c *bddCtx) voicemailMailboxIs(expected string) error {
	if c.lastGetErr != nil {
		return fmt.Errorf("retrieve failed: %w", c.lastGetErr)
	}
	st := c.lastEntity.State.(app.Voicemail)
	if st.Mailbox != expected {
		return fmt.Errorf("voicemail.Mailbox: got %q, want %q", st.Mailbox, expected)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Assertion steps: Call Queue
// ---------------------------------------------------------------------------

func (c *bddCtx) queueCallersIs(expected int) error {
	if c.lastGetErr != nil {
		return fmt.Errorf("retrieve failed: %w", c.lastGetErr)
	}
	st, ok := c.lastEntity.State.(app.CallQueue)
	if !ok {
		return fmt.Errorf("state type: got %T, want app.CallQueue", c.lastEntity.State)
	}
	if st.Callers != expected {
		return fmt.Errorf("queue.Callers: got %d, want %d", st.Callers, expected)
	}
	return nil
}

func (c *bddCtx) queueAvailableIs(expected int) error {
	if c.lastGetErr != nil {
		return fmt.Errorf("retrieve failed: %w", c.lastGetErr)
	}
	st := c.lastEntity.State.(app.CallQueue)
	if st.Available != expected {
		return fmt.Errorf("queue.Available: got %d, want %d", st.Available, expected)
	}
	return nil
}

func (c *bddCtx) queueStrategyIs(expected string) error {
	if c.lastGetErr != nil {
		return fmt.Errorf("retrieve failed: %w", c.lastGetErr)
	}
	st := c.lastEntity.State.(app.CallQueue)
	if st.Strategy != expected {
		return fmt.Errorf("queue.Strategy: got %q, want %q", st.Strategy, expected)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Command Dispatch steps
// ---------------------------------------------------------------------------

func (c *bddCtx) aCommandListenerOn(pattern string) error {
	if c.cmdSub != nil {
		c.cmdSub.Unsubscribe()
	}
	c.cmdReceived = make(chan string, 1)
	sub, err := c.cmds.Receive(pattern, func(_ messenger.Address, cmd any) {
		select {
		case c.cmdReceived <- actionNameOf(cmd):
		default:
		}
	})
	if err != nil {
		return err
	}
	c.cmdSub = sub
	return nil
}

func (c *bddCtx) iSendCommandTo(action, key string) error {
	plug, dev, id, err := parseKey(key)
	if err != nil {
		return err
	}
	target := domain.EntityKey{Plugin: plug, DeviceID: dev, ID: id}
	cmd, err := makeCommand(action)
	if err != nil {
		return err
	}
	return c.cmds.Send(target, cmd)
}

func (c *bddCtx) iSendSIPCallTo(extension, key string) error {
	plug, dev, id, err := parseKey(key)
	if err != nil {
		return err
	}
	target := domain.EntityKey{Plugin: plug, DeviceID: dev, ID: id}
	return c.cmds.Send(target, app.SIPCallOriginate{Extension: extension})
}

func (c *bddCtx) iSendSIPTransferTo(extension, key string) error {
	plug, dev, id, err := parseKey(key)
	if err != nil {
		return err
	}
	target := domain.EntityKey{Plugin: plug, DeviceID: dev, ID: id}
	return c.cmds.Send(target, app.SIPTransfer{Extension: extension})
}

func (c *bddCtx) iSendVoicemailDeleteTo(mailbox, key string) error {
	plug, dev, id, err := parseKey(key)
	if err != nil {
		return err
	}
	target := domain.EntityKey{Plugin: plug, DeviceID: dev, ID: id}
	return c.cmds.Send(target, app.VoicemailDelete{Mailbox: mailbox})
}

func (c *bddCtx) receivedCommandActionIs(expected string) error {
	select {
	case got := <-c.cmdReceived:
		if got != expected {
			return fmt.Errorf("command action: got %q, want %q", got, expected)
		}
		return nil
	case <-time.After(2 * time.Second):
		return fmt.Errorf("timed out waiting for command %q", expected)
	}
}

func actionNameOf(cmd any) string {
	type namer interface{ ActionName() string }
	if n, ok := cmd.(namer); ok {
		return n.ActionName()
	}
	return fmt.Sprintf("unknown(%T)", cmd)
}

func makeCommand(action string) (messenger.Action, error) {
	switch action {
	case "pbx_reload":
		return app.PBXReload{}, nil
	case "sip_call":
		return app.SIPCallOriginate{Extension: "200"}, nil
	case "sip_hangup":
		return app.SIPHangup{Channel: "SIP/100-00000001"}, nil
	case "sip_transfer":
		return app.SIPTransfer{Extension: "300"}, nil
	case "sip_mute":
		return app.SIPMute{Muted: true}, nil
	case "voicemail_delete":
		return app.VoicemailDelete{Mailbox: "100"}, nil
	default:
		return nil, fmt.Errorf("unknown action %q", action)
	}
}

// ---------------------------------------------------------------------------
// Query DSL steps
// ---------------------------------------------------------------------------

func parseBoolOrString(v string) any {
	switch v {
	case "true":
		return true
	case "false":
		return false
	default:
		return v
	}
}

func (c *bddCtx) iQueryWhereEquals(field, value string) error {
	entries, err := c.store.Query(storage.Query{
		Where: []storage.Filter{{Field: field, Op: storage.Eq, Value: value}},
	})
	if err != nil {
		return err
	}
	c.lastEntries = entries
	return nil
}

func (c *bddCtx) iQueryWhereTwoFilters(field1, value1, field2, value2 string) error {
	filter2Value := parseBoolOrString(value2)
	entries, err := c.store.Query(storage.Query{
		Where: []storage.Filter{
			{Field: field1, Op: storage.Eq, Value: value1},
			{Field: field2, Op: storage.Eq, Value: filter2Value},
		},
	})
	if err != nil {
		return err
	}
	c.lastEntries = entries
	return nil
}

func (c *bddCtx) iQueryWhereGreaterThan(field1, value1, field2 string, threshold float64) error {
	entries, err := c.store.Query(storage.Query{
		Where: []storage.Filter{
			{Field: field1, Op: storage.Eq, Value: value1},
			{Field: field2, Op: storage.Gt, Value: threshold},
		},
	})
	if err != nil {
		return err
	}
	c.lastEntries = entries
	return nil
}

func (c *bddCtx) iSearchWithPattern(pattern string) error {
	entries, err := c.store.Search(pattern)
	if err != nil {
		return err
	}
	c.lastEntries = entries
	return nil
}

func (c *bddCtx) iGetNResults(expected int) error {
	if len(c.lastEntries) != expected {
		return fmt.Errorf("result count: got %d, want %d", len(c.lastEntries), expected)
	}
	return nil
}

func (c *bddCtx) iGet1Result() error {
	return c.iGetNResults(1)
}

func (c *bddCtx) resultsInclude(key string) error {
	for _, e := range c.lastEntries {
		var entity domain.Entity
		if err := json.Unmarshal(e.Data, &entity); err != nil {
			continue
		}
		if entity.Key() == key {
			return nil
		}
	}
	return fmt.Errorf("results do not include %q (got %d results)", key, len(c.lastEntries))
}

func (c *bddCtx) resultsDoNotInclude(key string) error {
	for _, e := range c.lastEntries {
		var entity domain.Entity
		if err := json.Unmarshal(e.Data, &entity); err != nil {
			continue
		}
		if entity.Key() == key {
			return fmt.Errorf("results unexpectedly include %q", key)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Internal storage steps
// ---------------------------------------------------------------------------

func (c *bddCtx) iWriteInternalData(key, payload string) error {
	plug, dev, id, err := parseKey(key)
	if err != nil {
		return err
	}
	k := domain.EntityKey{Plugin: plug, DeviceID: dev, ID: id}
	return c.store.WriteFile(storage.Internal, k, json.RawMessage(payload))
}

func (c *bddCtx) iReadInternalData(key string) error {
	plug, dev, id, err := parseKey(key)
	if err != nil {
		return err
	}
	k := domain.EntityKey{Plugin: plug, DeviceID: dev, ID: id}
	data, err := c.store.ReadFile(storage.Internal, k)
	if err != nil {
		return fmt.Errorf("ReadFile internal %s: %w", key, err)
	}
	c.lastInternalData = data
	return nil
}

func (c *bddCtx) internalDataMatches(expected string) error {
	if string(c.lastInternalData) != expected {
		return fmt.Errorf("internal data: got %s, want %s", c.lastInternalData, expected)
	}
	return nil
}

func (c *bddCtx) queryingTypeReturnsOnlyStateEntities(typ string) error {
	entries, err := c.store.Query(storage.Query{
		Where: []storage.Filter{{Field: "type", Op: storage.Eq, Value: typ}},
	})
	if err != nil {
		return err
	}
	if len(entries) != 1 {
		return fmt.Errorf("query type=%s: got %d results, want 1 (internal data must not appear)", typ, len(entries))
	}
	return nil
}

// ---------------------------------------------------------------------------
// Translate: Decode and Encode steps
// ---------------------------------------------------------------------------

func (c *bddCtx) iDecodePayloadAs(typeName, rawJSON string) error {
	state, ok := translate.Decode(typeName, json.RawMessage(rawJSON))
	if !ok {
		return fmt.Errorf("Decode(%q, %s) returned false", typeName, rawJSON)
	}
	c.lastEntity.State = state
	c.lastGetErr = nil
	return nil
}

func (c *bddCtx) iEncodeCommandWithJSON(action, jsonPayload string) error {
	typ, ok := domain.LookupCommand(action)
	if !ok {
		return fmt.Errorf("unknown action %q", action)
	}
	v := reflect.New(typ).Interface()
	if err := json.Unmarshal([]byte(jsonPayload), v); err != nil {
		return fmt.Errorf("unmarshal command %q: %w", action, err)
	}
	cmd := reflect.ValueOf(v).Elem().Interface()
	out, err := translate.Encode(cmd, nil)
	if err != nil {
		return fmt.Errorf("Encode(%q): %w", action, err)
	}
	c.lastWirePayload = out
	return nil
}

func (c *bddCtx) wirePayloadFieldEqualsNum(field string, expected float64) error {
	var m map[string]any
	if err := json.Unmarshal(c.lastWirePayload, &m); err != nil {
		return fmt.Errorf("wire payload is not JSON: %w", err)
	}
	v, ok := m[field]
	if !ok {
		return fmt.Errorf("wire payload missing field %q; got %s", field, c.lastWirePayload)
	}
	got, ok := v.(float64)
	if !ok {
		return fmt.Errorf("wire payload field %q: got %T (%v), want float64", field, v, v)
	}
	if got != expected {
		return fmt.Errorf("wire payload field %q: got %v, want %v", field, got, expected)
	}
	return nil
}

func (c *bddCtx) wirePayloadFieldEqualsString(field, expected string) error {
	var m map[string]any
	if err := json.Unmarshal(c.lastWirePayload, &m); err != nil {
		return fmt.Errorf("wire payload is not JSON: %w", err)
	}
	v, ok := m[field]
	if !ok {
		return fmt.Errorf("wire payload missing field %q; got %s", field, c.lastWirePayload)
	}
	got := fmt.Sprintf("%v", v)
	if got != expected {
		return fmt.Errorf("wire payload field %q: got %q, want %q", field, got, expected)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Step registration
// ---------------------------------------------------------------------------

func (c *bddCtx) RegisterSteps(ctx *godog.ScenarioContext) {
	// --- Entity creation ---
	ctx.Step(`^a pbx entity "([^"]*)" named "([^"]*)" with connected (true|false)$`, c.aPBXEntity)
	ctx.Step(`^a pbx entity "([^"]*)" named "([^"]*)" with connected (true|false) version "([^"]*)" uptime (\d+)$`, c.aPBXEntityFull)
	ctx.Step(`^a sip_trunk entity "([^"]*)" named "([^"]*)" with registered (true|false) host "([^"]*)"$`, c.aSIPTrunkEntity)
	ctx.Step(`^a sip_trunk entity "([^"]*)" named "([^"]*)" with registered (true|false) host "([^"]*)" port (\d+) latency (\d+)$`, c.aSIPTrunkEntityFull)
	ctx.Step(`^a sip_endpoint entity "([^"]*)" named "([^"]*)" with registered (true|false)$`, c.aSIPEndpointEntity)
	ctx.Step(`^a sip_endpoint entity "([^"]*)" named "([^"]*)" with registered (true|false) in_call (true|false) ip "([^"]*)"$`, c.aSIPEndpointEntityFull)
	ctx.Step(`^a sip_call entity "([^"]*)" named "([^"]*)" with state "([^"]*)" caller "([^"]*)" callee "([^"]*)" duration (\d+)$`, c.aSIPCallEntity)
	ctx.Step(`^a voicemail entity "([^"]*)" named "([^"]*)" with new_messages (\d+) old_messages (\d+) mailbox "([^"]*)"$`, c.aVoicemailEntity)
	ctx.Step(`^a call_queue entity "([^"]*)" named "([^"]*)" with callers (\d+) available (\d+) strategy "([^"]*)"$`, c.aCallQueueEntity)

	// --- Update steps ---
	ctx.Step(`^I update pbx "([^"]*)" to connected (true|false)$`, c.updatePBXConnected)
	ctx.Step(`^I update trunk "([^"]*)" to registered (true|false)$`, c.updateTrunkRegistered)
	ctx.Step(`^I update endpoint "([^"]*)" to in_call (true|false)$`, c.updateEndpointInCall)
	ctx.Step(`^I update voicemail "([^"]*)" to new_messages (\d+) old_messages (\d+)$`, c.updateVoicemailMessages)
	ctx.Step(`^I update call "([^"]*)" to state "([^"]*)"$`, c.updateCallState)

	// --- Lifecycle ---
	ctx.Step(`^I retrieve "([^"]*)"$`, c.iRetrieve)
	ctx.Step(`^I delete "([^"]*)"$`, c.iDelete)
	ctx.Step(`^retrieving "([^"]*)" should fail$`, c.retrievingKeyFails)

	// --- Assertions: generic ---
	ctx.Step(`^the entity type is "([^"]*)"$`, c.entityTypeIs)
	ctx.Step(`^the entity name is "([^"]*)"$`, c.entityNameIs)

	// --- Assertions: PBX ---
	ctx.Step(`^the pbx connected is (true|false)$`, c.pbxConnectedIs)
	ctx.Step(`^the pbx version is "([^"]*)"$`, c.pbxVersionIs)
	ctx.Step(`^the pbx uptime is (\d+)$`, c.pbxUptimeIs)

	// --- Assertions: SIP Trunk ---
	ctx.Step(`^the trunk registered is (true|false)$`, c.trunkRegisteredIs)
	ctx.Step(`^the trunk host is "([^"]*)"$`, c.trunkHostIs)
	ctx.Step(`^the trunk latency is (\d+)$`, c.trunkLatencyIs)

	// --- Assertions: SIP Endpoint ---
	ctx.Step(`^the endpoint registered is (true|false)$`, c.endpointRegisteredIs)
	ctx.Step(`^the endpoint in_call is (true|false)$`, c.endpointInCallIs)
	ctx.Step(`^the endpoint ip is "([^"]*)"$`, c.endpointIPIs)

	// --- Assertions: SIP Call ---
	ctx.Step(`^the call state is "([^"]*)"$`, c.callStateIs)
	ctx.Step(`^the call caller is "([^"]*)"$`, c.callCallerIs)
	ctx.Step(`^the call callee is "([^"]*)"$`, c.callCalleeIs)
	ctx.Step(`^the call duration is (\d+)$`, c.callDurationIs)

	// --- Assertions: Voicemail ---
	ctx.Step(`^the voicemail new_messages is (\d+)$`, c.voicemailNewIs)
	ctx.Step(`^the voicemail old_messages is (\d+)$`, c.voicemailOldIs)
	ctx.Step(`^the voicemail mailbox is "([^"]*)"$`, c.voicemailMailboxIs)

	// --- Assertions: Call Queue ---
	ctx.Step(`^the queue callers is (\d+)$`, c.queueCallersIs)
	ctx.Step(`^the queue available is (\d+)$`, c.queueAvailableIs)
	ctx.Step(`^the queue strategy is "([^"]*)"$`, c.queueStrategyIs)

	// --- Command dispatch ---
	ctx.Step(`^a command listener on "([^"]*)"$`, c.aCommandListenerOn)
	ctx.Step(`^I send "([^"]*)" to "([^"]*)"$`, c.iSendCommandTo)
	ctx.Step(`^I send "sip_call" with extension "([^"]*)" to "([^"]*)"$`, c.iSendSIPCallTo)
	ctx.Step(`^I send "sip_transfer" with extension "([^"]*)" to "([^"]*)"$`, c.iSendSIPTransferTo)
	ctx.Step(`^I send "voicemail_delete" with mailbox "([^"]*)" to "([^"]*)"$`, c.iSendVoicemailDeleteTo)
	ctx.Step(`^the received command action is "([^"]*)"$`, c.receivedCommandActionIs)

	// --- Query DSL ---
	ctx.Step(`^I query where "([^"]*)" equals "([^"]*)"$`, c.iQueryWhereEquals)
	ctx.Step(`^I query where "([^"]*)" equals "([^"]*)" and "([^"]*)" equals "([^"]*)"$`, c.iQueryWhereTwoFilters)
	ctx.Step(`^I query where "([^"]*)" equals "([^"]*)" and "([^"]*)" greater than ([\d.]+)$`, c.iQueryWhereGreaterThan)
	ctx.Step(`^I search with pattern "([^"]*)"$`, c.iSearchWithPattern)
	ctx.Step(`^I get (\d+) results$`, c.iGetNResults)
	ctx.Step(`^I get 1 result$`, c.iGet1Result)
	ctx.Step(`^the results include "([^"]*)"$`, c.resultsInclude)
	ctx.Step(`^the results do not include "([^"]*)"$`, c.resultsDoNotInclude)

	// --- Internal storage ---
	ctx.Step(`^I write internal data for "([^"]*)" with payload '([^']*)'$`, c.iWriteInternalData)
	ctx.Step(`^I read internal data for "([^"]*)"$`, c.iReadInternalData)
	ctx.Step(`^the internal data matches '([^']*)'$`, c.internalDataMatches)
	ctx.Step(`^querying type "([^"]*)" returns only state entities$`, c.queryingTypeReturnsOnlyStateEntities)

	// --- Translate: Decode / Encode ---
	ctx.Step(`^I decode a "([^"]*)" payload '([^']*)'$`, c.iDecodePayloadAs)
	ctx.Step(`^I encode "([^"]*)" command with '([^']*)'$`, c.iEncodeCommandWithJSON)
	ctx.Step(`^the wire payload field "([^"]*)" equals (\d+(?:\.\d+)?)$`, c.wirePayloadFieldEqualsNum)
	ctx.Step(`^the wire payload field "([^"]*)" equals "([^"]*)"$`, c.wirePayloadFieldEqualsString)
}
