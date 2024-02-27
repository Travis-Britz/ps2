package census

import "github.com/Travis-Britz/ps2"

type ArmorInfo struct {
	ArmorInfoID   ps2.ArmorInfoID   `json:"armor_info_id,string"`
	ArmorFacingID ps2.ArmorFacingID `json:"armor_facing_id,string"`
	ArmorPercent  int               `json:"armor_percent,string"` // Armor percent as an integer, e.g. 40 == 40% and -15 == -15%
	Description   string            `json:"description"`
}

func (ArmorInfo) CollectionName() string { return "armor_info" }
