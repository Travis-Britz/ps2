package census

import "github.com/Travis-Britz/ps2"

type ItemType struct {
	ItemTypeID ps2.ItemTypeID `json:"item_type_id,string"`
	Name       string         `json:"name"`
	Code       string         `json:"code"`
}

func (ItemType) CollectionName() string { return "item_type" }
