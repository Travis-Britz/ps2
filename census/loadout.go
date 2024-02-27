package census

import "github.com/Travis-Britz/ps2"

type Loadout struct {
	LoadoutID ps2.LoadoutID `json:"loadout_id,string"`
	ProfileID ps2.ProfileID `json:"profile_id,string"`
	FactionID ps2.FactionID `json:"faction_id,string"`
	CodeName  string        `json:"code_name"`
}

func (Loadout) CollectionName() string { return "loadout" }
