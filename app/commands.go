package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"reflect"

	"github.com/slidebolt/plugin-asterisk/internal/hostapi"
	"github.com/slidebolt/plugin-asterisk/internal/mapping"
	domain "github.com/slidebolt/sb-domain"
	messenger "github.com/slidebolt/sb-messenger-sdk"
)

// lookupCommand resolves action names to concrete command types.
// It first checks plugin-specific commands, then falls back to the domain registry.
func lookupCommand(actionName string) (reflect.Type, bool) {
	return domain.LookupCommand(actionName)
}

// handleCommand only routes. Commands run in their own goroutine, so a slow call
// to the host cannot hold up the next command.
func (a *App) handleCommand(addr messenger.Address, cmd any) {
	if a.host == nil {
		log.Printf("plugin-asterisk: %s: ignored, host is not configured", addr.Key())
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(a.ctx, commandTimeout)
		defer cancel()
		if err := a.route(ctx, addr, cmd); err != nil {
			log.Printf("plugin-asterisk: %s %T: %v", addr.Key(), cmd, err)
		}
	}()
}

func (a *App) route(ctx context.Context, addr messenger.Address, cmd any) error {
	if addr.DeviceID == mapping.ServerDevice {
		if _, ok := cmd.(domain.PBXReload); ok {
			return a.host.Reload(ctx)
		}
		return unsupported(cmd, addr)
	}
	if call, ok := mapping.CallFromDevice(addr.DeviceID); ok {
		return a.routeCall(ctx, call, addr, cmd)
	}
	a.mu.Lock()
	endpoint, ok := a.endpoints[addr.DeviceID]
	a.mu.Unlock()
	if !ok {
		return fmt.Errorf("unknown device")
	}
	return a.routeEndpoint(ctx, endpoint, addr, cmd)
}

func (a *App) routeEndpoint(ctx context.Context, e hostapi.Endpoint, addr messenger.Address, cmd any) error {
	switch c := cmd.(type) {
	case domain.SIPDial:
		if c.Call == "" {
			return errors.New("sip_call needs a call name")
		}
		target := e.ID
		if c.Target != "" {
			target = c.Target
		}
		return a.host.AddParticipant(ctx, c.Call, hostapi.AddParticipantRequest{Target: target, RingSeconds: c.RingSeconds})
	case domain.SIPHangup:
		return a.host.HangupEndpoint(ctx, e.ID)
	case domain.SIPPlay:
		if e.Call == "" {
			return fmt.Errorf("%s is not in a call", e.ID)
		}
		return a.host.Play(ctx, e.Call, hostapi.PlayRequest{Sound: c.Sound, ThenHangup: c.ThenHangup})
	case domain.MediaStreamStart:
		return a.host.StartListen(ctx, e.ID)
	case domain.MediaStreamStop:
		return a.host.StopListen(ctx, e.ID)
	}
	return unsupported(cmd, addr)
}

func (a *App) routeCall(ctx context.Context, call string, addr messenger.Address, cmd any) error {
	if participant, ok := mapping.ParticipantFromEntity(addr.EntityID); ok {
		return a.routeParticipant(ctx, call, participant, addr, cmd)
	}
	switch c := cmd.(type) {
	case domain.SIPDial:
		return a.host.AddParticipant(ctx, call, hostapi.AddParticipantRequest{Target: c.Target, RingSeconds: c.RingSeconds})
	case domain.SIPHangup:
		return a.host.EndCall(ctx, call)
	case domain.SIPPlay:
		return a.host.Play(ctx, call, hostapi.PlayRequest{Sound: c.Sound, ThenHangup: c.ThenHangup})
	case domain.SIPTranscribeStart:
		return a.host.StartTranscription(ctx, call)
	case domain.SIPTranscribeStop:
		return a.host.StopTranscription(ctx, call)
	case domain.MediaStreamStart:
		return a.host.StartCallStream(ctx, call)
	case domain.MediaStreamStop:
		return a.host.StopCallStream(ctx, call)
	}
	return unsupported(cmd, addr)
}

func (a *App) routeParticipant(ctx context.Context, call, id string, addr messenger.Address, cmd any) error {
	action := ""
	var params map[string]any
	switch c := cmd.(type) {
	case domain.SIPMute:
		action = "mute"
	case domain.SIPUnmute:
		action = "unmute"
	case domain.SIPHold:
		action = "hold"
	case domain.SIPUnhold:
		action = "unhold"
	case domain.SIPDTMF:
		action = "dtmf"
		params = map[string]any{"digits": c.Digits}
	case domain.SIPRemove, domain.SIPHangup:
		return a.host.RemoveParticipant(ctx, call, id)
	default:
		return unsupported(cmd, addr)
	}
	return a.host.ParticipantCommand(ctx, call, id, hostapi.CommandRequest{Action: action, Parameters: params})
}

func unsupported(cmd any, addr messenger.Address) error {
	return fmt.Errorf("%T is not supported on %s", cmd, addr.EntityID)
}
