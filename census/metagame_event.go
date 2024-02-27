package census

import (
	"encoding/json"
	"time"

	"github.com/Travis-Britz/ps2"
)

type MetagameEvent struct {
	MetagameEventID ps2.MetagameEventID   `json:"metagame_event_id,string"`
	Name            ps2.Localization      `json:"name"`
	Description     ps2.Localization      `json:"description"`
	Type            ps2.MetagameEventType `json:"type,string"`
	ExperienceBonus int                   `json:"experience_bonus,string"`
	Duration        time.Duration         `json:"duration_minutes,string"`
}

func (MetagameEvent) CollectionName() string { return "metagame_event" }

func (e *MetagameEvent) UnmarshalJSON(data []byte) error {
	// shadow the type to prevent infinite recursion
	type shadowType MetagameEvent
	var shadowcopy shadowType
	if err := json.Unmarshal(data, &shadowcopy); err != nil {
		return err
	}
	shadowcopy.Duration *= time.Minute
	*e = MetagameEvent(shadowcopy)
	return nil
}

func (e *MetagameEvent) MarshalJSON() ([]byte, error) {
	// shadow the types to prevent infinite recursion
	type shadowType MetagameEvent
	copy := *e
	copy.Duration /= time.Minute
	return json.Marshal(shadowType(copy))
}
