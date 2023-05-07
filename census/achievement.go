package census

import "github.com/Travis-Britz/ps2"

type Achievement struct {
	AchievementID    ps2.AchievementID `json:"achievement_id,string"`
	ItemID           ps2.ItemID        `json:"item_id,string"`
	ObjectiveGroupID string            `json:"objective_group_id"`
	RewardID         ps2.RewardID      `json:"reward_id,string"`
	Repeatable       string            `json:"repeatable"` //bool?
	Name             Localization      `json:"name"`
	Description      Localization      `json:"description"`
	ImageSetID       int               `json:"image_set_id"`
	ImageID          int               `json:"image_id"`
	ImagePath        string            `json:"image_path"`
}
