package census

import "github.com/Travis-Britz/ps2"

type Faction struct {
	FactionID      ps2.FactionID `json:"faction_id,string"`
	Name           Localization  `json:"name"`
	CodeTag        string        `json:"code_tag"`
	UserSelectable string        `json:"user_selectable"`
}
