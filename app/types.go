package app

import domain "github.com/slidebolt/sb-domain"

// The SIP entity types live in sb-domain, so sb-script and sb-api understand them.
type (
	PBX         = domain.PBX
	SIPTrunk    = domain.SIPTrunk
	SIPEndpoint = domain.SIPEndpoint
	SIPCall     = domain.SIPCall
)
