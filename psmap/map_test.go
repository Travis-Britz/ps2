package psmap_test

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/Travis-Britz/ps2"
	"github.com/Travis-Britz/ps2/census"
	"github.com/Travis-Britz/ps2/psmap"
)

const (
	None = ps2.None
	VS   = ps2.VS
	NC   = ps2.NC
	TR   = ps2.TR
	NSO  = ps2.NSO
)

func TestCalculatePercentages(t *testing.T) {
	tt := map[string]struct {
		File       string
		Territory  map[ps2.FactionID]float32
		Facilities map[ps2.FactionID]int
		Cutoff     map[ps2.FactionID]int
		Status     psmap.Status
	}{
		"Hossin 1 (unstable cut off)": {
			File: "testdata/hossin_map_1.json",
			Territory: map[ps2.FactionID]float32{
				VS: 3.5294118,
				NC: 35.294117,
				TR: 18.82353,
			},
			Facilities: map[ps2.FactionID]int{
				VS: 3,
				NC: 30,
				TR: 16,
			},
			Cutoff: map[ps2.FactionID]int{
				None: 33,
				VS:   3,
			},
			Status: psmap.Unstable,
		},
		"Hossin 2 (unstable cut off)": {
			File: "testdata/hossin_map_2.json",
			Territory: map[ps2.FactionID]float32{
				VS: 11.764706,
				NC: 9.411765,
				TR: 14.117647,
			},
			Facilities: map[ps2.FactionID]int{
				VS: 10,
				NC: 8,
				TR: 12,
			},
			Cutoff: map[ps2.FactionID]int{
				VS: 1,
			},
			Status: psmap.Unstable,
		},
		"Esamir 1": {
			File: "testdata/esamir_map_1.json",
			// This test fails. After exhaustive searching I couldn't find out how the game is calculating Esamir territory ownership.
			Territory: map[ps2.FactionID]float32{
				VS: 30.196079,
				NC: 43.137257,
				TR: 25.882353,
			},
			Facilities: map[ps2.FactionID]int{
				VS: 14,
				NC: 20,
				TR: 12,
			},
			Status: psmap.Unlocked,
		},
	}

	for name, expected := range tt {
		md, ms, err := loadMap(expected.File)
		if err != nil {
			t.Fatal(name, err)
		}
		got, err := psmap.Summarize(md, ms)
		if err != nil {
			t.Fatal(name, err)
		}
		if got.Status != expected.Status {
			t.Errorf("%s: expected %s; got %s", name, expected.Status, got.Status)
		}
		for faction, score := range expected.Territory {
			if score != got.Territory[faction] {
				t.Errorf("%s: expected %v for %s, got %v", name, score, faction, got.Territory[faction])
			}
		}

		for faction, score := range expected.Facilities {
			if score != got.FacilityCount[faction] {
				t.Errorf("%s: expected %v for %s, got %v", name, score, faction, got.FacilityCount[faction])
			}
		}

		for faction, score := range expected.Cutoff {
			if score != got.CutoffCount[faction] {
				t.Errorf("%s: expected %v for %s, got %v", name, score, faction, got.CutoffCount[faction])
			}
		}
	}
}

func loadMap(filename string) (data psmap.Map, ms psmap.State, err error) {
	ms = psmap.State{Territory: map[ps2.RegionID]ps2.FactionID{}}
	var regionsFilename string
	var linksFilename string
	switch {
	case strings.Contains(filename, "hossin"):
		regionsFilename = "testdata/hossin_map_regions.json"
		linksFilename = "testdata/hossin_links.json"
	case strings.Contains(filename, "esamir"):
		regionsFilename = "testdata/esamir_map_regions.json"
		linksFilename = "testdata/esamir_links.json"
	default:
		panic("idk put something here later")
	}

	linkFile, err := os.Open(linksFilename)
	if err != nil {
		return data, ms, err
	}
	defer linkFile.Close()
	regionFile, err := os.Open(regionsFilename)
	if err != nil {
		return data, ms, err
	}
	defer regionFile.Close()

	latticeFile, err := os.Open(filename)
	if err != nil {
		return data, ms, err
	}
	defer latticeFile.Close()

	owners := []factionOwners{}
	if err := json.NewDecoder(latticeFile).Decode(&owners); err != nil {
		return data, ms, err
	}
	for _, region := range owners {
		ms.Territory[region.RegionID] = region.FactionID
	}

	var links []census.FacilityLink
	if err := json.NewDecoder(linkFile).Decode(&links); err != nil {
		return data, ms, err
	}
	for _, link := range links {
		data.Links = append(data.Links, psmap.Link{A: link.FacilityIDA, B: link.FacilityIDB})
	}
	var regions []census.MapRegion
	if err := json.NewDecoder(regionFile).Decode(&regions); err != nil {
		return data, ms, err
	}
	for _, region := range regions {
		data.Regions = append(data.Regions, psmap.Region{
			RegionID:       region.MapRegionID,
			Name:           region.Name,
			FacilityID:     region.FacilityID,
			FacilityTypeID: region.Type,
		})
	}
	return data, ms, nil
}

type factionOwners struct {
	RegionID  ps2.RegionID  `json:"region_id,string"`
	FactionID ps2.FactionID `json:"faction_id,string"`
}

func ExampleLoc_Bearing() {
	fmt.Println(psmap.Loc{Heading: 1.570}.Bearing())
	fmt.Println(psmap.Loc{Heading: 0.785}.Bearing())
	fmt.Println(psmap.Loc{Heading: 0}.Bearing())
	fmt.Println(psmap.Loc{Heading: -0.785}.Bearing())
	fmt.Println(psmap.Loc{Heading: -1.570}.Bearing())
	fmt.Println(psmap.Loc{Heading: -2.356}.Bearing())
	fmt.Println(psmap.Loc{Heading: -3.142}.Bearing())
	fmt.Println(psmap.Loc{Heading: 3.142}.Bearing())
	fmt.Println(psmap.Loc{Heading: 2.356}.Bearing())

	// Output:
	// 0
	// 45
	// 90
	// 135
	// 180
	// 225
	// 270
	// 270
	// 315

}
