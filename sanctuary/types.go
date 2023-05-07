package sanctuary

import "github.com/Travis-Britz/ps2"

type FacilityInfo struct {
	ZoneID         ps2.ZoneID         `json:"zone_id,string"`
	FacilityID     ps2.FacilityID     `json:"facility_id,string"`
	FacilityName   ps2.Localization   `json:"facility_name"`
	FacilityTypeID ps2.FacilityTypeID `json:"facility_type_id,string"`
	LocationX      float64            `json:"location_x,string"`
	LocationY      float64            `json:"location_y,string"`
	LocationZ      float64            `json:"location_z,string"`
}
type MapRegion struct {
	ps2.MapRegion
	LocalizedFacilityName ps2.Localization `json:"localized_facility_name"`
}
