// Package psmap deals with PlanetSide 2 maps.
package psmap

import (
	"fmt"

	"github.com/Travis-Britz/ps2"
	"github.com/Travis-Britz/structures/stack"
)

const (
	Locked Status = iota
	Unlocked
	Unstable
)

type Region interface {
	Region() ps2.RegionID
	Facility() ps2.FacilityID
	FacilityType() ps2.FacilityTypeID
}

type Link interface {
	A() ps2.FacilityID
	B() ps2.FacilityID
}

// State describes the territory ownership of a continent.
type State map[ps2.RegionID]ps2.FactionID

type Map interface {
	Regions() []Region
	Links() []Link
}

type facilityRegion struct {
	RegionID   ps2.RegionID
	FacilityID ps2.FacilityID
	Owner      ps2.FactionID
	Cutoff     bool
	Links      []*facilityRegion
}

// Summarize takes calculates territory ownership percentages,
// factoring in cutoff territory and disabled regions.
//
// result represents territory ownership of a zone.
// The faction key will correspond to warpgate ownership;
// on Nexus (outfit wars) there may only be two teams.
func Summarize(mapData Map, regions State) (summary Summary, err error) {
	summary = Summary{
		Territory:     map[ps2.FactionID]float32{},
		FacilityCount: map[ps2.FactionID]int{},
		CutoffCount:   map[ps2.FactionID]int{},
	}
	lattice := make(map[ps2.FacilityID]*facilityRegion) // lattice is the graph of facility connections
	regionIdx := make(map[ps2.RegionID]ps2.FacilityID)  // regionIdx maps RegionIDs to FacilityIDs
	warpgates := make([]*facilityRegion, 0, 3)

	// start by building a graph of connected facilities
	for _, reg := range mapData.Regions() {
		// the census /map endpoint gives facility ownership by region id,
		// but not every region has a facility.
		// regions without a facility will typically be owned by faction 0 (None)
		if reg.Facility() == 0 {
			continue
		}

		r := &facilityRegion{
			RegionID:   reg.Region(),
			FacilityID: reg.Facility(),
			Owner:      regions[reg.Region()],
			// Cutoff:     true, // every region starts as cut off, then as we visit each region in the graph we mark it as available
		}
		lattice[reg.Facility()] = r
		summary.CutoffCount[r.Owner]++

		regionIdx[reg.Region()] = reg.Facility()
		if reg.FacilityType() == ps2.Warpgate {
			warpgates = append(warpgates, r)
		}
	}
	// go through the list of lattice connections and build up the list of neighbor facilities for each facility
	for _, link := range mapData.Links() {

		// check for referenced facilities that don't exist.
		// we can't always trust census to be consistent,
		// and we don't need any nil pointers to dereference.
		fA, ok := lattice[link.A()]
		if !ok {
			return summary, fmt.Errorf("a facility link referenced a facility missing from the supplied map data; link: %v, facility: %v", link, link.A())
		}
		fB, ok := lattice[link.B()]
		if !ok {
			return summary, fmt.Errorf("a facility link referenced a facility missing from the supplied map data; link: %v, facility: %v", link, link.B())
		}

		// the lattice links may or may not contain a link both ways, but the links are bidirectional.
		// it doesn't matter if we have duplicate neighbors added;
		// our graph traversal will skip visited facilities.
		fA.Links = append(fA.Links, fB)
		fB.Links = append(fB.Links, fA)

	}

	frontier := &stack.Stack[*facilityRegion]{}
	visited := map[ps2.FacilityID]bool{}

	for _, start := range warpgates {
		// start.Cutoff = false
		visited[start.FacilityID] = true
		summary.CutoffCount[start.Owner]--
		for current, more := start, true; more; current, more = frontier.Pop() {
			for _, next := range current.Links {
				if visited[next.FacilityID] {
					continue
				}
				if next.Owner == current.Owner {
					frontier.Push(next)
					visited[next.FacilityID] = true
					// next.Cutoff = false
					summary.FacilityCount[next.Owner]++
					summary.CutoffCount[next.Owner]--
				}
			}
		}
	}

	factionCount := make(map[ps2.FactionID]struct{})
	totalTerritories := float32(len(lattice) - len(warpgates))
	for _, warpgate := range warpgates {
		factionCount[warpgate.Owner] = struct{}{}
		owned := float32(summary.FacilityCount[warpgate.Owner])
		summary.Territory[warpgate.Owner] = 100 * owned / totalTerritories
	}

	switch {
	// if all warpgates are owned by one faction then a continent is locked
	case len(factionCount) == 1:
		summary.Status = Locked

		// if any facilities are owned by faction 0 then the continent is probably in an unstable state.
		// however, the haunted bastion event can disable regions.
		// Five was chosen arbitrarily to distinguish between haunted bastions and unstable.
	case summary.CutoffCount[none] > 5:
		summary.Status = Unstable
	default:
		summary.Status = Unlocked
	}

	return summary, nil
}

// Summary describes territory control, continent status, etc. for a continent.
type Summary struct {
	// FacilityCount is the number of owned facilities for a faction, excluding warpgates and cut off regions
	FacilityCount map[ps2.FactionID]int

	// CutoffCount is the number of cut off regions owned by a faction. Disabled regions are listed here for faction 0.
	CutoffCount map[ps2.FactionID]int

	// Territory is the percentage of territory owned by a faction. The result should be cast to an int (floored) to align with the in-game numbers.
	Territory map[ps2.FactionID]float32

	// Status is the locked/unlocked status of the continent.
	Status Status
}

type Status uint8

func (s Status) String() string {
	switch s {
	case Locked:
		return "locked"
	case Unlocked:
		return "unlocked"
	case Unstable:
		return "unstable"
	default:
		return fmt.Sprintf("invalid_state(%d)", s)
	}
}

func (s Status) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%q", s.String())), nil
}

const (
	none = ps2.None
	nc   = ps2.NC
	vs   = ps2.VS
	tr   = ps2.TR
)
