package mapstate

import "github.com/Travis-Britz/ps2"

type Zone struct {
	ps2.Zone
	Map
}
type Map struct {
	Regions []Region
	Lattice Lattice
}
type Region struct {
	ps2.MapRegion
	Hexes []ps2.MapHex
}
type Lattice struct {
	Facilities map[ps2.FacilityID]Region
	Links      []Link
}

type Link struct {
	A ps2.FacilityID
	B ps2.FacilityID
}
