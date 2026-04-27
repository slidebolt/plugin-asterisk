package app

import (
	"fmt"

	domain "github.com/slidebolt/sb-domain"
)

// primaryEntityID is the single child-entity slug for each device type.
// One device = one thing; its primary entity carries the device's state
// and exposes its commands. Additional entities (e.g. presence, call_state)
// can be attached to the same device later without schema changes.
const (
	pbxPrimary         = "status"
	sipTrunkPrimary    = "registration"
	sipEndpointPrimary = "registration"
	sipCallPrimary     = "state"
	voicemailPrimary   = "mailbox"
	callQueuePrimary   = "stats"
)

type seedRow struct {
	device domain.Device
	entity domain.Entity
}

func (a *App) seedDemo() error {
	rows := []seedRow{
		{
			device: domain.Device{ID: "server", Plugin: PluginID, Name: "Asterisk PBX"},
			entity: domain.Entity{
				ID: pbxPrimary, Plugin: PluginID, DeviceID: "server",
				Type: "pbx", Name: "Asterisk PBX",
				Commands: []string{"pbx_reload"},
				State:    PBX{Connected: false},
			},
		},
		{
			device: domain.Device{ID: "trunk_voipms", Plugin: PluginID, Name: "VoIP.ms Trunk"},
			entity: domain.Entity{
				ID: sipTrunkPrimary, Plugin: PluginID, DeviceID: "trunk_voipms",
				Type: "sip_trunk", Name: "VoIP.ms Trunk",
				State: SIPTrunk{Registered: false, Host: "chicago.voip.ms", Port: 5060},
			},
		},
		{
			device: domain.Device{ID: "ext_201", Plugin: PluginID, Name: "Gavin (Phone)"},
			entity: domain.Entity{
				ID: sipEndpointPrimary, Plugin: PluginID, DeviceID: "ext_201",
				Type: "sip_endpoint", Name: "Gavin (Phone)",
				Commands: []string{"sip_call", "sip_hangup"},
				State:    SIPEndpoint{Registered: false},
			},
		},
		{
			device: domain.Device{ID: "ext_202", Plugin: PluginID, Name: "Gavin (Laptop)"},
			entity: domain.Entity{
				ID: sipEndpointPrimary, Plugin: PluginID, DeviceID: "ext_202",
				Type: "sip_endpoint", Name: "Gavin (Laptop)",
				Commands: []string{"sip_call", "sip_hangup"},
				State:    SIPEndpoint{Registered: false},
			},
		},
		{
			device: domain.Device{ID: "ext_3000", Plugin: PluginID, Name: "Intercom"},
			entity: domain.Entity{
				ID: sipEndpointPrimary, Plugin: PluginID, DeviceID: "ext_3000",
				Type: "sip_endpoint", Name: "Intercom",
				Commands: []string{"sip_call", "sip_hangup"},
				State:    SIPEndpoint{Registered: false},
			},
		},
	}

	// Parents before children: save devices first so the UI and any
	// reconcilers that enumerate devices see them before the entities
	// arrive.
	for _, r := range rows {
		if err := a.store.Save(r.device); err != nil {
			return fmt.Errorf("save device %s: %w", r.device.Key(), err)
		}
	}
	for _, r := range rows {
		if err := a.store.Save(r.entity); err != nil {
			return fmt.Errorf("save entity %s: %w", r.entity.Key(), err)
		}
	}
	return nil
}
