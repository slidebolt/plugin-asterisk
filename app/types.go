package app

import domain "github.com/slidebolt/sb-domain"

// PBX represents the state of the Asterisk server itself.
type PBX struct {
	Connected bool   `json:"connected"`
	Version   string `json:"version,omitempty"`
	Uptime    int64  `json:"uptime,omitempty"`
}

// SIPTrunk represents the registration state of a SIP trunk (e.g. voip.ms).
type SIPTrunk struct {
	Registered bool   `json:"registered"`
	Host       string `json:"host,omitempty"`
	Port       int    `json:"port,omitempty"`
	Latency    int    `json:"latency,omitempty"`
}

// SIPEndpoint represents a SIP extension/phone.
type SIPEndpoint struct {
	Registered bool   `json:"registered"`
	InCall     bool   `json:"in_call"`
	IP         string `json:"ip,omitempty"`
	Agent      string `json:"agent,omitempty"`
}

// SIPCall represents an active call.
type SIPCall struct {
	State     string `json:"state"`
	Caller    string `json:"caller"`
	Callee    string `json:"callee"`
	Duration  int    `json:"duration"`
	Direction string `json:"direction,omitempty"`
}

// Voicemail represents a voicemail box.
type Voicemail struct {
	NewMessages int    `json:"new_messages"`
	OldMessages int    `json:"old_messages"`
	Mailbox     string `json:"mailbox,omitempty"`
}

// CallQueue represents an Asterisk call queue.
type CallQueue struct {
	Callers   int    `json:"callers"`
	Available int    `json:"available"`
	Strategy  string `json:"strategy,omitempty"`
	Holdtime  int    `json:"holdtime,omitempty"`
}

// --- Commands ---

// PBXReload triggers an Asterisk configuration reload.
type PBXReload struct{}

func (PBXReload) ActionName() string { return "pbx_reload" }

// SIPCallOriginate initiates an outbound call.
type SIPCallOriginate struct {
	Extension string `json:"extension"`
	Context   string `json:"context,omitempty"`
	CallerID  string `json:"caller_id,omitempty"`
}

func (SIPCallOriginate) ActionName() string { return "sip_call" }

// SIPHangup hangs up an active call or channel.
type SIPHangup struct {
	Channel string `json:"channel,omitempty"`
}

func (SIPHangup) ActionName() string { return "sip_hangup" }

// SIPTransfer transfers an active call to another extension.
type SIPTransfer struct {
	Extension string `json:"extension"`
	Channel   string `json:"channel,omitempty"`
}

func (SIPTransfer) ActionName() string { return "sip_transfer" }

// SIPMute mutes/unmutes a channel.
type SIPMute struct {
	Muted bool `json:"muted"`
}

func (SIPMute) ActionName() string { return "sip_mute" }

// VoicemailDelete deletes voicemail messages.
type VoicemailDelete struct {
	Mailbox string `json:"mailbox"`
}

func (VoicemailDelete) ActionName() string { return "voicemail_delete" }

func init() {
	domain.Register("pbx", PBX{})
	domain.Register("sip_trunk", SIPTrunk{})
	domain.Register("sip_endpoint", SIPEndpoint{})
	domain.Register("sip_call", SIPCall{})
	domain.Register("voicemail", Voicemail{})
	domain.Register("call_queue", CallQueue{})

	domain.RegisterCommand("pbx_reload", PBXReload{})
	domain.RegisterCommand("sip_call", SIPCallOriginate{})
	domain.RegisterCommand("sip_hangup", SIPHangup{})
	domain.RegisterCommand("sip_transfer", SIPTransfer{})
	domain.RegisterCommand("sip_mute", SIPMute{})
	domain.RegisterCommand("voicemail_delete", VoicemailDelete{})
}
