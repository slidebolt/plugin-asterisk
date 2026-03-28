package app

import (
	"encoding/json"
	"reflect"
	"testing"

	domain "github.com/slidebolt/sb-domain"
	testkit "github.com/slidebolt/sb-testkit"
)

func TestOnStart_PersistsSeededEntityData(t *testing.T) {
	env := testkit.NewTestEnv(t)
	env.Start("messenger")
	env.Start("storage")

	deps := map[string]json.RawMessage{
		"messenger": env.MessengerPayload(),
	}

	app := New()
	if _, err := app.OnStart(deps); err != nil {
		t.Fatalf("OnStart: %v", err)
	}
	t.Cleanup(func() { _ = app.OnShutdown() })

	raw, err := env.Storage().Get(domain.EntityKey{
		Plugin:   PluginID,
		DeviceID: "asterisk",
		ID:       "server",
	})
	if err != nil {
		t.Fatalf("get seeded entity: %v", err)
	}

	var got domain.Entity
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal seeded entity: %v", err)
	}

	if got.Plugin != PluginID || got.DeviceID != "asterisk" || got.ID != "server" {
		t.Fatalf("identity = %s.%s.%s", got.Plugin, got.DeviceID, got.ID)
	}
	if got.Type != "pbx" || got.Name != "Asterisk PBX" {
		t.Fatalf("entity metadata = type:%q name:%q", got.Type, got.Name)
	}

	wantCommands := []string{"pbx_reload"}
	if !reflect.DeepEqual(got.Commands, wantCommands) {
		t.Fatalf("commands = %v, want %v", got.Commands, wantCommands)
	}

	pbx, ok := got.State.(PBX)
	if !ok {
		t.Fatalf("state type = %T, want PBX", got.State)
	}
	if pbx.Connected {
		t.Fatalf("state = %+v, want connected=false", pbx)
	}
}

func TestOnStart_PersistsTrunkEntity(t *testing.T) {
	env := testkit.NewTestEnv(t)
	env.Start("messenger")
	env.Start("storage")

	deps := map[string]json.RawMessage{
		"messenger": env.MessengerPayload(),
	}

	app := New()
	if _, err := app.OnStart(deps); err != nil {
		t.Fatalf("OnStart: %v", err)
	}
	t.Cleanup(func() { _ = app.OnShutdown() })

	raw, err := env.Storage().Get(domain.EntityKey{
		Plugin:   PluginID,
		DeviceID: "asterisk",
		ID:       "trunk_voipms",
	})
	if err != nil {
		t.Fatalf("get seeded trunk: %v", err)
	}

	var got domain.Entity
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Type != "sip_trunk" {
		t.Fatalf("type = %q, want sip_trunk", got.Type)
	}

	trunk, ok := got.State.(SIPTrunk)
	if !ok {
		t.Fatalf("state type = %T, want SIPTrunk", got.State)
	}
	if trunk.Registered || trunk.Host != "chicago.voip.ms" {
		t.Fatalf("state = %+v", trunk)
	}
}

func TestOnStart_PersistsEndpointEntity(t *testing.T) {
	env := testkit.NewTestEnv(t)
	env.Start("messenger")
	env.Start("storage")

	deps := map[string]json.RawMessage{
		"messenger": env.MessengerPayload(),
	}

	app := New()
	if _, err := app.OnStart(deps); err != nil {
		t.Fatalf("OnStart: %v", err)
	}
	t.Cleanup(func() { _ = app.OnShutdown() })

	raw, err := env.Storage().Get(domain.EntityKey{
		Plugin:   PluginID,
		DeviceID: "asterisk",
		ID:       "ext_201",
	})
	if err != nil {
		t.Fatalf("get seeded endpoint: %v", err)
	}

	var got domain.Entity
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Type != "sip_endpoint" {
		t.Fatalf("type = %q, want sip_endpoint", got.Type)
	}

	wantCmds := []string{"sip_call", "sip_hangup"}
	if !reflect.DeepEqual(got.Commands, wantCmds) {
		t.Fatalf("commands = %v, want %v", got.Commands, wantCmds)
	}
}

func TestOnStart_PersistsAllEndpoints(t *testing.T) {
	env := testkit.NewTestEnv(t)
	env.Start("messenger")
	env.Start("storage")

	deps := map[string]json.RawMessage{
		"messenger": env.MessengerPayload(),
	}

	app := New()
	if _, err := app.OnStart(deps); err != nil {
		t.Fatalf("OnStart: %v", err)
	}
	t.Cleanup(func() { _ = app.OnShutdown() })

	// Verify all 3 real extensions are seeded
	for _, ext := range []struct{ id, name string }{
		{"ext_201", "Gavin (Phone)"},
		{"ext_202", "Gavin (Laptop)"},
		{"ext_3000", "Intercom"},
	} {
		raw, err := env.Storage().Get(domain.EntityKey{
			Plugin: PluginID, DeviceID: "asterisk", ID: ext.id,
		})
		if err != nil {
			t.Fatalf("get %s: %v", ext.id, err)
		}
		var got domain.Entity
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("unmarshal %s: %v", ext.id, err)
		}
		if got.Name != ext.name {
			t.Errorf("%s name: got %q, want %q", ext.id, got.Name, ext.name)
		}
	}
}
