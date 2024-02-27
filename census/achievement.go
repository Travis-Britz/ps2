package census

import "github.com/Travis-Britz/ps2"

type Achievement struct {
	AchievementID    ps2.AchievementID    `json:"achievement_id,string"`
	ItemID           ps2.ItemID           `json:"item_id,string"`
	ObjectiveGroupID ps2.ObjectiveGroupID `json:"objective_group_id,string"`
	RewardID         ps2.RewardID         `json:"reward_id,string"`
	Repeatable       stringNumericBool    `json:"repeatable"`
	Name             ps2.Localization     `json:"name"`
	Description      ps2.Localization     `json:"description"`
	ImageSetID       ps2.ImageSetID       `json:"image_set_id,string"`
	ImageID          ps2.ImageID          `json:"image_id,string"`
	ImagePath        string               `json:"image_path"`
}

func (a Achievement) CollectionName() string { return "achievement" }

func (a Achievement) ImageURL() string { return apiBase + a.ImagePath }
