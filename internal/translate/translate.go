package translate

// translate.go — Decode/Encode translation layer for Asterisk/SIP entities.
//
// Decode: raw ARI/AMI event data → canonical domain state (lenient)
// Encode: canonical domain command → raw ARI/AMI action payload (strict)

import (
	"encoding/json"
	"fmt"

	"github.com/slidebolt/plugin-asterisk/app"
)

// Decode converts a raw protocol payload into a canonical domain state.
// Returns (state, true) on success, (nil, false) to silently skip.
func Decode(entityType string, raw json.RawMessage) (any, bool) {
	switch entityType {
	case "pbx":
		return decodePBX(raw)
	case "sip_trunk":
		return decodeSIPTrunk(raw)
	case "sip_endpoint":
		return decodeSIPEndpoint(raw)
	case "sip_call":
		return decodeSIPCall(raw)
	case "voicemail":
		return decodeVoicemail(raw)
	case "call_queue":
		return decodeCallQueue(raw)
	default:
		return nil, false
	}
}

// Encode converts a SlideBolt domain command into a raw protocol payload.
// internal holds the raw discovery payload for device-specific metadata.
func Encode(cmd any, internal json.RawMessage) (json.RawMessage, error) {
	switch c := cmd.(type) {
	case app.PBXReload:
		return encodePBXReload(c, internal)
	case app.SIPCallOriginate:
		return encodeSIPCallOriginate(c, internal)
	case app.SIPHangup:
		return encodeSIPHangup(c, internal)
	case app.SIPTransfer:
		return encodeSIPTransfer(c, internal)
	case app.SIPMute:
		return encodeSIPMute(c, internal)
	case app.VoicemailDelete:
		return encodeVoicemailDelete(c, internal)
	default:
		return nil, fmt.Errorf("translate: unsupported command type %T", cmd)
	}
}

// ---------------------------------------------------------------------------
// Decode: per-type (raw protocol → domain state)
//
// Identity decode for now — replace with ARI/AMI parsing.
// ---------------------------------------------------------------------------

func decodePBX(raw json.RawMessage) (any, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var s app.PBX
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, false
	}
	return s, true
}

func decodeSIPTrunk(raw json.RawMessage) (any, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var s app.SIPTrunk
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, false
	}
	return s, true
}

func decodeSIPEndpoint(raw json.RawMessage) (any, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var s app.SIPEndpoint
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, false
	}
	return s, true
}

func decodeSIPCall(raw json.RawMessage) (any, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var s app.SIPCall
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, false
	}
	return s, true
}

func decodeVoicemail(raw json.RawMessage) (any, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var s app.Voicemail
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, false
	}
	return s, true
}

func decodeCallQueue(raw json.RawMessage) (any, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	var s app.CallQueue
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, false
	}
	return s, true
}

// ---------------------------------------------------------------------------
// Encode: per-command (domain command → raw protocol payload)
//
// Identity encode for now — replace with ARI/AMI action serialization.
// ---------------------------------------------------------------------------

func encodePBXReload(_ app.PBXReload, _ json.RawMessage) (json.RawMessage, error) {
	return json.Marshal(map[string]any{"action": "Reload"})
}

func encodeSIPCallOriginate(c app.SIPCallOriginate, _ json.RawMessage) (json.RawMessage, error) {
	if c.Extension == "" {
		return nil, fmt.Errorf("translate: extension must not be empty")
	}
	return json.Marshal(c)
}

func encodeSIPHangup(c app.SIPHangup, _ json.RawMessage) (json.RawMessage, error) {
	return json.Marshal(c)
}

func encodeSIPTransfer(c app.SIPTransfer, _ json.RawMessage) (json.RawMessage, error) {
	if c.Extension == "" {
		return nil, fmt.Errorf("translate: transfer extension must not be empty")
	}
	return json.Marshal(c)
}

func encodeSIPMute(c app.SIPMute, _ json.RawMessage) (json.RawMessage, error) {
	return json.Marshal(c)
}

func encodeVoicemailDelete(c app.VoicemailDelete, _ json.RawMessage) (json.RawMessage, error) {
	if c.Mailbox == "" {
		return nil, fmt.Errorf("translate: mailbox must not be empty")
	}
	return json.Marshal(c)
}
