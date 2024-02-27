package census

import "github.com/Travis-Britz/ps2"

type Profile struct {
	ProfileID              ps2.ProfileID     `json:"profile_id,string"`
	ProfileTypeID          ps2.ProfileTypeID `json:"profile_type_id,string"`
	ProfileTypeDescription string            `json:"profile_type_description"`
	FactionID              ps2.FactionID     `json:"faction_id,string"`
	Name                   ps2.Localization  `json:"name"`
	Description            ps2.Localization  `json:"description"`
	ImageSetID             ps2.ImageSetID    `json:"image_set_id,string"`
	ImageID                ps2.ImageID       `json:"image_id,string"`
	ImagePath              string            `json:"image_path"`
	MovementSpeed          float64           `json:"movement_speed,string"`
	BackpedalSpeedModifier float64           `json:"backpedal_speed_modifier,string"`
	SprintSpeedModifier    float64           `json:"sprint_speed_modifier,string"`
	StrafeSpeedModifier    float64           `json:"strafe_speed_modifier,string"`
}

func (p Profile) ImageURL() string {
	return apiBase + p.ImagePath
}
func (Profile) CollectionName() string { return "profile" }
