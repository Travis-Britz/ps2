package mapstate

import "github.com/Travis-Britz/ps2"

// ----------------------------------------------------------------------

type Hex struct {
	X    int
	Y    int
	Type ps2.MapHexType
}

type Region struct {
	Hexes    []Hex
	RegionID ps2.RegionID
	Faction  ps2.FactionID
	Facility
}

type Facility struct {
	ID      ps2.FacilityID
	Name    string
	Type    ps2.FacilityTypeID
	X, Y, Z float64
}

type Link struct {
	A, B ps2.FacilityID
}

type Map struct {
	Regions []Region
	Links   []Link
}

func (m Map) Facilities() (facilities []Facility) {
	for _, r := range m.Regions {
		if r.Facility != (Facility{}) {
			facilities = append(facilities, r.Facility)
		}
	}
	return
}

func (m Map) Neighbors(f Facility) (facilities []Facility) {
	return facilities
}
