package census

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Travis-Britz/ps2"
)

type MapHex struct {
	ZoneID      ps2.ZoneID     `json:"zone_id,string"`
	MapRegionID ps2.RegionID   `json:"map_region_id,string"`
	X           int            `json:"x,string"`
	Y           int            `json:"y,string"`
	HexType     ps2.MapHexType `json:"hex_type,string"`
	TypeName    string         `json:"type_name"`
}

func (MapHex) CollectionName() string { return "map_hex" }

type MapRegion struct {
	MapRegionID ps2.RegionID       `json:"map_region_id,string"`
	FacilityID  ps2.FacilityID     `json:"facility_id,string"`
	ZoneID      ps2.ZoneID         `json:"zone_id,string"`
	Name        string             `json:"facility_name"`
	Type        ps2.FacilityTypeID `json:"facility_type_id,string"`
	TypeName    string             `json:"facility_type"`
	LocationX   float64            `json:"location_x,string"`
	LocationY   float64            `json:"location_y,string"`
	LocationZ   float64            `json:"location_z,string"`
}

func (r MapRegion) Region() ps2.RegionID             { return r.MapRegionID }
func (r MapRegion) Facility() ps2.FacilityID         { return r.FacilityID }
func (r MapRegion) FacilityType() ps2.FacilityTypeID { return r.Type }

func (MapRegion) CollectionName() string { return "map_region" }

// Facility is a capturable game facility.
//
// Note: The census collection this type uses is shared with MapRegion
// because there is no collection for facilities.
// Not all map regions have a facility,
// which means when loading facilities from census
// (like with [census.LoadCollection]) you will need to filter out facilities with a facility ID of 0.
type Facility struct {
	FacilityID ps2.FacilityID     `json:"facility_id,string"`
	ZoneID     ps2.ZoneID         `json:"zone_id,string"`
	Name       string             `json:"facility_name"`
	Type       ps2.FacilityTypeID `json:"facility_type_id,string"`
	TypeName   string             `json:"facility_type"`
	LocationX  float64            `json:"location_x,string"`
	LocationY  float64            `json:"location_y,string"`
	LocationZ  float64            `json:"location_z,string"`
}

func (f Facility) FacilityType() ps2.FacilityTypeID { return f.Type }
func (f Facility) Facility() ps2.FacilityID         { return f.FacilityID }

func (Facility) CollectionName() string { return "map_region" }

type FacilityType struct {
	FacilityTypeID ps2.FacilityTypeID `json:"facility_type_id,string"`
	Description    string             `json:"description"`
	ImageID        ps2.ImageID        `json:"image_id,string"`
	ImageSetID     ps2.ImageSetID     `json:"image_set_id,string"`
	ImagePath      string             `json:"image_path"`
}

func (f FacilityType) ImageURL() string {
	return apiBase + f.ImagePath
}

func (FacilityType) CollectionName() string { return "facility_type" }

type FacilityLink struct {
	ZoneID      ps2.ZoneID     `json:"zone_id,string"`
	FacilityIDA ps2.FacilityID `json:"facility_id_a,string"`
	FacilityIDB ps2.FacilityID `json:"facility_id_b,string"`
	Description string         `json:"description"`
}

func (fl FacilityLink) A() ps2.FacilityID { return fl.FacilityIDA }
func (fl FacilityLink) B() ps2.FacilityID { return fl.FacilityIDB }

func (FacilityLink) CollectionName() string { return "facility_link" }

type Region struct {
	RegionID         ps2.RegionID     `json:"region_id,string"`
	ZoneID           ps2.ZoneID       `json:"zone_id,string"`
	InitialFactionID ps2.FactionID    `json:"initial_faction_id,string"`
	Name             ps2.Localization `json:"name"`
}

func (Region) CollectionName() string { return "region" }

func GetMap(ctx context.Context, client *Client, world ps2.WorldID, zone ...ps2.ZoneInstanceID) (zm []ZoneState, err error) {
	zones := make([]string, 0, 5)
	for _, z := range zone {
		zones = append(zones, z.StringID())
	}
	query := "map?world_id=" + world.StringID() + "&zone_ids=" + strings.Join(zones, ",")
	var response struct {
		MapList []struct {
			ZoneID  ps2.ZoneInstanceID `json:"ZoneId,string"`
			Regions struct {
				IsList stringNumericBool `json:"IsList"`
				Row    []struct {
					RowData struct {
						RegionID  ps2.RegionID  `json:"RegionId,string"`
						FactionID ps2.FactionID `json:"FactionId,string"`
					} `json:"RowData"`
				} `json:"Row"`
			} `json:"Regions"`
		} `json:"map_list"`
		Returned int `json:"returned"`
	}
	if err = client.Get(ctx, ps2.PC, query, &response); err != nil {
		return zm, fmt.Errorf("census.GetMap: %w", err)
	}
	for _, z := range response.MapList {
		var m ZoneState
		m.WorldID = world
		m.ZoneInstanceID = z.ZoneID
		m.Timestamp = time.Now()
		for _, rd := range z.Regions.Row {
			m.Regions = append(m.Regions, RegionControl{RegionID: rd.RowData.RegionID, FactionID: rd.RowData.FactionID})
		}
		zm = append(zm, m)
	}
	return zm, err
}

type ZoneState struct {
	ps2.WorldID
	ps2.ZoneInstanceID
	Regions   []RegionControl
	Timestamp time.Time
}

func (zm ZoneState) IsUnstable() bool {
	for _, r := range zm.Regions {
		if r.FactionID == ps2.None && !isMissingFacility(r.RegionID) {
			return true
		}
	}
	return false
}
func (zm ZoneState) IsLocked() bool {
	warpgateCount := make(map[ps2.FactionID]int)
	for _, r := range zm.Regions {
		if isWarpgate(r.RegionID) {
			warpgateCount[r.FactionID]++
			if warpgateCount[r.FactionID] > 1 {
				return true
			}
		}
	}
	return false
}

type RegionControl struct {
	ps2.RegionID
	ps2.FactionID
}

// stringNumericBool is a bool value represented as "0" or "1" with json.
type stringNumericBool bool

func (b *stringNumericBool) UnmarshalJSON(data []byte) error {
	data = bytes.Trim(data, "\"")
	if bytes.Equal(data, []byte("1")) {
		*b = true
	}
	return nil
}

func isMissingFacility(r ps2.RegionID) bool {
	regions := []ps2.RegionID{
		18328,
		18347,
		18352,
		18354,
		18357,
		18358,
		18262,
		18249,
	}
	return slices.Contains(regions, r)
}

var warpgatePositions = map[ps2.RegionID]string{
	2201:  "⬆",
	2202:  "⬅",
	2203:  "➡",
	4230:  "⬅",
	4240:  "➡",
	4250:  "⬇",
	6001:  "⬅",
	6002:  "➡",
	6003:  "⬇",
	18029: "⬆",
	18030: "⬇",
	18062: "➡",
	18303: "↗",
	18304: "↖",
	18305: "⬇",
}

func isWarpgate(r ps2.RegionID) bool {
	_, found := warpgatePositions[r]
	return found
}
