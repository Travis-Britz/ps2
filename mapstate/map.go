package mapstate

import (
	"fmt"

	"github.com/Travis-Britz/ps2"
	"github.com/Travis-Britz/structures/stack"
)

type Region struct {
	RegionID   ps2.RegionID
	FacilityID ps2.FacilityID
	Type       ps2.FacilityTypeID
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

type RegionOwnership struct {
	Region ps2.RegionID
	Owner  ps2.FactionID
}

type MapData struct {
	Regions []Region
	Links   []Link
}

type region struct {
	RegionID   ps2.RegionID
	FacilityID ps2.FacilityID
	Owner      ps2.FactionID
	Cutoff     bool
	Links      []*region
}

// CalculatePercentages takes calculates territory ownership percentages,
// factoring in cutoff territory and disabled regions.
//
// result represents territory ownership of a zone.
// The faction key will correspond to warpgate ownership;
// on Nexus (outfit wars) there may only be two teams.
func CalculatePercentages(mapData MapData, regions []RegionOwnership) (result territory, err error) {
	graph := make(map[ps2.FacilityID]*region)
	regionIdx := make(map[ps2.RegionID]ps2.FacilityID)
	warpgates := make([]*region, 0, 3)

	for _, reg := range mapData.Regions {
		if reg.FacilityID == 0 {
			continue
		}
		r := &region{
			RegionID:   reg.RegionID,
			FacilityID: reg.FacilityID,
			Cutoff:     true,
			Links:      []*region{},
		}
		graph[reg.FacilityID] = r
		regionIdx[reg.RegionID] = reg.FacilityID
		if reg.Type == ps2.Warpgate {
			warpgates = append(warpgates, r)
		}
	}
	for _, link := range mapData.Links {
		fA, ok := graph[link.A]
		if !ok {
			return nil, fmt.Errorf("a facility link referenced a facility missing from the supplied map data; link: %v, facility: %v", link, link.A)
		}
		fB, ok := graph[link.B]
		if !ok {
			return nil, fmt.Errorf("a facility link referenced a facility missing from the supplied map data; link: %v, facility: %v", link, link.B)
		}

		fA.Links = append(fA.Links, fB)
		fB.Links = append(fB.Links, fA)

	}

	for _, reg := range regions {
		id := regionIdx[reg.Region]
		r, ok := graph[id]
		if !ok {
			continue
		}
		r.Owner = reg.Owner
	}

	frontier := &stack.Stack[*region]{}
	visited := map[ps2.FacilityID]bool{}

	for _, start := range warpgates {
		start.Cutoff = false
		visited[start.FacilityID] = true
		for current, more := start, true; more; current, more = frontier.Pop() {
			for _, next := range current.Links {
				if visited[next.FacilityID] {
					continue
				}
				if next.Owner == current.Owner {
					frontier.Push(next)
					visited[next.FacilityID] = true
					next.Cutoff = false
				}
			}
		}
	}
	ownedCount := make(map[ps2.FactionID]int)
	cutOffCount := make(map[ps2.FactionID]int)
	for _, region := range regions {
		id := regionIdx[region.Region]
		r := graph[id]
		if !r.Cutoff {
			ownedCount[r.Owner]++
		} else {
			cutOffCount[r.Owner]++
		}

	}
	result = make(territory)
	for _, warpgate := range warpgates {
		result[warpgate.Owner] = 100 * float32(ownedCount[warpgate.Owner]-1) / float32(len(regions)-len(warpgates))
	}
	return result, nil
}

type territory map[ps2.FactionID]float32
