package census

import "github.com/Travis-Britz/ps2"

type Experience struct {
	ExperienceID          ps2.ExperienceID          `json:"experience_id,string"`
	Description           string                    `json:"description"`
	Xp                    float64                   `json:"xp,string"`
	ExperienceAwardTypeID ps2.ExperienceAwardTypeID `json:"experience_award_type_id,string"`
}

func (Experience) CollectionName() string { return "experience" }

type ExperienceAwardType struct {
	ExperienceAwardTypeID ps2.ExperienceAwardTypeID `json:"experience_award_type_id,string"`
	Name                  string                    `json:"name"`
}

func (ExperienceAwardType) CollectionName() string { return "experience_award_type" }
