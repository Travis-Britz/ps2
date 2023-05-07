package census

import "github.com/Travis-Britz/ps2"

type MetagameEvent struct {
	MetagameEventID ps2.MetagameEventID `json:"metagame_event_id"`
	Name            Localization        `json:"name"`
	Description     Localization        `json:"description"`
	Type            int                 `json:"type,string"`
	ExperienceBonus string              `json:"experience_bonus"`
}
