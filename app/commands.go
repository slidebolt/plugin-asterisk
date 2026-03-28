package app

import (
	"log"
	"reflect"

	domain "github.com/slidebolt/sb-domain"
	messenger "github.com/slidebolt/sb-messenger-sdk"
)

// lookupCommand resolves action names to concrete command types.
// It first checks plugin-specific commands, then falls back to the domain registry.
func lookupCommand(actionName string) (reflect.Type, bool) {
	return domain.LookupCommand(actionName)
}

func (a *App) handleCommand(addr messenger.Address, cmd any) {
	switch c := cmd.(type) {
	case PBXReload:
		log.Printf("plugin-asterisk: pbx %s reload", addr.Key())
	case SIPCallOriginate:
		log.Printf("plugin-asterisk: endpoint %s call extension=%s", addr.Key(), c.Extension)
	case SIPHangup:
		log.Printf("plugin-asterisk: call %s hangup channel=%s", addr.Key(), c.Channel)
	case SIPTransfer:
		log.Printf("plugin-asterisk: call %s transfer extension=%s", addr.Key(), c.Extension)
	case SIPMute:
		log.Printf("plugin-asterisk: call %s mute=%v", addr.Key(), c.Muted)
	case VoicemailDelete:
		log.Printf("plugin-asterisk: voicemail %s delete mailbox=%s", addr.Key(), c.Mailbox)
	default:
		log.Printf("plugin-asterisk: unknown command %T for %s", cmd, addr.Key())
	}
}
