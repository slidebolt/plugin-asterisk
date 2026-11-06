package app

import (
	"encoding/json"
	"testing"

	domain "github.com/slidebolt/sb-domain"
	testkit "github.com/slidebolt/sb-testkit"
)

// A failed OnStart aborts sb-manager's startAll and stops every binary, so a
// missing host must only show up as a state.
func TestOnStart_NoHostConfigStaysUpAndReportsIt(t *testing.T) {
	t.Setenv("ASTERISK_HOST_URL", "")
	t.Setenv("ASTERISK_HOST_TOKEN", "")
	env := testkit.NewTestEnv(t)
	env.Start("messenger")
	env.Start("storage")
	app := New()
	if _, err := app.OnStart(map[string]json.RawMessage{"messenger": env.MessengerPayload()}); err != nil {
		t.Fatalf("OnStart must not fail: %v", err)
	}
	t.Cleanup(func() { _ = app.OnShutdown() })

	raw, err := env.Storage().Get(domain.EntityKey{Plugin: PluginID, DeviceID: "server", ID: "status"})
	if err != nil {
		t.Fatalf("get server status: %v", err)
	}
	var e domain.Entity
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatal(err)
	}
	s, ok := e.State.(domain.PBX)
	if !ok || s.Connected || s.Error == "" {
		t.Fatalf("state = %T %+v", e.State, e.State)
	}
	entries, err := env.Storage().Search(PluginID + ".ext_201.>")
	if err != nil || len(entries) != 0 {
		t.Fatalf("devices must come from the host, not seeds: %v %v", entries, err)
	}
}
