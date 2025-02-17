// Package psmap deals with PlanetSide 2 maps.
package psmap

import (
	"fmt"
	"math"
	"time"

	"github.com/Travis-Britz/ps2"
	"github.com/Travis-Britz/structures/stack"
)

const (
	Locked Status = iota
	Unlocked
	Unstable
)

// Map contains the data required for drawing a map.
type Map struct {
	ZoneID ps2.ZoneID `json:"zone_id"`

	// Size is the full size LOD0 map in pixels.
	// Standard continents are 8192x8192.
	// Some instanced zones have smaller dimensions.
	Size int `json:"size,omitempty"`

	// HexSize is the size of each hex as reported by Census.
	HexSize int `json:"hex_size"`

	Regions []Region `json:"regions"`

	Links []Link `json:"links"`
}

// Region contains the data required for drawing a map region.
type Region struct {
	RegionID ps2.RegionID `json:"region_id"`
	Name     string       `json:"name"`

	// FacilityID is the ID of the facility located in the region.
	// Regions without a facility will have 0 as the value.
	FacilityID ps2.FacilityID `json:"facility_id,omitempty"`

	// FacilityTypeID is the type of facility such as Warpgate, Amp Station, etc.
	// A value of 0 is equivalent to missing data.
	FacilityTypeID ps2.FacilityTypeID `json:"facility_type_id,omitempty"`

	// FacilityX is the X coordinate on a Cartesian plane as returned by census.
	// The center of the map is (0,0).
	// Census is missing data for some construction facilities.
	// Since no facilities are located at exactly (0,0),
	// any facility at those coordinates can be assumed to have missing data.
	FacilityX float64 `json:"facility_x,omitempty"`

	// FacilityY is the Y coordinate on a Cartesian plane as returned by census.
	// The center of the map is (0,0).
	// Census is missing data for some construction facilities.
	// Since no facilities are located at exactly (0,0),
	// any facility at those coordinates can be assumed to have missing data.
	FacilityY float64 `json:"facility_y,omitempty"`

	// Hexes is the slice of map hex tiles that are part of the region.
	Hexes []Hex `json:"hexes"`
}

// Point returns the coordinates for the facility.
func (r Region) Point() (float64, float64) {
	return r.FacilityX, r.FacilityY
}

// Hex represents the position of a single map Hex tile.
// X,Y correspond to the game's tile grid as returned by the census /map_hex endpoint.
type Hex struct {
	X int `json:"x"`
	Y int `json:"y"`

	// Type is the type of hex.
	// 0 is a valid value and the type for standard unrestricted hexes,
	// but storing it in json for every tile is redundant.
	Type ps2.MapHexType `json:"type,omitempty"`
}

// Link represents a map lattice link between two facilities.
type Link struct {
	A ps2.FacilityID `json:"facility_a"`
	B ps2.FacilityID `json:"facility_b"`
}

// Loc represents a map location as reported by the /loc command in-game.
// /loc command result:
// x=3211.266 y=470.785 z=3136.692, Heading: 0.681   /loc 3211.266 470.785 3136.692
type Loc struct {
	// X is the inverted Y axis (multiply by -1)
	X float64

	// Y is elevation
	Y float64

	// Z is the X axis
	Z float64

	// Heading is the angle in radians in range [-π,π], with 0 being East.
	Heading float64
}

// Bearing returns the compass bearing direction in degrees [0,360) where north is 0 and degrees increase clockwise.
func (l Loc) Bearing() float64 {
	// use math.Round because int(float) floors the result
	deg := math.Round(l.Heading * (180 / math.Pi))
	deg += 180               // adjust from [-π,π] to [0,2π]
	deg += 90                // move 0 from east to north
	deg = math.Mod(deg, 360) // now deg is in the mathematical convention of 0 degrees = east, increasing counter-clockwise

	deg = (360 - deg) // convert to compass bearing convention of north 0 increasing clockwise

	return math.Mod(deg, 360) // mod again so 360 becomes 0
}

// Point converts the location to a cartesian coordinate where 0,0 is the center of the map.
func (l Loc) Point() (float64, float64) {
	return l.Z, l.X * -1
}

// State describes the territory ownership of a continent.
type State struct {
	ZoneID    ps2.ZoneInstanceID
	Timestamp time.Time
	Territory map[ps2.RegionID]ps2.FactionID
}

func (s State) Owner(r ps2.RegionID) ps2.FactionID {
	return s.Territory[r]
}

type owner interface {
	Owner(ps2.RegionID) ps2.FactionID
}

// facilityRegion is our node type for graph traversal.
type facilityRegion struct {
	RegionID   ps2.RegionID
	FacilityID ps2.FacilityID
	Owner      ps2.FactionID
	Cutoff     bool
	Links      []*facilityRegion
}

// Summarize calculates territory ownership percentages,
// factoring in cutoff territory and disabled regions.
//
// result represents territory ownership of a zone.
// The faction key will correspond to warpgate ownership;
// on Nexus (outfit wars) there may only be two teams.
func Summarize(data Map, regions owner) (summary Summary, err error) {
	summary = Summary{
		Territory:     map[ps2.FactionID]float32{},
		FacilityCount: map[ps2.FactionID]int{},
		CutoffCount:   map[ps2.FactionID]int{},
		Cutoff:        map[ps2.RegionID]bool{},
	}
	lattice := make(map[ps2.FacilityID]*facilityRegion) // lattice is the graph of facility connections
	regionIdx := make(map[ps2.RegionID]ps2.FacilityID)  // regionIdx maps RegionIDs to FacilityIDs
	warpgates := make([]*facilityRegion, 0, 3)

	// start by building a graph of connected facilities
	for _, reg := range data.Regions {
		// the census /map endpoint gives facility ownership by region id,
		// but not every region has a facility.
		// regions without a facility will typically be owned by faction 0 (None)
		if reg.FacilityID == 0 {
			continue
		}

		r := &facilityRegion{
			RegionID:   reg.RegionID,
			FacilityID: reg.FacilityID,
			Owner:      regions.Owner(reg.RegionID),
			// Cutoff:     true, // every region starts as cut off, then as we visit each region in the graph we mark it as available
		}
		lattice[reg.FacilityID] = r
		summary.CutoffCount[r.Owner]++
		if r.Owner != none {
			summary.Cutoff[reg.RegionID] = true
		}

		regionIdx[reg.RegionID] = reg.FacilityID
		if reg.FacilityTypeID == ps2.Warpgate {
			warpgates = append(warpgates, r)
		}
	}
	// go through the list of lattice connections and build up the list of neighbor facilities for each facility
	for _, link := range data.Links {

		// check for referenced facilities that don't exist.
		// we can't always trust census to be consistent,
		// and we don't need any nil pointers to dereference.
		fA, ok := lattice[link.A]
		if !ok {
			return summary, fmt.Errorf("a facility link referenced a facility missing from the supplied map data; link: %v, facility: %v", link, link.A)
		}
		fB, ok := lattice[link.B]
		if !ok {
			return summary, fmt.Errorf("a facility link referenced a facility missing from the supplied map data; link: %v, facility: %v", link, link.B)
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
		delete(summary.Cutoff, start.RegionID)
		for current, more := start, true; more; current, more = frontier.Pop() {
			for _, next := range current.Links {
				if visited[next.FacilityID] {
					continue
				}
				if next.Owner == current.Owner {
					frontier.Push(next)
					visited[next.FacilityID] = true
					delete(summary.Cutoff, next.RegionID)
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

	Cutoff map[ps2.RegionID]bool

	// Status is the locked/unlocked status of the continent.
	Status Status

	//todo: owning faction for locked continents?
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
	nso  = ps2.NSO
)
