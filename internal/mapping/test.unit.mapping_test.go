package mapping

import (
	"slices"
	"testing"

	"github.com/slidebolt/plugin-asterisk/internal/hostapi"
	domain "github.com/slidebolt/sb-domain"
)

func find(t *testing.T, item Item, id string) domain.Entity {
	t.Helper()
	for _, e := range item.Entities {
		if e.ID == id {
			return e
		}
	}
	t.Fatalf("no entity %q in %+v", id, item.Entities)
	return domain.Entity{}
}

func TestDeviceIDs(t *testing.T) {
	cases := map[string]hostapi.Endpoint{
		"ext_3000":        {ID: "3000", Kind: "extension"},
		"ext_float_10000": {ID: "float-10000", Kind: "extension"},
		"trunk_trunk1":    {ID: "trunk1", Kind: "trunk"},
	}
	for want, e := range cases {
		if got := EndpointDevice(e); got != want {
			t.Errorf("%s: got %s", want, got)
		}
	}
	if id, ok := CallFromDevice(CallDevice("front-door")); !ok || id != "front-door" {
		t.Errorf("call id = %q %v", id, ok)
	}
	if _, ok := CallFromDevice("ext_201"); ok {
		t.Error("ext_201 is not a call")
	}
	if id, ok := ParticipantFromEntity("p_201"); !ok || id != "201" {
		t.Errorf("participant id = %q %v", id, ok)
	}
}

func TestEndpoint_Extension(t *testing.T) {
	item := Endpoint(hostapi.Endpoint{ID: "201", Name: "Kitchen Phone", Kind: "extension", Status: "online", Call: "c"})
	if item.Device.ID != "ext_201" || item.Device.Name != "Kitchen Phone" || len(item.Device.Entities) != 3 {
		t.Fatalf("device = %+v", item.Device)
	}
	reg := find(t, item, EntityRegistration)
	if !slices.Contains(reg.Commands, "sip_call") || !slices.Contains(reg.Commands, "sip_hangup") {
		t.Fatalf("commands = %v", reg.Commands)
	}
	if s := find(t, item, "availability").State.(domain.BinarySensor); !s.On || s.DeviceClass != "connectivity" {
		t.Fatalf("availability = %+v", s)
	}
	if v := find(t, item, "call-state").State.(domain.Sensor).Value; v != "in_call" {
		t.Fatalf("call-state = %v", v)
	}
}

func TestEndpoint_AudioOnlyWhenPolicyAllowsIt(t *testing.T) {
	item := Endpoint(hostapi.Endpoint{ID: "3000", Kind: "extension", Stream: &hostapi.Stream{Status: "live", URL: "u", Viewers: 2}})
	a := find(t, item, EntityAudio)
	s := a.State.(domain.MediaStream)
	if a.Type != "media_stream" || s.Status != "live" || s.URL != "u" || s.Viewers != 2 || s.Kind != "audio" {
		t.Fatalf("audio = %+v", a)
	}
	for _, e := range Endpoint(hostapi.Endpoint{ID: "201", Kind: "extension"}).Entities {
		if e.ID == EntityAudio {
			t.Fatal("no stream means no audio entity")
		}
	}
}

func TestEndpoint_TrunkHasNoCallCommands(t *testing.T) {
	item := Endpoint(hostapi.Endpoint{ID: "trunk1", Kind: "trunk", Status: "online"})
	reg := find(t, item, EntityRegistration)
	if reg.Type != "sip_trunk" || len(reg.Commands) != 0 {
		t.Fatalf("trunk = %+v", reg)
	}
}

func TestCall(t *testing.T) {
	item := Call(hostapi.Call{
		ID: "x", Status: "ended", Reason: "r",
		CreatedAt: "2026-01-01T00:00:00Z", EndedAt: "2026-01-01T00:00:10Z",
		Participants: []hostapi.Participant{{ID: "201", Target: "201", Status: "left", Muted: true, LastDTMF: "#"}},
	})
	c := find(t, item, EntityCall).State.(domain.SIPCall)
	if c.Status != "ended" || c.Reason != "r" || c.Duration != 10 {
		t.Fatalf("call = %+v", c)
	}
	p := find(t, item, "p_201")
	s := p.State.(domain.SIPParticipant)
	if s.Status != "left" || !s.Muted || s.LastDTMF != "#" || !slices.Contains(p.Commands, "sip_mute") {
		t.Fatalf("participant = %+v", p)
	}
	if a := find(t, item, EntityAudio).State.(domain.MediaStream); a.Status != "idle" {
		t.Fatalf("audio = %+v", a)
	}
	if tr := find(t, item, EntityTranscript).State.(domain.Transcript); tr.Status != "idle" {
		t.Fatalf("transcript = %+v", tr)
	}
}

func TestCall_Transcript(t *testing.T) {
	item := Call(hostapi.Call{ID: "x", Status: "active", Transcript: &hostapi.Transcript{Status: "listening", Seq: 2, Text: "turn the lights on"}})
	e := find(t, item, EntityTranscript)
	s := e.State.(domain.Transcript)
	if e.Type != "transcript" || s.Seq != 2 || s.Text != "turn the lights on" || s.Status != "listening" {
		t.Fatalf("transcript = %+v", e)
	}
	if !slices.Contains(find(t, item, EntityCall).Commands, "sip_transcribe_start") {
		t.Fatal("call has no transcribe command")
	}
}

func TestEndpoint_Service(t *testing.T) {
	item := Endpoint(hostapi.Endpoint{ID: "4000", Name: "Agent", Kind: "service", Status: "online"})
	if item.Device.ID != "ext_4000" || len(item.Entities) != 2 {
		t.Fatalf("service = %+v", item)
	}
	reg := find(t, item, EntityRegistration)
	if s := reg.State.(domain.SIPEndpoint); !s.Registered || s.Role != "service" || len(reg.Commands) != 0 {
		t.Fatalf("registration = %+v", reg)
	}
}

func TestEndpoint_ServiceWithText(t *testing.T) {
	item := Endpoint(hostapi.Endpoint{ID: "4000", Name: "Agent", Kind: "service", Status: "online",
		Inbox: &hostapi.Inbox{Seq: 2, From: "201", Text: "hi", At: "2026-11-07T00:00:00Z"}})
	if len(item.Entities) != 3 {
		t.Fatalf("service with text = %+v", item)
	}
	e := find(t, item, EntityMessages)
	s := e.State.(domain.SIPMessages)
	if e.Type != "sip_messages" || s.Seq != 2 || s.From != "201" || s.Text != "hi" || s.At == "" {
		t.Fatalf("messages = %+v", e)
	}
	if !slices.Equal(e.Commands, []string{"sip_message_send"}) {
		t.Fatalf("commands = %v", e.Commands)
	}
}

func TestCall_Speech(t *testing.T) {
	idle := find(t, Call(hostapi.Call{ID: "x", Status: "active"}), EntitySpeech).State.(domain.Speech)
	if idle.Status != "idle" {
		t.Fatalf("default speech = %+v", idle)
	}
	item := Call(hostapi.Call{ID: "x", Status: "active", Speech: &hostapi.Speech{Status: "speaking", Seq: 3, Text: "hello"}})
	e := find(t, item, EntitySpeech)
	if s := e.State.(domain.Speech); e.Type != "speech" || s.Status != "speaking" || s.Seq != 3 || s.Text != "hello" {
		t.Fatalf("speech = %+v", e)
	}
	if !slices.Contains(find(t, item, EntityCall).Commands, "sip_say") {
		t.Fatal("call has no sip_say command")
	}
}
