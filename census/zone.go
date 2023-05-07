package census

import "github.com/Travis-Britz/ps2"

type Zone struct {
	ZoneID      ps2.ZoneID   `json:"zone_id,string"`
	Code        string       `json:"code"`
	HexSize     int          `json:"hex_size,string"`
	Name        Localization `json:"name"`
	Description Localization `json:"description"`
}
