package census

import "github.com/Travis-Britz/ps2"

type MapHex struct {
	ZoneID      ps2.ZoneID      `json:"zone_id,string"`
	MapRegionID ps2.MapRegionID `json:"map_region_id,string"`
	X           int             `json:"x,string"`
	Y           int             `json:"y,string"`
	HexType     ps2.MapHexType  `json:"hex_type,string"`
	TypeName    string          `json:"type_name"`
}
