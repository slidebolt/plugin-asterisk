// Unit tests for plugin-asterisk.
//
// Test layer philosophy:
//   Unit tests (this file): pure domain logic, cross-entity behavior,
//     and custom entity type registration. Things that don't express
//     well as BDD scenarios or that test infrastructure capabilities
//     across multiple entity types simultaneously.
//
//   BDD tests (features/*.feature, -tags bdd): per-entity behavioral
//     contract. One feature file per entity type. These are the
//     source of truth for what a plugin promises to support.
//
// Run:
//
//	go test ./...              - unit tests only
//	go test -tags bdd ./...    - unit tests + BDD scenarios

package app

import (
	"encoding/json"
	"testing"
	"time"

	domain "github.com/slidebolt/sb-domain"
	messenger "github.com/slidebolt/sb-messenger-sdk"
	storage "github.com/slidebolt/sb-storage-sdk"
	testkit "github.com/slidebolt/sb-testkit"
)

// --- Test helpers ---

func env(t *testing.T) (*testkit.TestEnv, storage.Storage, *messenger.Commands) {
	t.Helper()
	e := testkit.NewTestEnv(t)
	e.Start("messenger")
	e.Start("storage")
	cmds := messenger.NewCommands(e.Messenger(), domain.LookupCommand)
	return e, e.Storage(), cmds
}

func saveEntity(t *testing.T, store storage.Storage, plugin, device, id, typ, name string, state any) domain.Entity {
	t.Helper()
	e := domain.Entity{
		ID: id, Plugin: plugin, DeviceID: device,
		Type: typ, Name: name, State: state,
	}
	if err := store.Save(e); err != nil {
		t.Fatalf("save %s: %v", id, err)
	}
	return e
}

func getEntity(t *testing.T, store storage.Storage, plugin, device, id string) domain.Entity {
	t.Helper()
	raw, err := store.Get(domain.EntityKey{Plugin: plugin, DeviceID: device, ID: id})
	if err != nil {
		t.Fatalf("get %s.%s.%s: %v", plugin, device, id, err)
	}
	var entity domain.Entity
	if err := json.Unmarshal(raw, &entity); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return entity
}

func queryByType(t *testing.T, store storage.Storage, typ string) []storage.Entry {
	t.Helper()
	entries, err := store.Query(storage.Query{
		Where: []storage.Filter{{Field: "type", Op: storage.Eq, Value: typ}},
	})
	if err != nil {
		t.Fatalf("query type=%s: %v", typ, err)
	}
	return entries
}

func sendAndReceive(t *testing.T, cmds *messenger.Commands, entity domain.Entity, cmd any, pattern string) any {
	t.Helper()
	done := make(chan any, 1)
	cmds.Receive(pattern, func(addr messenger.Address, c any) {
		done <- c
	})
	if err := cmds.Send(entity, cmd.(messenger.Action)); err != nil {
		t.Fatalf("send: %v", err)
	}
	select {
	case got := <-done:
		return got
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for command")
		return nil
	}
}

// ==========================================================================
// Internal storage: plugin-private data, invisible to query/search
// ==========================================================================

func TestInternal_WriteReadDelete(t *testing.T) {
	_, store, _ := env(t)
	key := domain.EntityKey{Plugin: PluginID, DeviceID: "asterisk", ID: "server"}
	payload := json.RawMessage(`{"ariEndpoint":"http://172.27.255.250:8088","username":"admin"}`)

	if err := store.WriteFile(storage.Internal, key, payload); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := store.ReadFile(storage.Internal, key)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != string(payload) {
		t.Errorf("ReadFile: got %s, want %s", got, payload)
	}

	if err := store.DeleteFile(storage.Internal, key); err != nil {
		t.Fatalf("DeleteFile: %v", err)
	}
	if _, err := store.ReadFile(storage.Internal, key); err == nil {
		t.Fatal("expected ReadFile to fail after DeleteFile")
	}
}

func TestInternal_NotVisibleInQuery(t *testing.T) {
	_, store, _ := env(t)
	key := domain.EntityKey{Plugin: PluginID, DeviceID: "asterisk", ID: "server"}

	saveEntity(t, store, PluginID, "asterisk", "server", "pbx", "PBX", PBX{Connected: true})
	store.WriteFile(storage.Internal, key, json.RawMessage(`{"ariEndpoint":"http://172.27.255.250:8088"}`))

	entries := queryByType(t, store, "pbx")
	if len(entries) != 1 {
		t.Fatalf("query: got %d results, want 1", len(entries))
	}
}

func TestInternal_NotVisibleInSearch(t *testing.T) {
	_, store, _ := env(t)
	key := domain.EntityKey{Plugin: PluginID, DeviceID: "asterisk", ID: "server"}

	saveEntity(t, store, PluginID, "asterisk", "server", "pbx", "PBX", PBX{Connected: true})
	store.WriteFile(storage.Internal, key, json.RawMessage(`{"ariEndpoint":"http://172.27.255.250:8088"}`))

	entries, err := store.Search(PluginID + ".>")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("search: got %d results, want 1", len(entries))
	}
	if entries[0].Key != PluginID+".asterisk.server" {
		t.Errorf("search result key: got %q, want %s.asterisk.server", entries[0].Key, PluginID)
	}
}

// ==========================================================================
// Cross-cutting: multi-plugin isolation
// ==========================================================================

func TestCrossCutting_MultiPluginIsolation(t *testing.T) {
	_, store, _ := env(t)
	saveEntity(t, store, PluginID, "asterisk", "server", "pbx", "Asterisk PBX", PBX{Connected: true})
	saveEntity(t, store, "plugin-other", "dev1", "server", "pbx", "Other PBX", PBX{Connected: false})

	entries, _ := store.Query(storage.Query{
		Pattern: PluginID + ".>",
		Where:   []storage.Filter{{Field: "type", Op: storage.Eq, Value: "pbx"}},
	})
	if len(entries) != 1 {
		t.Fatalf("asterisk pbx: got %d, want 1", len(entries))
	}
}

func TestCrossCutting_QueryAllRegistered(t *testing.T) {
	_, store, _ := env(t)
	saveEntity(t, store, PluginID, "asterisk", "trunk_voipms", "sip_trunk", "VoIP.ms",
		SIPTrunk{Registered: true})
	saveEntity(t, store, PluginID, "asterisk", "trunk_other", "sip_trunk", "Other Trunk",
		SIPTrunk{Registered: false})
	saveEntity(t, store, PluginID, "asterisk", "ext_100", "sip_endpoint", "Ext 100",
		SIPEndpoint{Registered: true})

	entries, _ := store.Query(storage.Query{
		Where: []storage.Filter{{Field: "state.registered", Op: storage.Eq, Value: true}},
	})
	if len(entries) != 2 {
		t.Fatalf("registered: got %d, want 2", len(entries))
	}
}

// ==========================================================================
// Custom entity types — full end-to-end
// ==========================================================================

func TestCustom_PBX_SaveGetHydrate(t *testing.T) {
	_, store, _ := env(t)
	saveEntity(t, store, PluginID, "asterisk", "server", "pbx", "Asterisk PBX",
		PBX{Connected: true, Version: "20.5.0", Uptime: 86400})

	got := getEntity(t, store, PluginID, "asterisk", "server")
	s, ok := got.State.(PBX)
	if !ok {
		t.Fatalf("state type: got %T, want PBX", got.State)
	}
	if !s.Connected || s.Version != "20.5.0" || s.Uptime != 86400 {
		t.Errorf("state: %+v", s)
	}
}

func TestCustom_SIPTrunk_SaveGetHydrate(t *testing.T) {
	_, store, _ := env(t)
	saveEntity(t, store, PluginID, "asterisk", "trunk_voipms", "sip_trunk", "VoIP.ms",
		SIPTrunk{Registered: true, Host: "chicago3.voip.ms", Port: 5060, Latency: 25})

	got := getEntity(t, store, PluginID, "asterisk", "trunk_voipms")
	s, ok := got.State.(SIPTrunk)
	if !ok {
		t.Fatalf("state type: got %T, want SIPTrunk", got.State)
	}
	if !s.Registered || s.Host != "chicago3.voip.ms" || s.Port != 5060 || s.Latency != 25 {
		t.Errorf("state: %+v", s)
	}
}

func TestCustom_SIPEndpoint_SaveGetHydrate(t *testing.T) {
	_, store, _ := env(t)
	saveEntity(t, store, PluginID, "asterisk", "ext_100", "sip_endpoint", "Extension 100",
		SIPEndpoint{Registered: true, InCall: false, IP: "192.168.88.50", Agent: "Yealink T54W"})

	got := getEntity(t, store, PluginID, "asterisk", "ext_100")
	s, ok := got.State.(SIPEndpoint)
	if !ok {
		t.Fatalf("state type: got %T, want SIPEndpoint", got.State)
	}
	if !s.Registered || s.InCall || s.IP != "192.168.88.50" || s.Agent != "Yealink T54W" {
		t.Errorf("state: %+v", s)
	}
}

func TestCustom_SIPCall_SaveGetHydrate(t *testing.T) {
	_, store, _ := env(t)
	saveEntity(t, store, PluginID, "asterisk", "call_abc123", "sip_call", "Active Call",
		SIPCall{State: "up", Caller: "100", Callee: "200", Duration: 120, Direction: "outbound"})

	got := getEntity(t, store, PluginID, "asterisk", "call_abc123")
	s, ok := got.State.(SIPCall)
	if !ok {
		t.Fatalf("state type: got %T, want SIPCall", got.State)
	}
	if s.State != "up" || s.Caller != "100" || s.Callee != "200" || s.Duration != 120 || s.Direction != "outbound" {
		t.Errorf("state: %+v", s)
	}
}

func TestCustom_Voicemail_SaveGetHydrate(t *testing.T) {
	_, store, _ := env(t)
	saveEntity(t, store, PluginID, "asterisk", "voicemail_100", "voicemail", "VM 100",
		Voicemail{NewMessages: 3, OldMessages: 7, Mailbox: "100"})

	got := getEntity(t, store, PluginID, "asterisk", "voicemail_100")
	s, ok := got.State.(Voicemail)
	if !ok {
		t.Fatalf("state type: got %T, want Voicemail", got.State)
	}
	if s.NewMessages != 3 || s.OldMessages != 7 || s.Mailbox != "100" {
		t.Errorf("state: %+v", s)
	}
}

func TestCustom_CallQueue_SaveGetHydrate(t *testing.T) {
	_, store, _ := env(t)
	saveEntity(t, store, PluginID, "asterisk", "queue_support", "call_queue", "Support Queue",
		CallQueue{Callers: 5, Available: 2, Strategy: "ringall", Holdtime: 45})

	got := getEntity(t, store, PluginID, "asterisk", "queue_support")
	s, ok := got.State.(CallQueue)
	if !ok {
		t.Fatalf("state type: got %T, want CallQueue", got.State)
	}
	if s.Callers != 5 || s.Available != 2 || s.Strategy != "ringall" || s.Holdtime != 45 {
		t.Errorf("state: %+v", s)
	}
}

// ==========================================================================
// Query by custom fields
// ==========================================================================

func TestCustom_QueryByType(t *testing.T) {
	_, store, _ := env(t)
	saveEntity(t, store, PluginID, "asterisk", "trunk_voipms", "sip_trunk", "VoIP.ms", SIPTrunk{Registered: true})
	saveEntity(t, store, PluginID, "asterisk", "trunk_other", "sip_trunk", "Other", SIPTrunk{Registered: false})
	saveEntity(t, store, PluginID, "asterisk", "ext_100", "sip_endpoint", "Ext 100", SIPEndpoint{Registered: true})

	entries := queryByType(t, store, "sip_trunk")
	if len(entries) != 2 {
		t.Fatalf("sip_trunks: got %d, want 2", len(entries))
	}
}

func TestCustom_QueryEndpointsInCall(t *testing.T) {
	_, store, _ := env(t)
	saveEntity(t, store, PluginID, "asterisk", "ext_100", "sip_endpoint", "Ext 100",
		SIPEndpoint{Registered: true, InCall: true})
	saveEntity(t, store, PluginID, "asterisk", "ext_200", "sip_endpoint", "Ext 200",
		SIPEndpoint{Registered: true, InCall: false})

	entries, err := store.Query(storage.Query{
		Where: []storage.Filter{
			{Field: "type", Op: storage.Eq, Value: "sip_endpoint"},
			{Field: "state.in_call", Op: storage.Eq, Value: true},
		},
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("in-call endpoints: got %d, want 1", len(entries))
	}
}

func TestCustom_QueryVoicemailWithNewMessages(t *testing.T) {
	_, store, _ := env(t)
	saveEntity(t, store, PluginID, "asterisk", "voicemail_100", "voicemail", "VM 100",
		Voicemail{NewMessages: 3, Mailbox: "100"})
	saveEntity(t, store, PluginID, "asterisk", "voicemail_200", "voicemail", "VM 200",
		Voicemail{NewMessages: 0, Mailbox: "200"})

	entries, err := store.Query(storage.Query{
		Where: []storage.Filter{
			{Field: "type", Op: storage.Eq, Value: "voicemail"},
			{Field: "state.new_messages", Op: storage.Gt, Value: float64(0)},
		},
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("voicemails with new msgs: got %d, want 1", len(entries))
	}
}

// ==========================================================================
// Command dispatch
// ==========================================================================

func TestCommand_PBXReload(t *testing.T) {
	_, _, cmds := env(t)
	entity := domain.Entity{ID: "server", Plugin: PluginID, DeviceID: "asterisk", Type: "pbx"}
	got := sendAndReceive(t, cmds, entity, PBXReload{}, PluginID+".>")
	if _, ok := got.(PBXReload); !ok {
		t.Fatalf("type: got %T, want PBXReload", got)
	}
}

func TestCommand_SIPCallOriginate(t *testing.T) {
	_, _, cmds := env(t)
	entity := domain.Entity{ID: "ext_100", Plugin: PluginID, DeviceID: "asterisk", Type: "sip_endpoint"}
	got := sendAndReceive(t, cmds, entity, SIPCallOriginate{Extension: "200", Context: "internal"}, PluginID+".>")
	cmd, ok := got.(SIPCallOriginate)
	if !ok {
		t.Fatalf("type: got %T, want SIPCallOriginate", got)
	}
	if cmd.Extension != "200" || cmd.Context != "internal" {
		t.Errorf("command: %+v", cmd)
	}
}

func TestCommand_SIPHangup(t *testing.T) {
	_, _, cmds := env(t)
	entity := domain.Entity{ID: "call_abc", Plugin: PluginID, DeviceID: "asterisk", Type: "sip_call"}
	got := sendAndReceive(t, cmds, entity, SIPHangup{Channel: "SIP/100-00000001"}, PluginID+".>")
	cmd, ok := got.(SIPHangup)
	if !ok {
		t.Fatalf("type: got %T, want SIPHangup", got)
	}
	if cmd.Channel != "SIP/100-00000001" {
		t.Errorf("channel: got %q", cmd.Channel)
	}
}

func TestCommand_SIPTransfer(t *testing.T) {
	_, _, cmds := env(t)
	entity := domain.Entity{ID: "call_abc", Plugin: PluginID, DeviceID: "asterisk", Type: "sip_call"}
	got := sendAndReceive(t, cmds, entity, SIPTransfer{Extension: "300"}, PluginID+".>")
	cmd, ok := got.(SIPTransfer)
	if !ok {
		t.Fatalf("type: got %T, want SIPTransfer", got)
	}
	if cmd.Extension != "300" {
		t.Errorf("extension: got %q", cmd.Extension)
	}
}

func TestCommand_VoicemailDelete(t *testing.T) {
	_, _, cmds := env(t)
	entity := domain.Entity{ID: "voicemail_100", Plugin: PluginID, DeviceID: "asterisk", Type: "voicemail"}
	got := sendAndReceive(t, cmds, entity, VoicemailDelete{Mailbox: "100"}, PluginID+".>")
	cmd, ok := got.(VoicemailDelete)
	if !ok {
		t.Fatalf("type: got %T, want VoicemailDelete", got)
	}
	if cmd.Mailbox != "100" {
		t.Errorf("mailbox: got %q", cmd.Mailbox)
	}
}

// ==========================================================================
// Mixed entities: all types coexist and query in isolation
// ==========================================================================

func TestMixed_QueryEachTypeInIsolation(t *testing.T) {
	_, store, _ := env(t)
	saveEntity(t, store, PluginID, "asterisk", "server", "pbx", "PBX", PBX{Connected: true})
	saveEntity(t, store, PluginID, "asterisk", "trunk_voipms", "sip_trunk", "Trunk", SIPTrunk{Registered: true})
	saveEntity(t, store, PluginID, "asterisk", "ext_100", "sip_endpoint", "Ext", SIPEndpoint{Registered: true})
	saveEntity(t, store, PluginID, "asterisk", "call_abc", "sip_call", "Call", SIPCall{State: "up"})
	saveEntity(t, store, PluginID, "asterisk", "voicemail_100", "voicemail", "VM", Voicemail{NewMessages: 1})
	saveEntity(t, store, PluginID, "asterisk", "queue_support", "call_queue", "Queue", CallQueue{Callers: 2})

	tests := []struct {
		typ   string
		count int
	}{
		{"pbx", 1},
		{"sip_trunk", 1},
		{"sip_endpoint", 1},
		{"sip_call", 1},
		{"voicemail", 1},
		{"call_queue", 1},
	}
	for _, tc := range tests {
		entries := queryByType(t, store, tc.typ)
		if len(entries) != tc.count {
			t.Errorf("%s: got %d, want %d", tc.typ, len(entries), tc.count)
		}
	}
}

func TestMixed_CustomAndBuiltinBoolField(t *testing.T) {
	_, store, _ := env(t)
	saveEntity(t, store, PluginID, "asterisk", "trunk_voipms", "sip_trunk", "Trunk",
		SIPTrunk{Registered: true})
	saveEntity(t, store, PluginID, "asterisk", "ext_100", "sip_endpoint", "Ext",
		SIPEndpoint{Registered: true, InCall: true})

	// Query state.in_call=true should only match endpoints, not trunks
	inCall, _ := store.Query(storage.Query{
		Where: []storage.Filter{{Field: "state.in_call", Op: storage.Eq, Value: true}},
	})
	if len(inCall) != 1 {
		t.Fatalf("state.in_call=true: got %d, want 1 (only endpoint)", len(inCall))
	}

	// Query state.registered=true should match both trunk and endpoint
	registered, _ := store.Query(storage.Query{
		Where: []storage.Filter{{Field: "state.registered", Op: storage.Eq, Value: true}},
	})
	if len(registered) != 2 {
		t.Fatalf("state.registered=true: got %d, want 2 (trunk+endpoint)", len(registered))
	}
}

func TestMixed_PatternIsolatesPlugin(t *testing.T) {
	_, store, _ := env(t)
	saveEntity(t, store, PluginID, "asterisk", "server", "pbx", "PBX", PBX{Connected: true})
	saveEntity(t, store, "plugin-other", "dev1", "light001", "light", "Light", domain.Light{Power: true})

	entries, _ := store.Query(storage.Query{
		Pattern: PluginID + ".>",
	})
	if len(entries) != 1 {
		t.Fatalf("asterisk pattern: got %d, want 1", len(entries))
	}

	var entity domain.Entity
	json.Unmarshal(entries[0].Data, &entity)
	if entity.Plugin != PluginID {
		t.Errorf("plugin: got %q, want %s", entity.Plugin, PluginID)
	}
}

func TestMixed_DeleteCustomDoesNotAffectOther(t *testing.T) {
	_, store, _ := env(t)
	saveEntity(t, store, PluginID, "asterisk", "trunk_voipms", "sip_trunk", "Trunk", SIPTrunk{Registered: true})
	saveEntity(t, store, PluginID, "asterisk", "ext_100", "sip_endpoint", "Ext", SIPEndpoint{Registered: true})

	store.Delete(domain.EntityKey{Plugin: PluginID, DeviceID: "asterisk", ID: "trunk_voipms"})

	entries := queryByType(t, store, "sip_trunk")
	if len(entries) != 0 {
		t.Fatalf("sip_trunks after delete: got %d, want 0", len(entries))
	}

	entries = queryByType(t, store, "sip_endpoint")
	if len(entries) != 1 {
		t.Fatalf("sip_endpoints after delete: got %d, want 1", len(entries))
	}
}

func TestMixed_OverwriteReflectsInQuery(t *testing.T) {
	_, store, _ := env(t)
	saveEntity(t, store, PluginID, "asterisk", "voicemail_100", "voicemail", "VM",
		Voicemail{NewMessages: 0, Mailbox: "100"})

	saveEntity(t, store, PluginID, "asterisk", "voicemail_100", "voicemail", "VM",
		Voicemail{NewMessages: 5, Mailbox: "100"})

	entries, _ := store.Query(storage.Query{
		Where: []storage.Filter{
			{Field: "type", Op: storage.Eq, Value: "voicemail"},
			{Field: "state.new_messages", Op: storage.Gt, Value: float64(0)},
		},
	})
	if len(entries) != 1 {
		t.Fatalf("after overwrite: got %d, want 1", len(entries))
	}
}

func TestMixed_FullLifecycle_SaveQueryCommandHydrate(t *testing.T) {
	_, store, cmds := env(t)

	saveEntity(t, store, PluginID, "asterisk", "server", "pbx", "Asterisk PBX",
		PBX{Connected: true, Version: "20.5.0"})
	saveEntity(t, store, PluginID, "asterisk", "ext_100", "sip_endpoint", "Extension 100",
		SIPEndpoint{Registered: true, InCall: false})

	// Query for all pbx entities
	pbxEntities := queryByType(t, store, "pbx")
	if len(pbxEntities) != 1 {
		t.Fatalf("pbx entities: got %d, want 1", len(pbxEntities))
	}

	// Hydrate the PBX from query result
	var pbxEntity domain.Entity
	if err := json.Unmarshal(pbxEntities[0].Data, &pbxEntity); err != nil {
		t.Fatalf("unmarshal pbx: %v", err)
	}
	s, ok := pbxEntity.State.(PBX)
	if !ok {
		t.Fatalf("hydrated pbx: got %T, want PBX", pbxEntity.State)
	}
	if s.Version != "20.5.0" {
		t.Errorf("version: got %q, want 20.5.0", s.Version)
	}

	// Send custom command to the PBX
	got := sendAndReceive(t, cmds, pbxEntity, PBXReload{}, PluginID+".>")
	if _, ok := got.(PBXReload); !ok {
		t.Fatalf("command type: got %T, want PBXReload", got)
	}

	// Send custom command to the endpoint
	endpointEntity := domain.Entity{ID: "ext_100", Plugin: PluginID, DeviceID: "asterisk", Type: "sip_endpoint"}
	gotCall := sendAndReceive(t, cmds, endpointEntity,
		SIPCallOriginate{Extension: "200"}, PluginID+".>")
	originate, ok := gotCall.(SIPCallOriginate)
	if !ok {
		t.Fatalf("command type: got %T, want SIPCallOriginate", gotCall)
	}
	if originate.Extension != "200" {
		t.Errorf("extension: got %q, want 200", originate.Extension)
	}
}
