//go:build integration

// Integration tests that hit the real Asterisk ARI API.
//
// These require a running Asterisk instance. Configure via environment variables
// (copy .env.integration.example to .env.integration and fill in values):
//
//	ASTERISK_ARI_URL=http://<host>:8088
//	ASTERISK_ARI_USER=<username>
//	ASTERISK_ARI_PASS=<password>
//
// Run:
//
//	go test -tags integration -v ./app/...
package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

func ariURL() string {
	v := os.Getenv("ASTERISK_ARI_URL")
	if v == "" {
		panic("ASTERISK_ARI_URL env var is required for integration tests")
	}
	return v
}

func ariUser() string {
	v := os.Getenv("ASTERISK_ARI_USER")
	if v == "" {
		panic("ASTERISK_ARI_USER env var is required for integration tests")
	}
	return v
}

func ariPass() string {
	v := os.Getenv("ASTERISK_ARI_PASS")
	if v == "" {
		panic("ASTERISK_ARI_PASS env var is required for integration tests")
	}
	return v
}

func ariGet(t *testing.T, path string) json.RawMessage {
	t.Helper()
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", ariURL()+path, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.SetBasicAuth(ariUser(), ariPass())
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("ARI request %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("ARI %s returned %d: %s", path, resp.StatusCode, body)
	}
	return json.RawMessage(body)
}

// --- ARI response types ---

type ariAsteriskInfo struct {
	System struct {
		Version  string `json:"version"`
		EntityID string `json:"entity_id"`
	} `json:"system"`
	Status struct {
		StartupTime    string `json:"startup_time"`
		LastReloadTime string `json:"last_reload_time"`
	} `json:"status"`
}

type ariEndpoint struct {
	Technology string   `json:"technology"`
	Resource   string   `json:"resource"`
	State      string   `json:"state"`
	ChannelIDs []string `json:"channel_ids"`
}

type ariChannel struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	State string `json:"state"`
	Caller struct {
		Name   string `json:"name"`
		Number string `json:"number"`
	} `json:"caller"`
	Connected struct {
		Name   string `json:"name"`
		Number string `json:"number"`
	} `json:"connected"`
	Creationtime string `json:"creationtime"`
}

// ==========================================================================
// Server info
// ==========================================================================

func TestARI_ServerInfo(t *testing.T) {
	raw := ariGet(t, "/ari/asterisk/info")
	var info ariAsteriskInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if info.System.Version == "" {
		t.Fatal("version is empty")
	}
	t.Logf("Asterisk version: %s", info.System.Version)
	t.Logf("Entity ID: %s", info.System.EntityID)
	t.Logf("Startup: %s", info.Status.StartupTime)

	// Can hydrate our PBX entity from this
	pbx := PBX{
		Connected: true,
		Version:   info.System.Version,
	}
	if !pbx.Connected || pbx.Version == "" {
		t.Errorf("PBX state: %+v", pbx)
	}
}

// ==========================================================================
// Endpoints (extensions + trunks)
// ==========================================================================

func TestARI_ListEndpoints(t *testing.T) {
	raw := ariGet(t, "/ari/endpoints")
	var endpoints []ariEndpoint
	if err := json.Unmarshal(raw, &endpoints); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(endpoints) == 0 {
		t.Fatal("no endpoints returned")
	}

	var trunks, extensions []ariEndpoint
	for _, ep := range endpoints {
		t.Logf("endpoint: %s/%s state=%s channels=%d",
			ep.Technology, ep.Resource, ep.State, len(ep.ChannelIDs))

		// Classify: trunk vs extension
		switch ep.Resource {
		case "voipms":
			trunks = append(trunks, ep)
		default:
			extensions = append(extensions, ep)
		}
	}

	// Verify we found the expected topology
	if len(trunks) != 1 {
		t.Errorf("trunks: got %d, want 1", len(trunks))
	}
	if trunks[0].Resource != "voipms" {
		t.Errorf("trunk resource: got %q, want voipms", trunks[0].Resource)
	}

	// Verify the trunk can hydrate to our SIPTrunk type
	trunk := SIPTrunk{
		Registered: trunks[0].State == "online",
		Host:       "chicago.voip.ms",
	}
	t.Logf("SIPTrunk: %+v", trunk)

	// Verify extensions are present
	if len(extensions) < 3 {
		t.Errorf("extensions: got %d, want at least 3 (201, 202, 3000)", len(extensions))
	}

	// Map extensions to SIPEndpoint entities
	for _, ep := range extensions {
		endpoint := SIPEndpoint{
			Registered: ep.State == "online",
			InCall:     len(ep.ChannelIDs) > 0,
		}
		t.Logf("SIPEndpoint %s: %+v", ep.Resource, endpoint)
	}
}

func TestARI_EndpointDetail(t *testing.T) {
	// Test individual endpoint lookup
	for _, resource := range []string{"201", "202", "voipms"} {
		t.Run(resource, func(t *testing.T) {
			raw := ariGet(t, fmt.Sprintf("/ari/endpoints/PJSIP/%s", resource))
			var ep ariEndpoint
			if err := json.Unmarshal(raw, &ep); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if ep.Resource != resource {
				t.Errorf("resource: got %q, want %q", ep.Resource, resource)
			}
			if ep.Technology != "PJSIP" {
				t.Errorf("technology: got %q, want PJSIP", ep.Technology)
			}
			t.Logf("%s: state=%s channels=%d", resource, ep.State, len(ep.ChannelIDs))
		})
	}
}

// ==========================================================================
// Active channels (calls)
// ==========================================================================

func TestARI_ListChannels(t *testing.T) {
	raw := ariGet(t, "/ari/channels")
	var channels []ariChannel
	if err := json.Unmarshal(raw, &channels); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	t.Logf("active channels: %d", len(channels))
	for _, ch := range channels {
		call := SIPCall{
			State:  ch.State,
			Caller: ch.Caller.Number,
			Callee: ch.Connected.Number,
		}
		t.Logf("SIPCall %s: %+v", ch.ID, call)
	}
}

// ==========================================================================
// Full discovery: build the real entity set from live ARI data
// ==========================================================================

func TestARI_FullDiscovery(t *testing.T) {
	// 1. Server info → PBX entity
	infoRaw := ariGet(t, "/ari/asterisk/info")
	var info ariAsteriskInfo
	json.Unmarshal(infoRaw, &info)

	pbx := PBX{Connected: true, Version: info.System.Version}
	t.Logf("PBX: version=%s connected=%v", pbx.Version, pbx.Connected)

	// 2. Endpoints → trunks + extensions
	epRaw := ariGet(t, "/ari/endpoints")
	var endpoints []ariEndpoint
	json.Unmarshal(epRaw, &endpoints)

	var trunkCount, extCount, onlineCount int
	for _, ep := range endpoints {
		switch ep.Resource {
		case "voipms":
			trunkCount++
			trunk := SIPTrunk{
				Registered: ep.State == "online",
				Host:       "chicago.voip.ms",
				Port:       5060,
			}
			t.Logf("Trunk %s: registered=%v host=%s", ep.Resource, trunk.Registered, trunk.Host)
		default:
			extCount++
			endpoint := SIPEndpoint{
				Registered: ep.State == "online",
				InCall:     len(ep.ChannelIDs) > 0,
			}
			if endpoint.Registered {
				onlineCount++
			}
			t.Logf("Endpoint %s: registered=%v in_call=%v", ep.Resource, endpoint.Registered, endpoint.InCall)
		}
	}

	// 3. Active channels → SIPCall entities
	chRaw := ariGet(t, "/ari/channels")
	var channels []ariChannel
	json.Unmarshal(chRaw, &channels)

	t.Logf("--- Discovery Summary ---")
	t.Logf("PBX: Asterisk %s", pbx.Version)
	t.Logf("Trunks: %d", trunkCount)
	t.Logf("Extensions: %d total, %d online", extCount, onlineCount)
	t.Logf("Active calls: %d", len(channels))

	// Assertions against known production topology
	if trunkCount != 1 {
		t.Errorf("expected 1 trunk, got %d", trunkCount)
	}
	if extCount < 13 {
		t.Errorf("expected at least 13 extensions (201,202,3000,float-10000..10010), got %d", extCount)
	}
}
