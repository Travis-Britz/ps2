package census

import "github.com/Travis-Britz/ps2"

type Faction struct {
	FactionID      ps2.FactionID     `json:"faction_id,string"`
	Name           ps2.Localization  `json:"name"`
	CodeTag        string            `json:"code_tag"`
	ImageID        ps2.ImageID       `json:"image_id,string"`
	ImageSetID     ps2.ImageSetID    `json:"image_set_id,string"`
	ImagePath      string            `json:"image_path"`
	UserSelectable stringNumericBool `json:"user_selectable"`
}

func (f Faction) ImageURL() string {
	return apiBase + f.ImagePath
}

func (Faction) CollectionName() string { return "faction" }
