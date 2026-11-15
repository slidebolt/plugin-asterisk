package app

import (
	"encoding/json"
	"testing"
)

func TestNewHelloManifest(t *testing.T) {
	h := New().Hello()
	if h.ID != PluginID {
		t.Fatalf("id: got %q want %q", h.ID, PluginID)
	}
	if len(h.DependsOn) != 3 || h.DependsOn[2] != "asterisk-host" {
		t.Fatalf("dependsOn: got %v", h.DependsOn)
	}
	if h.Setup == nil || h.Setup.Mode != "selectable" {
		t.Fatalf("setup: %+v", h.Setup)
	}
}

func TestHostConnectionPrefersDependencyPayload(t *testing.T) {
	t.Setenv("ASTERISK_HOST_URL", "http://environment.invalid")
	t.Setenv("ASTERISK_HOST_TOKEN", "environment")
	url, token, err := hostConnection(map[string]json.RawMessage{
		"asterisk-host": json.RawMessage(`{"url":"http://127.0.0.1:47832","token":"dependency"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if url != "http://127.0.0.1:47832" || token != "dependency" {
		t.Fatalf("connection = %q %q", url, token)
	}
}
