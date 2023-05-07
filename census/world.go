package census

import "github.com/Travis-Britz/ps2"

type World struct {
	WorldID ps2.WorldID  `json:"world_id,string"`
	State   string       `json:"state"`
	Name    Localization `json:"name"`
}
