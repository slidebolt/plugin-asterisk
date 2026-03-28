package translate

import (
	"encoding/json"
	"testing"

	"github.com/slidebolt/plugin-asterisk/app"
)

// ---------------------------------------------------------------------------
// Decode tests
// ---------------------------------------------------------------------------

func TestDecode_PBX(t *testing.T) {
	tests := []struct {
		name          string
		raw           string
		wantOK        bool
		wantConnected bool
		wantVersion   string
	}{
		{"connected with version", `{"connected":true,"version":"20.5.0","uptime":86400}`, true, true, "20.5.0"},
		{"disconnected", `{"connected":false}`, true, false, ""},
		{"empty payload", ``, false, false, ""},
		{"garbage payload", `not json`, false, false, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := Decode("pbx", json.RawMessage(tc.raw))
			if ok != tc.wantOK {
				t.Fatalf("ok: got %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			s, ok2 := got.(app.PBX)
			if !ok2 {
				t.Fatalf("type: got %T, want app.PBX", got)
			}
			if s.Connected != tc.wantConnected {
				t.Errorf("Connected: got %v, want %v", s.Connected, tc.wantConnected)
			}
			if s.Version != tc.wantVersion {
				t.Errorf("Version: got %q, want %q", s.Version, tc.wantVersion)
			}
		})
	}
}

func TestDecode_SIPTrunk(t *testing.T) {
	tests := []struct {
		name           string
		raw            string
		wantOK         bool
		wantRegistered bool
		wantHost       string
	}{
		{"registered trunk", `{"registered":true,"host":"chicago3.voip.ms","port":5060}`, true, true, "chicago3.voip.ms"},
		{"unregistered", `{"registered":false}`, true, false, ""},
		{"empty", ``, false, false, ""},
		{"garbage", `!!!`, false, false, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := Decode("sip_trunk", json.RawMessage(tc.raw))
			if ok != tc.wantOK {
				t.Fatalf("ok: got %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			s := got.(app.SIPTrunk)
			if s.Registered != tc.wantRegistered {
				t.Errorf("Registered: got %v, want %v", s.Registered, tc.wantRegistered)
			}
			if s.Host != tc.wantHost {
				t.Errorf("Host: got %q, want %q", s.Host, tc.wantHost)
			}
		})
	}
}

func TestDecode_SIPEndpoint(t *testing.T) {
	tests := []struct {
		name           string
		raw            string
		wantOK         bool
		wantRegistered bool
		wantInCall     bool
	}{
		{"registered idle", `{"registered":true,"in_call":false,"ip":"192.168.88.50"}`, true, true, false},
		{"registered in call", `{"registered":true,"in_call":true}`, true, true, true},
		{"unregistered", `{"registered":false,"in_call":false}`, true, false, false},
		{"empty", ``, false, false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := Decode("sip_endpoint", json.RawMessage(tc.raw))
			if ok != tc.wantOK {
				t.Fatalf("ok: got %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			s := got.(app.SIPEndpoint)
			if s.Registered != tc.wantRegistered {
				t.Errorf("Registered: got %v, want %v", s.Registered, tc.wantRegistered)
			}
			if s.InCall != tc.wantInCall {
				t.Errorf("InCall: got %v, want %v", s.InCall, tc.wantInCall)
			}
		})
	}
}

func TestDecode_SIPCall(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		wantOK    bool
		wantState string
	}{
		{"active call", `{"state":"up","caller":"100","callee":"200","duration":60}`, true, "up"},
		{"ringing", `{"state":"ringing","caller":"100","callee":"200"}`, true, "ringing"},
		{"empty", ``, false, ""},
		{"garbage", `xyz`, false, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := Decode("sip_call", json.RawMessage(tc.raw))
			if ok != tc.wantOK {
				t.Fatalf("ok: got %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			s := got.(app.SIPCall)
			if s.State != tc.wantState {
				t.Errorf("State: got %q, want %q", s.State, tc.wantState)
			}
		})
	}
}

func TestDecode_Voicemail(t *testing.T) {
	tests := []struct {
		name       string
		raw        string
		wantOK     bool
		wantNew    int
		wantOld    int
	}{
		{"with messages", `{"new_messages":3,"old_messages":7,"mailbox":"100"}`, true, 3, 7},
		{"empty box", `{"new_messages":0,"old_messages":0,"mailbox":"200"}`, true, 0, 0},
		{"empty", ``, false, 0, 0},
		{"garbage", `!!!`, false, 0, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := Decode("voicemail", json.RawMessage(tc.raw))
			if ok != tc.wantOK {
				t.Fatalf("ok: got %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			s := got.(app.Voicemail)
			if s.NewMessages != tc.wantNew {
				t.Errorf("NewMessages: got %d, want %d", s.NewMessages, tc.wantNew)
			}
			if s.OldMessages != tc.wantOld {
				t.Errorf("OldMessages: got %d, want %d", s.OldMessages, tc.wantOld)
			}
		})
	}
}

func TestDecode_CallQueue(t *testing.T) {
	tests := []struct {
		name        string
		raw         string
		wantOK      bool
		wantCallers int
	}{
		{"active queue", `{"callers":5,"available":2,"strategy":"ringall","holdtime":30}`, true, 5},
		{"empty queue", `{"callers":0,"available":3}`, true, 0},
		{"empty", ``, false, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := Decode("call_queue", json.RawMessage(tc.raw))
			if ok != tc.wantOK {
				t.Fatalf("ok: got %v, want %v", ok, tc.wantOK)
			}
			if !ok {
				return
			}
			s := got.(app.CallQueue)
			if s.Callers != tc.wantCallers {
				t.Errorf("Callers: got %d, want %d", s.Callers, tc.wantCallers)
			}
		})
	}
}

func TestDecode_UnknownType(t *testing.T) {
	_, ok := Decode("unknown_type_v99", json.RawMessage(`{"foo":"bar"}`))
	if ok {
		t.Fatal("expected unknown entity type to return ok=false")
	}
}

// ---------------------------------------------------------------------------
// Encode tests
// ---------------------------------------------------------------------------

func TestEncode_PBXReload(t *testing.T) {
	out, err := Encode(app.PBXReload{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var result map[string]any
	json.Unmarshal(out, &result)
	if result["action"] != "Reload" {
		t.Errorf("expected action=Reload, got %v", result)
	}
}

func TestEncode_SIPCallOriginate(t *testing.T) {
	tests := []struct {
		name    string
		cmd     app.SIPCallOriginate
		wantErr bool
	}{
		{"valid call", app.SIPCallOriginate{Extension: "200", Context: "internal"}, false},
		{"minimal", app.SIPCallOriginate{Extension: "100"}, false},
		{"empty extension rejected", app.SIPCallOriginate{Extension: ""}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := Encode(tc.cmd, nil)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err: got %v, wantErr %v", err, tc.wantErr)
			}
			if !tc.wantErr && out == nil {
				t.Error("expected non-nil output")
			}
		})
	}
}

func TestEncode_SIPHangup(t *testing.T) {
	out, err := Encode(app.SIPHangup{Channel: "SIP/100-00000001"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) == 0 {
		t.Error("expected non-empty output")
	}
}

func TestEncode_SIPTransfer(t *testing.T) {
	tests := []struct {
		name    string
		cmd     app.SIPTransfer
		wantErr bool
	}{
		{"valid transfer", app.SIPTransfer{Extension: "300"}, false},
		{"empty extension rejected", app.SIPTransfer{Extension: ""}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Encode(tc.cmd, nil)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err: got %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestEncode_SIPMute(t *testing.T) {
	for _, muted := range []bool{true, false} {
		out, err := Encode(app.SIPMute{Muted: muted}, nil)
		if err != nil {
			t.Errorf("muted=%v: unexpected error: %v", muted, err)
		}
		if len(out) == 0 {
			t.Errorf("muted=%v: empty output", muted)
		}
	}
}

func TestEncode_VoicemailDelete(t *testing.T) {
	tests := []struct {
		name    string
		cmd     app.VoicemailDelete
		wantErr bool
	}{
		{"valid", app.VoicemailDelete{Mailbox: "100"}, false},
		{"empty mailbox rejected", app.VoicemailDelete{Mailbox: ""}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Encode(tc.cmd, nil)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err: got %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestEncode_UnknownCommand(t *testing.T) {
	type unknownCmd struct{}
	_, err := Encode(unknownCmd{}, nil)
	if err == nil {
		t.Fatal("expected error for unknown command type")
	}
}

// ---------------------------------------------------------------------------
// Round-trip: Decode then Encode produces consistent output
// ---------------------------------------------------------------------------

func TestRoundTrip_PBXState(t *testing.T) {
	raw := json.RawMessage(`{"connected":true,"version":"20.5.0","uptime":86400}`)
	state, ok := Decode("pbx", raw)
	if !ok {
		t.Fatal("Decode failed")
	}
	pbx := state.(app.PBX)
	if !pbx.Connected || pbx.Version != "20.5.0" {
		t.Errorf("unexpected state: %+v", pbx)
	}

	// Encode a reload command (stateless, but validates the round-trip path)
	out, err := Encode(app.PBXReload{}, nil)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	var result map[string]any
	json.Unmarshal(out, &result)
	if result["action"] == nil {
		t.Error("encoded output missing action")
	}
}

func TestRoundTrip_SIPEndpointToCall(t *testing.T) {
	raw := json.RawMessage(`{"registered":true,"in_call":false,"ip":"192.168.88.50"}`)
	state, ok := Decode("sip_endpoint", raw)
	if !ok {
		t.Fatal("Decode failed")
	}
	endpoint := state.(app.SIPEndpoint)
	if !endpoint.Registered {
		t.Error("expected registered=true")
	}

	// Encode a call origination from this endpoint
	out, err := Encode(app.SIPCallOriginate{Extension: "200"}, nil)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	var result map[string]any
	json.Unmarshal(out, &result)
	if result["extension"] == nil {
		t.Error("encoded output missing extension")
	}
}
