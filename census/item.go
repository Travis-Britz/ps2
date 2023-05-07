package census

import "github.com/Travis-Britz/ps2"

type Item struct {
	ItemID              ps2.ItemID         `json:"item_id,string"`
	ItemTypeID          ps2.ItemTypeID     `json:"item_type_id,string"`
	ItemCategoryID      ps2.ItemCategoryID `json:"item_category_id,string"`
	IsVehicleWeapon     string             `json:"is_vehicle_weapon"`
	Name                Localization       `json:"name"`
	Description         Localization       `json:"description"`
	FactionID           ps2.FactionID      `json:"faction_id,string"`
	MaxStackSize        string             `json:"max_stack_size"`
	ImageSetID          string             `json:"image_set_id"`
	ImageID             string             `json:"image_id"`
	ImagePath           string             `json:"image_path"`
	IsDefaultAttachment string             `json:"is_default_attachment"`
}
