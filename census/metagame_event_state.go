package census

import "github.com/Travis-Britz/ps2"

type MetagameEventState struct {
	MetagameEventStateID ps2.MetagameEventStateID `json:"metagame_event_state_id,string"`
	Name                 string                   `json:"name"`
}
