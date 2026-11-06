package hostapi

import "encoding/json"

type Server struct {
	Connected   bool   `json:"connected"`
	Version     string `json:"version,omitempty"`
	ActiveCalls int    `json:"activeCalls"`
	Error       string `json:"error,omitempty"`
}

type Features struct {
	AutoAnswer bool   `json:"autoAnswer"`
	Listen     string `json:"listen"`
}

type Stream struct {
	Status  string `json:"status"`
	URL     string `json:"url,omitempty"`
	Viewers int    `json:"viewers"`
	Error   string `json:"error,omitempty"`
}

type Endpoint struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Kind     string   `json:"kind"`
	Role     string   `json:"role,omitempty"`
	Status   string   `json:"status"`
	Call     string   `json:"call,omitempty"`
	Features Features `json:"features"`
	Stream   *Stream  `json:"stream,omitempty"`
}

type Participant struct {
	ID       string `json:"id"`
	Target   string `json:"target"`
	Status   string `json:"status"`
	Muted    bool   `json:"muted"`
	Held     bool   `json:"held"`
	LastDTMF string `json:"lastDtmf,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

type Transcript struct {
	Status string `json:"status"`
	Seq    int    `json:"seq"`
	Text   string `json:"text,omitempty"`
	Error  string `json:"error,omitempty"`
}

type Call struct {
	ID           string        `json:"id"`
	Status       string        `json:"status"`
	Reason       string        `json:"reason,omitempty"`
	CreatedAt    string        `json:"createdAt"`
	EndedAt      string        `json:"endedAt,omitempty"`
	Participants []Participant `json:"participants"`
	Stream       *Stream       `json:"stream,omitempty"`
	Transcript   *Transcript   `json:"transcript,omitempty"`
}

type Snapshot struct {
	APIVersion int        `json:"apiVersion"`
	StreamID   string     `json:"streamId"`
	Sequence   int64      `json:"sequence"`
	Server     Server     `json:"server"`
	Endpoints  []Endpoint `json:"endpoints"`
	Calls      []Call     `json:"calls"`
}

// Event types.
const (
	EventServer   = "server.updated"
	EventEndpoint = "endpoint.updated"
	EventCall     = "call.updated"
	EventRemoved  = "call.removed"
)

type Event struct {
	Version   int             `json:"version"`
	ID        int64           `json:"id"`
	StreamID  string          `json:"streamId"`
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Data      json.RawMessage `json:"data"`
}

type AddParticipantRequest struct {
	Target      string `json:"target"`
	RingSeconds int    `json:"ringSeconds,omitempty"`
}

type CommandRequest struct {
	Action     string         `json:"action"`
	Parameters map[string]any `json:"parameters,omitempty"`
}

type PlayRequest struct {
	Sound      string `json:"sound"`
	ThenHangup bool   `json:"thenHangup,omitempty"`
}
