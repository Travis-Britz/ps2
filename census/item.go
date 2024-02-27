package census

import (
	"github.com/Travis-Britz/ps2"
)

type Item struct {
	ItemID              ps2.ItemID         `json:"item_id,string"`
	ItemTypeID          ps2.ItemTypeID     `json:"item_type_id,string"`
	ItemCategoryID      ps2.ItemCategoryID `json:"item_category_id,string"`
	IsVehicleWeapon     stringNumericBool  `json:"is_vehicle_weapon"`
	Name                ps2.Localization   `json:"name"`
	Description         ps2.Localization   `json:"description"`
	FactionID           ps2.FactionID      `json:"faction_id,string"`
	MaxStackSize        int                `json:"max_stack_size,string"`
	ImageSetID          ps2.ImageSetID     `json:"image_set_id,string"`
	ImageID             ps2.ImageID        `json:"image_id,string"`
	ImagePath           string             `json:"image_path"`
	IsDefaultAttachment stringNumericBool  `json:"is_default_attachment"`
}

func (Item) CollectionName() string { return "item" }

func (i Item) ImageURL() string {
	return apiBase + i.ImagePath
}
