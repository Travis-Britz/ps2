package statemanager

import "github.com/Travis-Britz/ps2"

const (
	none = ps2.FactionUnknown
	vs   = ps2.VS
	nc   = ps2.NC
	tr   = ps2.TR
	nso  = ps2.NSO
)

// isPermanent is a list of playable continents that are always listed and available.
var isPermanent = map[ps2.ZoneID]bool{
	ps2.Indar:   true,
	ps2.Hossin:  true,
	ps2.Amerish: true,
	ps2.Esamir:  true,
	ps2.Oshur:   true,
}

// isPlayable is a list of zones that are playable, including special zones like those for outfit wars.
// It does not include non-combat zones like sanctuary or VR training.
var isPlayable = map[ps2.ZoneID]bool{
	ps2.Indar:      true,
	ps2.Hossin:     true,
	ps2.Amerish:    true,
	ps2.Esamir:     true,
	ps2.Oshur:      true,
	ps2.Koltyr:     true,
	ps2.Desolation: true,
	ps2.Nexus:      true,
}
