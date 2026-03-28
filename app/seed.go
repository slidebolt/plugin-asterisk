package app

import (
	"fmt"

	domain "github.com/slidebolt/sb-domain"
)

func (a *App) seedDemo() error {
	entities := []domain.Entity{
		{
			ID: "server", Plugin: PluginID, DeviceID: "asterisk",
			Type: "pbx", Name: "Asterisk PBX",
			Commands: []string{"pbx_reload"},
			State:    PBX{Connected: false},
		},
		{
			ID: "trunk_voipms", Plugin: PluginID, DeviceID: "asterisk",
			Type: "sip_trunk", Name: "VoIP.ms Trunk",
			State: SIPTrunk{Registered: false, Host: "chicago.voip.ms", Port: 5060},
		},
		// Real extensions from pjsip.conf
		{
			ID: "ext_201", Plugin: PluginID, DeviceID: "asterisk",
			Type: "sip_endpoint", Name: "Gavin (Phone)",
			Commands: []string{"sip_call", "sip_hangup"},
			State:    SIPEndpoint{Registered: false},
		},
		{
			ID: "ext_202", Plugin: PluginID, DeviceID: "asterisk",
			Type: "sip_endpoint", Name: "Gavin (Laptop)",
			Commands: []string{"sip_call", "sip_hangup"},
			State:    SIPEndpoint{Registered: false},
		},
		{
			ID: "ext_3000", Plugin: PluginID, DeviceID: "asterisk",
			Type: "sip_endpoint", Name: "Intercom",
			Commands: []string{"sip_call", "sip_hangup"},
			State:    SIPEndpoint{Registered: false},
		},
	}
	for _, entity := range entities {
		if err := a.store.Save(entity); err != nil {
			return fmt.Errorf("save %s: %w", entity.ID, err)
		}
	}
	return nil
}
