package census

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Travis-Britz/ps2"
)

func GetMap(ctx context.Context, client *Client, world ps2.WorldID, zone ...ps2.ZoneInstanceID) (zm []ZoneMap, err error) {
	zones := make([]string, 0, 5)
	for _, z := range zone {
		zones = append(zones, z.String())
	}
	query := "map?world_id=" + world.String() + "&zone_ids=" + strings.Join(zones, ",")
	var resp mapResponse
	if err = client.Get(ctx, ps2.EnvPC, query, &resp); err != nil {
		return zm, fmt.Errorf("census.GetMap: %w", err)
	}
	for _, z := range resp.MapList {
		var m ZoneMap
		m.WorldID = world
		m.ZoneInstanceID = z.ZoneID
		m.Timestamp = time.Now()
		for _, rd := range z.Regions.Row {
			m.Regions = append(m.Regions, RegionControl{MapRegionID: rd.RowData.RegionID, FactionID: rd.RowData.FactionID})
		}
		zm = append(zm, m)
	}
	return zm, err
}

type mapResponse struct {
	MapList []struct {
		ZoneID  ps2.ZoneInstanceID `json:"ZoneId,string"`
		Regions struct {
			IsList stringNumericBool `json:"IsList"`
			Row    []struct {
				RowData struct {
					RegionID  ps2.MapRegionID `json:"RegionId,string"`
					FactionID ps2.FactionID   `json:"FactionId,string"`
				} `json:"RowData"`
			} `json:"Row"`
		} `json:"Regions"`
	} `json:"map_list"`
	Returned int `json:"returned"`
}

type ZoneMap struct {
	ps2.WorldID
	ps2.ZoneInstanceID
	Regions   []RegionControl
	Timestamp time.Time
}

func (zm ZoneMap) IsUnstable() bool {
	for _, r := range zm.Regions {
		if r.FactionID == ps2.FactionUnknown && !isMissingFacility(r.MapRegionID) {
			return true
		}
	}
	return false
}
func (zm ZoneMap) IsLocked() bool {
	warpgateCount := make(map[ps2.FactionID]int)
	for _, r := range zm.Regions {
		if isWarpgate(r.MapRegionID) {
			warpgateCount[r.FactionID]++
			if warpgateCount[r.FactionID] > 1 {
				return true
			}
		}
	}
	return false
}

type RegionControl struct {
	ps2.MapRegionID
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

var warpgatePositions = map[ps2.MapRegionID]string{
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

func isWarpgate(r ps2.MapRegionID) bool {
	_, found := warpgatePositions[r]
	return found
}

func isMissingFacility(r ps2.MapRegionID) bool {
	regions := []ps2.MapRegionID{
		18328,
		18347,
		18352,
		18354,
		18357,
		18358,
		18262,
		18249,
	}
	for _, rr := range regions {
		if r == rr {
			return true
		}
	}
	return false
}
