package census

import "github.com/Travis-Britz/ps2"

type World struct {
	WorldID     ps2.WorldID      `json:"world_id,string"`
	State       string           `json:"state"`
	Name        ps2.Localization `json:"name"`
	Description ps2.Localization `json:"description"`
}

func (World) CollectionName() string { return "world" }
