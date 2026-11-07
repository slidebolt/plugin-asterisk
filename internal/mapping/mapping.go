// Package mapping turns host objects into SlideBolt devices and entities. It
// holds no behavior and no knowledge of any particular device.
package mapping

import (
	"regexp"
	"strings"
	"time"

	"github.com/slidebolt/plugin-asterisk/internal/hostapi"
	domain "github.com/slidebolt/sb-domain"
)

// PluginID is the plugin these entities belong to.
const PluginID = "plugin-asterisk"

const (
	ServerDevice = "server"
	callPrefix   = "call_"
	extPrefix    = "ext_"
	trunkPrefix  = "trunk_"

	participantPrefix  = "p_"
	EntityRegistration = "registration"
	EntityAudio        = "audio"
	EntityCall         = "call"
	EntityTranscript   = "transcript"
	EntitySpeech       = "speech"
)

var deviceUnsafe = regexp.MustCompile(`[^A-Za-z0-9_]`)

// EndpointDevice returns the device ID for a host endpoint.
func EndpointDevice(e hostapi.Endpoint) string {
	prefix := extPrefix
	if e.Kind == "trunk" {
		prefix = trunkPrefix
	}
	return prefix + deviceUnsafe.ReplaceAllString(e.ID, "_")
}

// CallDevice returns the device ID for a call.
func CallDevice(id string) string {
	return callPrefix + id
}

// CallFromDevice returns the call ID of a call device.
func CallFromDevice(device string) (string, bool) {
	return strings.CutPrefix(device, callPrefix)
}

// ParticipantFromEntity returns the participant ID of a participant entity.
func ParticipantFromEntity(entity string) (string, bool) {
	return strings.CutPrefix(entity, participantPrefix)
}

func newItem(device domain.Device, entities ...domain.Entity) Item {
	device.Entities = entities
	return Item{Device: device, Entities: entities}
}

// Item is one device with its entities.
type Item struct {
	Device   domain.Device
	Entities []domain.Entity
}

func entity(device, id, typ, name string, commands []string, state any) domain.Entity {
	return domain.Entity{
		ID: id, Plugin: PluginID, DeviceID: device, Type: typ, Name: name,
		Commands: commands, State: state,
	}
}

// Server maps the Asterisk server.
func Server(s hostapi.Server) Item {
	const d = ServerDevice
	return newItem(domain.Device{ID: d, Plugin: PluginID, Name: "Asterisk PBX"},
		entity(d, "connection", "binary_sensor", "Connection", nil, domain.BinarySensor{On: s.Connected, DeviceClass: "connectivity"}),
		entity(d, "version", "sensor", "Version", nil, domain.Sensor{Value: s.Version}),
		entity(d, "active-calls", "sensor", "Active Calls", nil, domain.Sensor{Value: s.ActiveCalls}),
		entity(d, "status", "pbx", "Asterisk PBX", []string{"pbx_reload"}, domain.PBX{
			Connected: s.Connected, Version: s.Version, ActiveCalls: s.ActiveCalls, Error: s.Error,
		}),
	)
}

// Endpoint maps a phone, intercom or trunk.
func Endpoint(e hostapi.Endpoint) Item {
	d := EndpointDevice(e)
	online := e.Status == "online"
	availability := entity(d, "availability", "binary_sensor", "Availability", nil, domain.BinarySensor{On: online, DeviceClass: "connectivity"})
	device := domain.Device{ID: d, Plugin: PluginID, Name: e.Name}

	if e.Kind == "trunk" {
		return newItem(device, availability,
			entity(d, EntityRegistration, "sip_trunk", e.Name, nil, domain.SIPTrunk{Registered: online}))
	}
	if e.Kind == "service" {
		// A number SlideBolt answers. It cannot be dialed out to or hung up.
		return newItem(device, availability,
			entity(d, EntityRegistration, "sip_endpoint", e.Name, nil, domain.SIPEndpoint{
				Registered: online, Status: e.Status, Role: "service",
			}))
	}

	callState := "idle"
	if e.Call != "" {
		callState = "in_call"
	}
	entities := []domain.Entity{
		availability,
		entity(d, "call-state", "sensor", "Call State", nil, domain.Sensor{Value: callState}),
		entity(d, EntityRegistration, "sip_endpoint", e.Name, []string{"sip_call", "sip_hangup", "sip_play"}, domain.SIPEndpoint{
			Registered: online, InCall: e.Call != "", Status: e.Status, Role: e.Role, Call: e.Call,
		}),
	}
	if e.Stream != nil {
		entities = append(entities, audio(d, e.Name+" Audio", e.Stream))
	}
	return newItem(device, entities...)
}

func audio(device, name string, s *hostapi.Stream) domain.Entity {
	return entity(device, EntityAudio, "media_stream", name, []string{"media_stream_start", "media_stream_stop"}, domain.MediaStream{
		Status: s.Status, Kind: "audio", URL: s.URL, Format: "mpegts", Viewers: s.Viewers, Error: s.Error,
	})
}

// Call maps a call and its participants.
func Call(c hostapi.Call) Item {
	d := CallDevice(c.ID)
	state := "up"
	if c.Status == "ended" {
		state = "ended"
	}
	entities := []domain.Entity{
		entity(d, EntityCall, "sip_call", "Call "+c.ID, []string{"sip_call", "sip_hangup", "sip_play", "sip_transcribe_start", "sip_transcribe_stop", "sip_say"}, domain.SIPCall{
			State: state, Status: c.Status, Reason: c.Reason, Duration: duration(c),
		}),
	}
	stream := c.Stream
	if stream == nil {
		stream = &hostapi.Stream{Status: "idle"}
	}
	entities = append(entities, audio(d, "Call "+c.ID+" Audio", stream))
	transcript := c.Transcript
	if transcript == nil {
		transcript = &hostapi.Transcript{Status: "idle"}
	}
	entities = append(entities, entity(d, EntityTranscript, "transcript", "Call "+c.ID+" Transcript", []string{"sip_transcribe_start", "sip_transcribe_stop"}, domain.Transcript{
		Status: transcript.Status, Seq: transcript.Seq, Text: transcript.Text, Error: transcript.Error,
	}))
	speech := c.Speech
	if speech == nil {
		speech = &hostapi.Speech{Status: "idle"}
	}
	entities = append(entities, entity(d, EntitySpeech, "speech", "Call "+c.ID+" Speech", []string{"sip_say"}, domain.Speech{
		Status: speech.Status, Seq: speech.Seq, Text: speech.Text, Error: speech.Error,
	}))
	for _, p := range c.Participants {
		entities = append(entities, entity(d, participantPrefix+p.ID, "sip_participant", p.ID,
			[]string{"sip_mute", "sip_unmute", "sip_hold", "sip_unhold", "sip_dtmf", "sip_remove", "sip_hangup"},
			domain.SIPParticipant{
				Status: p.Status, Target: p.Target, Muted: p.Muted, Held: p.Held, LastDTMF: p.LastDTMF, Reason: p.Reason,
			}))
	}
	return newItem(domain.Device{ID: d, Plugin: PluginID, Name: "Call " + c.ID}, entities...)
}

func duration(c hostapi.Call) int {
	start, err := time.Parse(time.RFC3339Nano, c.CreatedAt)
	if err != nil || c.EndedAt == "" {
		return 0
	}
	end, err := time.Parse(time.RFC3339Nano, c.EndedAt)
	if err != nil {
		return 0
	}
	return int(end.Sub(start).Seconds())
}
