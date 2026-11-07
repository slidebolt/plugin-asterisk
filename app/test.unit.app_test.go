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
	case <-time.After(15 * time.Second):
		t.Fatal("timed out waiting for command")
		return nil
	}
}

// Primary-entity IDs per device type. One device = one thing; the primary
// entity is the canonical state-bearer for that device.
const (
	pbxEntity      = "status"
	trunkEntity    = "registration"
	endpointEntity = "registration"
	callEntity     = "state"
	vmEntity       = "mailbox"
	queueEntity    = "stats"
)

// ==========================================================================
// Internal storage: plugin-private data, invisible to query/search
// ==========================================================================

func TestInternal_WriteReadDelete(t *testing.T) {
	_, store, _ := env(t)
	key := domain.EntityKey{Plugin: PluginID, DeviceID: "server", ID: pbxEntity}
	payload := json.RawMessage(`{"ariEndpoint":"http://asterisk.example:8088","username":"admin"}`)

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
	key := domain.EntityKey{Plugin: PluginID, DeviceID: "server", ID: pbxEntity}

	saveEntity(t, store, PluginID, "server", pbxEntity, "pbx", "PBX", PBX{Connected: true})
	store.WriteFile(storage.Internal, key, json.RawMessage(`{"ariEndpoint":"http://asterisk.example:8088"}`))

	entries := queryByType(t, store, "pbx")
	if len(entries) != 1 {
		t.Fatalf("query: got %d results, want 1", len(entries))
	}
}

func TestInternal_NotVisibleInSearch(t *testing.T) {
	_, store, _ := env(t)
	key := domain.EntityKey{Plugin: PluginID, DeviceID: "server", ID: pbxEntity}

	saveEntity(t, store, PluginID, "server", pbxEntity, "pbx", "PBX", PBX{Connected: true})
	store.WriteFile(storage.Internal, key, json.RawMessage(`{"ariEndpoint":"http://asterisk.example:8088"}`))

	entries, err := store.Search(PluginID + ".>")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("search: got %d results, want 1", len(entries))
	}
	want := PluginID + ".server." + pbxEntity
	if entries[0].Key != want {
		t.Errorf("search result key: got %q, want %s", entries[0].Key, want)
	}
}

// ==========================================================================
// Cross-cutting: multi-plugin isolation
// ==========================================================================

func TestCrossCutting_MultiPluginIsolation(t *testing.T) {
	_, store, _ := env(t)
	saveEntity(t, store, PluginID, "server", pbxEntity, "pbx", "Asterisk PBX", PBX{Connected: true})
	saveEntity(t, store, "plugin-other", "server", "status", "pbx", "Other PBX", PBX{Connected: false})

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
	saveEntity(t, store, PluginID, "trunk_main", trunkEntity, "sip_trunk", "Main Trunk",
		SIPTrunk{Registered: true})
	saveEntity(t, store, PluginID, "trunk_other", trunkEntity, "sip_trunk", "Other Trunk",
		SIPTrunk{Registered: false})
	saveEntity(t, store, PluginID, "ext_100", endpointEntity, "sip_endpoint", "Ext 100",
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
	saveEntity(t, store, PluginID, "server", pbxEntity, "pbx", "Asterisk PBX",
		PBX{Connected: true, Version: "20.5.0", Uptime: 86400})

	got := getEntity(t, store, PluginID, "server", pbxEntity)
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
	saveEntity(t, store, PluginID, "trunk_main", trunkEntity, "sip_trunk", "Main Trunk",
		SIPTrunk{Registered: true, Host: "sip.example.com", Port: 5060, Latency: 25})

	got := getEntity(t, store, PluginID, "trunk_main", trunkEntity)
	s, ok := got.State.(SIPTrunk)
	if !ok {
		t.Fatalf("state type: got %T, want SIPTrunk", got.State)
	}
	if !s.Registered || s.Host != "sip.example.com" || s.Port != 5060 || s.Latency != 25 {
		t.Errorf("state: %+v", s)
	}
}

func TestCustom_SIPEndpoint_SaveGetHydrate(t *testing.T) {
	_, store, _ := env(t)
	saveEntity(t, store, PluginID, "ext_100", endpointEntity, "sip_endpoint", "Extension 100",
		SIPEndpoint{Registered: true, InCall: false, IP: "10.0.0.50", Agent: "Yealink T54W"})

	got := getEntity(t, store, PluginID, "ext_100", endpointEntity)
	s, ok := got.State.(SIPEndpoint)
	if !ok {
		t.Fatalf("state type: got %T, want SIPEndpoint", got.State)
	}
	if !s.Registered || s.InCall || s.IP != "10.0.0.50" || s.Agent != "Yealink T54W" {
		t.Errorf("state: %+v", s)
	}
}

func TestCustom_SIPCall_SaveGetHydrate(t *testing.T) {
	_, store, _ := env(t)
	saveEntity(t, store, PluginID, "call_abc123", callEntity, "sip_call", "Active Call",
		SIPCall{State: "up", Caller: "100", Callee: "200", Duration: 120, Direction: "outbound"})

	got := getEntity(t, store, PluginID, "call_abc123", callEntity)
	s, ok := got.State.(SIPCall)
	if !ok {
		t.Fatalf("state type: got %T, want SIPCall", got.State)
	}
	if s.State != "up" || s.Caller != "100" || s.Callee != "200" || s.Duration != 120 || s.Direction != "outbound" {
		t.Errorf("state: %+v", s)
	}
}

// ==========================================================================
// Query by custom fields
// ==========================================================================

func TestCustom_QueryByType(t *testing.T) {
	_, store, _ := env(t)
	saveEntity(t, store, PluginID, "trunk_main", trunkEntity, "sip_trunk", "Main Trunk", SIPTrunk{Registered: true})
	saveEntity(t, store, PluginID, "trunk_other", trunkEntity, "sip_trunk", "Other", SIPTrunk{Registered: false})
	saveEntity(t, store, PluginID, "ext_100", endpointEntity, "sip_endpoint", "Ext 100", SIPEndpoint{Registered: true})

	entries := queryByType(t, store, "sip_trunk")
	if len(entries) != 2 {
		t.Fatalf("sip_trunks: got %d, want 2", len(entries))
	}
}

func TestCustom_QueryEndpointsInCall(t *testing.T) {
	_, store, _ := env(t)
	saveEntity(t, store, PluginID, "ext_100", endpointEntity, "sip_endpoint", "Ext 100",
		SIPEndpoint{Registered: true, InCall: true})
	saveEntity(t, store, PluginID, "ext_200", endpointEntity, "sip_endpoint", "Ext 200",
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

// ==========================================================================
// Command dispatch
// ==========================================================================

func TestCommand_PBXReload(t *testing.T) {
	_, _, cmds := env(t)
	entity := domain.Entity{ID: pbxEntity, Plugin: PluginID, DeviceID: "server", Type: "pbx"}
	got := sendAndReceive(t, cmds, entity, domain.PBXReload{}, PluginID+".>")
	if _, ok := got.(domain.PBXReload); !ok {
		t.Fatalf("type: got %T, want PBXReload", got)
	}
}

func TestCommand_SIPDial(t *testing.T) {
	_, _, cmds := env(t)
	entity := domain.Entity{ID: endpointEntity, Plugin: PluginID, DeviceID: "ext_100", Type: "sip_endpoint"}
	got := sendAndReceive(t, cmds, entity, domain.SIPDial{Call: "front-door", RingSeconds: 30}, PluginID+".>")
	cmd, ok := got.(domain.SIPDial)
	if !ok {
		t.Fatalf("type: got %T, want SIPDial", got)
	}
	if cmd.Call != "front-door" || cmd.RingSeconds != 30 {
		t.Errorf("command: %+v", cmd)
	}
}

func TestCommand_SIPHangup(t *testing.T) {
	_, _, cmds := env(t)
	entity := domain.Entity{ID: callEntity, Plugin: PluginID, DeviceID: "call_abc", Type: "sip_call"}
	got := sendAndReceive(t, cmds, entity, domain.SIPHangup{}, PluginID+".>")
	if _, ok := got.(domain.SIPHangup); !ok {
		t.Fatalf("type: got %T, want SIPHangup", got)
	}
}

// ==========================================================================
// Mixed entities: all types coexist and query in isolation
// ==========================================================================

func TestMixed_CustomAndBuiltinBoolField(t *testing.T) {
	_, store, _ := env(t)
	saveEntity(t, store, PluginID, "trunk_main", trunkEntity, "sip_trunk", "Trunk",
		SIPTrunk{Registered: true})
	saveEntity(t, store, PluginID, "ext_100", endpointEntity, "sip_endpoint", "Ext",
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
	saveEntity(t, store, PluginID, "server", pbxEntity, "pbx", "PBX", PBX{Connected: true})
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
	saveEntity(t, store, PluginID, "trunk_main", trunkEntity, "sip_trunk", "Trunk", SIPTrunk{Registered: true})
	saveEntity(t, store, PluginID, "ext_100", endpointEntity, "sip_endpoint", "Ext", SIPEndpoint{Registered: true})

	store.Delete(domain.EntityKey{Plugin: PluginID, DeviceID: "trunk_main", ID: trunkEntity})

	entries := queryByType(t, store, "sip_trunk")
	if len(entries) != 0 {
		t.Fatalf("sip_trunks after delete: got %d, want 0", len(entries))
	}

	entries = queryByType(t, store, "sip_endpoint")
	if len(entries) != 1 {
		t.Fatalf("sip_endpoints after delete: got %d, want 1", len(entries))
	}
}

func TestMixed_FullLifecycle_SaveQueryCommandHydrate(t *testing.T) {
	_, store, cmds := env(t)

	saveEntity(t, store, PluginID, "server", pbxEntity, "pbx", "Asterisk PBX",
		PBX{Connected: true, Version: "20.5.0"})
	saveEntity(t, store, PluginID, "ext_100", endpointEntity, "sip_endpoint", "Extension 100",
		SIPEndpoint{Registered: true, InCall: false})

	// Query for all pbx entities
	pbxEntities := queryByType(t, store, "pbx")
	if len(pbxEntities) != 1 {
		t.Fatalf("pbx entities: got %d, want 1", len(pbxEntities))
	}

	// Hydrate the PBX from query result
	var pbxEnt domain.Entity
	if err := json.Unmarshal(pbxEntities[0].Data, &pbxEnt); err != nil {
		t.Fatalf("unmarshal pbx: %v", err)
	}
	s, ok := pbxEnt.State.(PBX)
	if !ok {
		t.Fatalf("hydrated pbx: got %T, want PBX", pbxEnt.State)
	}
	if s.Version != "20.5.0" {
		t.Errorf("version: got %q, want 20.5.0", s.Version)
	}

	// Send custom command to the PBX
	got := sendAndReceive(t, cmds, pbxEnt, domain.PBXReload{}, PluginID+".>")
	if _, ok := got.(domain.PBXReload); !ok {
		t.Fatalf("command type: got %T, want PBXReload", got)
	}

	// Send custom command to the endpoint
	endpointEnt := domain.Entity{ID: endpointEntity, Plugin: PluginID, DeviceID: "ext_100", Type: "sip_endpoint"}
	gotCall := sendAndReceive(t, cmds, endpointEnt, domain.SIPDial{Call: "c"}, PluginID+".>")
	if dial, ok := gotCall.(domain.SIPDial); !ok || dial.Call != "c" {
		t.Fatalf("command: got %T %+v", gotCall, gotCall)
	}
}
