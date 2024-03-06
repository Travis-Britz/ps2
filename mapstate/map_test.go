package mapstate_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/Travis-Britz/ps2"
	"github.com/Travis-Britz/ps2/mapstate"
)

const (
	None = ps2.None
	VS   = ps2.VS
	NC   = ps2.NC
	TR   = ps2.TR
	NSO  = ps2.NSO
)

func TestCalculatePercentages(t *testing.T) {
	linkFile, err := os.Open("testdata/hossin_links.json")
	if err != nil {
		t.Fatalf("couldn't open test data file: %v", err)
	}
	defer linkFile.Close()

	rawLinks := []struct {
		A ps2.FacilityID `json:"facility_id_a,string"`
		B ps2.FacilityID `json:"facility_id_b,string"`
	}{}

	if err := json.NewDecoder(linkFile).Decode(&rawLinks); err != nil {
		t.Fatal(err)
	}

	regionFile, err := os.Open("testdata/hossin_map_region.json")
	if err != nil {
		t.Fatalf("couldn't open test data file: %v", err)
	}
	defer regionFile.Close()

	rawRegions := []struct {
		RegionID   ps2.RegionID       `json:"map_region_id,string"`
		FacilityID ps2.FacilityID     `json:"facility_id,string"`
		Type       ps2.FacilityTypeID `json:"facility_type_id,string"`
	}{}

	if err := json.NewDecoder(regionFile).Decode(&rawRegions); err != nil {
		t.Fatal(err)
	}

	mapData := mapstate.MapData{}
	for _, link := range rawLinks {
		mapData.Links = append(mapData.Links, mapstate.Link{A: link.A, B: link.B})
	}
	for _, region := range rawRegions {
		mapData.Regions = append(
			mapData.Regions,
			mapstate.Region{
				FacilityID: region.FacilityID,
				RegionID:   region.RegionID,
				Type:       region.Type,
			},
		)
	}

	latticeFile, err := os.Open("testdata/hossin_map_2.json")
	if err != nil {
		t.Fatalf("couldn't open test data file: %v", err)
	}
	defer latticeFile.Close()

	rawOwners := []struct {
		Region  ps2.RegionID  `json:"region_id,string"`
		Faction ps2.FactionID `json:"faction_id,string"`
	}{}

	if err := json.NewDecoder(latticeFile).Decode(&rawOwners); err != nil {
		t.Fatal(err)
	}
	latticeState := []mapstate.RegionOwnership{}
	for _, owner := range rawOwners {
		latticeState = append(latticeState, mapstate.RegionOwnership{
			Region: owner.Region,
			Owner:  owner.Faction,
		})
	}

	result, err := mapstate.CalculatePercentages(mapData, latticeState)
	if err != nil {
		t.Fatal(err)
	}

	expected := map[ps2.FactionID]float32{
		NC: 9.411765,
		VS: 11.764706,
		TR: 14.117647,
	}

	for faction, score := range expected {
		if score != result[faction] {
			t.Fatalf("expected %v for %s; got %v", score, faction, result[faction])
		}
	}
}
