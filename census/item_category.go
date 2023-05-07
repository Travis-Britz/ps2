package census

import "github.com/Travis-Britz/ps2"

type ItemCategory struct {
	ItemCategoryID ps2.ItemCategoryID `json:"item_category_id,string"`
	Name           Localization       `json:"name"`
}
