package app

import (
	"encoding/json"
	"reflect"
	"testing"

	domain "github.com/slidebolt/sb-domain"
	testkit "github.com/slidebolt/sb-testkit"
)

func startApp(t *testing.T) (*testkit.TestEnv, *App) {
	t.Helper()
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
	return env, app
}

func TestOnStart_SeedsPBXDeviceAndEntity(t *testing.T) {
	env, _ := startApp(t)

	// Device record (2-part key)
	rawDev, err := env.Storage().Get(domain.DeviceKey{Plugin: PluginID, ID: "server"})
	if err != nil {
		t.Fatalf("get pbx device: %v", err)
	}
	var dev domain.Device
	if err := json.Unmarshal(rawDev, &dev); err != nil {
		t.Fatalf("unmarshal device: %v", err)
	}
	if dev.ID != "server" || dev.Plugin != PluginID || dev.Name != "Asterisk PBX" {
		t.Fatalf("device = %+v", dev)
	}

	// Primary entity (3-part key)
	rawEnt, err := env.Storage().Get(domain.EntityKey{
		Plugin: PluginID, DeviceID: "server", ID: "status",
	})
	if err != nil {
		t.Fatalf("get pbx entity: %v", err)
	}
	var ent domain.Entity
	if err := json.Unmarshal(rawEnt, &ent); err != nil {
		t.Fatalf("unmarshal entity: %v", err)
	}
	if ent.Type != "pbx" {
		t.Fatalf("type = %q, want pbx", ent.Type)
	}
	if !reflect.DeepEqual(ent.Commands, []string{"pbx_reload"}) {
		t.Fatalf("commands = %v", ent.Commands)
	}
	if _, ok := ent.State.(PBX); !ok {
		t.Fatalf("state type = %T, want PBX", ent.State)
	}
}

func TestOnStart_SeedsTrunkDeviceAndEntity(t *testing.T) {
	env, _ := startApp(t)

	if _, err := env.Storage().Get(domain.DeviceKey{Plugin: PluginID, ID: "trunk_voipms"}); err != nil {
		t.Fatalf("get trunk device: %v", err)
	}
	raw, err := env.Storage().Get(domain.EntityKey{
		Plugin: PluginID, DeviceID: "trunk_voipms", ID: "registration",
	})
	if err != nil {
		t.Fatalf("get trunk entity: %v", err)
	}
	var ent domain.Entity
	if err := json.Unmarshal(raw, &ent); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ent.Type != "sip_trunk" {
		t.Fatalf("type = %q", ent.Type)
	}
	trunk, ok := ent.State.(SIPTrunk)
	if !ok {
		t.Fatalf("state type = %T, want SIPTrunk", ent.State)
	}
	if trunk.Host != "chicago.voip.ms" {
		t.Fatalf("host = %q", trunk.Host)
	}
}

func TestOnStart_SeedsAllEndpointDevicesAndEntities(t *testing.T) {
	env, _ := startApp(t)

	for _, ext := range []struct{ id, name string }{
		{"ext_201", "Gavin (Phone)"},
		{"ext_202", "Gavin (Laptop)"},
		{"ext_3000", "Intercom"},
	} {
		rawDev, err := env.Storage().Get(domain.DeviceKey{Plugin: PluginID, ID: ext.id})
		if err != nil {
			t.Fatalf("get device %s: %v", ext.id, err)
		}
		var dev domain.Device
		if err := json.Unmarshal(rawDev, &dev); err != nil {
			t.Fatalf("unmarshal device %s: %v", ext.id, err)
		}
		if dev.Name != ext.name {
			t.Errorf("%s device name = %q, want %q", ext.id, dev.Name, ext.name)
		}

		rawEnt, err := env.Storage().Get(domain.EntityKey{
			Plugin: PluginID, DeviceID: ext.id, ID: "registration",
		})
		if err != nil {
			t.Fatalf("get entity %s: %v", ext.id, err)
		}
		var ent domain.Entity
		if err := json.Unmarshal(rawEnt, &ent); err != nil {
			t.Fatalf("unmarshal entity %s: %v", ext.id, err)
		}
		wantCmds := []string{"sip_call", "sip_hangup"}
		if !reflect.DeepEqual(ent.Commands, wantCmds) {
			t.Errorf("%s commands = %v", ext.id, ent.Commands)
		}
	}
}
