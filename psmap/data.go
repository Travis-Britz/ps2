package psmap

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Travis-Britz/ps2"
	"github.com/Travis-Britz/ps2/census"
)

// IgnoredRegions are regions that this package will attempt to remove from any data sources.
// Currently only one problematic region exists,
// but this way users can add new regions (if planetside ever gets any) without waiting for a package update from me.
var IgnoredRegions = []ps2.RegionID{
	18347, // Oshur Vast Expanse - this region is a line of hex tiles that circles the entire map and has no gameplay relevance. This is the only region in the game with empty tiles in the center, which also breaks my method for outlining regions.
}

// LoadData loads map zones, regions, facilities, and hexes from census.
// Results are cached indefinitely by the package.
// LoadData is lazily called automatically by functions in this package that need map data if it has not been cached yet.
// func LoadData() error {
// 	return errNotImplemented
// }

// todo: add context
func GetAllMapData(ctx context.Context, env ps2.Environment) (data []Map, err error) {
	res := censusMapResult{}
	err = census.GetEnv(
		ctx,
		env,
		"zone?c:join=map_region^list:1^inject_at:regions^hide:zone_id(map_hex^list:1^inject_at:hexes^hide:zone_id'map_region_id)"+
			"&c:join=facility_link^list:1^inject_at:links^hide:zone_id'description"+
			"&c:lang=en"+
			"&c:limit=5000",
		&res,
	)
	if err != nil {
		return data, err
	}
	for _, zone := range res.ZoneList {
		zoneData := Map{
			ZoneID:  zone.ZoneID,
			HexSize: zone.HexSize,
		}
		if cont, err := zone.ZoneID.ContinentID(); err == nil {
			if size, err := Size(cont); err == nil {
				zoneData.Size = size
			}
		}
		for _, region := range zone.MapRegions {
			if slices.Contains(IgnoredRegions, region.MapRegionID) {
				continue
			}
			mapregion := Region{
				RegionID:       region.MapRegionID,
				Name:           region.Name,
				FacilityID:     region.FacilityID,
				FacilityTypeID: region.Type,
				FacilityX:      region.LocationZ,
				FacilityY:      region.LocationX,
			}

			hexes := make([]Hex, 0, len(region.Hexes))
			for _, h := range region.Hexes {
				hexes = append(hexes, Hex{
					X:    h.X,
					Y:    h.Y,
					Type: h.HexType,
				})
			}
			mapregion.Hexes = hexes
			zoneData.Regions = append(zoneData.Regions, mapregion)
		}
		for _, link := range zone.FacilityLinks {
			zoneData.Links = append(zoneData.Links, Link{
				A: link.FacilityIDA,
				B: link.FacilityIDB,
			})
		}

		data = append(data, zoneData)
	}

	return data, nil
}

func GetMapData(cont ps2.ContinentID) (data Map, err error) {

	// get from cache somewhere in here?

	zone, err := cont.ZoneID()
	if err != nil {
		return data, err
	}
	data, err = getMapData(zone)
	if err != nil {
		return data, fmt.Errorf("psmap: get data: %w", err)
		// get lithafalcon
		// if err != nil {
		// get embedded data
		//}
		//
	}

	return
	// return data, errNotImplemented
}
func GetMapState(ctx context.Context, w ps2.WorldID, zone ...ps2.ZoneInstanceID) ([]State, error) {
	return getMapState(ctx, w, zone...)
}

func getMapData(zoneid ps2.ZoneID) (data Map, err error) {
	res := censusMapResult{}
	err = census.Get(
		context.Background(),
		fmt.Sprintf(
			"zone?zone_id=%d"+
				"&c:join=map_region^list:1^inject_at:regions^hide:zone_id(map_hex^list:1^inject_at:hexes^hide:zone_id'map_region_id)"+
				"&c:join=facility_link^list:1^inject_at:links^hide:zone_id'description"+
				"&c:lang=en",
			zoneid,
		),
		&res,
	)
	if err != nil {
		return data, err
	}
	if len(res.ZoneList) == 0 {
		return data, errors.New("no results")
	}
	zone := res.ZoneList[0]
	data.ZoneID = zone.ZoneID
	data.HexSize = zone.HexSize
	for _, region := range zone.MapRegions {

		// skip oshur vast expanse
		// todo: define a constant or variable of blacklisted regions
		if region.MapRegionID == 18347 {
			continue
		}
		mapregion := Region{
			RegionID:       region.MapRegionID,
			Name:           region.Name,
			FacilityID:     region.FacilityID,
			FacilityTypeID: region.Type,
			FacilityX:      region.LocationZ,
			FacilityY:      region.LocationX * -1,
		}

		hexes := make([]Hex, 0, len(region.Hexes))
		for _, h := range region.Hexes {
			hexes = append(hexes, Hex{
				X:    h.X,
				Y:    h.Y,
				Type: h.HexType,
			})
		}
		mapregion.Hexes = hexes
		data.Regions = append(data.Regions, mapregion)
	}

	for _, link := range zone.FacilityLinks {
		data.Links = append(data.Links, Link{
			A: link.FacilityIDA,
			B: link.FacilityIDB,
		})
	}
	return data, nil
}

func getMapState(ctx context.Context, world ps2.WorldID, zone ...ps2.ZoneInstanceID) (state []State, err error) {

	zoneids := []string{}
	for _, z := range zone {
		zoneids = append(zoneids, z.StringID())
	}
	query := "map?world_id=" + world.StringID() + "&zone_ids=" + strings.Join(zoneids, ",")
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
	if err = census.GetEnv(ctx, ps2.GetEnvironment(world), query, &response); err != nil {
		return nil, fmt.Errorf("psmap: get state: %w", err)
	}
	if len(response.MapList) < 1 {
		return nil, fmt.Errorf("no results")
	}

	for _, zonestate := range response.MapList {
		zone := State{
			ZoneID:    zonestate.ZoneID,
			Territory: map[ps2.RegionID]ps2.FactionID{},
			Timestamp: time.Now().UTC(),
		}
		for _, rd := range zonestate.Regions.Row {
			zone.Territory[rd.RowData.RegionID] = rd.RowData.FactionID
		}
		state = append(state, zone)
	}

	return state, nil
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
