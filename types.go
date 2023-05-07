package ps2

import (
	"bytes"
	"fmt"
	"time"
)

type Environment uint8

func (e Environment) String() string {
	switch e {
	case EnvPC:
		return "ps2"
	case EnvPS4US:
		return "ps2ps4us"
	case EnvPS4EU:
		return "ps2ps4eu"
	default:
		return ""
	}
}

func GetEnvironment(w WorldID) Environment {
	switch w {
	case Ceres, Lithcorp, Rashnu:
		return EnvPS4EU
	case Genudine, Palos, Crux, Searhus, Xelas:
		return EnvPS4US
	default:
		return EnvPC
	}
}

type Event uint8

const (
	Unknown Event = iota
	ContinentLock
	ContinentUnlock
	PlayerLogin
	PlayerLogout
	GainExperience
	VehicleDestroy
	Death
	AchievementEarned
	BattleRankUp
	ItemAdded
	Metagame
	FacilityControl
	PlayerFacilityCapture
	PlayerFacilityDefend
	SkillAdded
)

var events = map[Event]string{
	ContinentLock:         "ContinentLock",
	ContinentUnlock:       "ContinentUnlock",
	PlayerLogin:           "PlayerLogin",
	PlayerLogout:          "PlayerLogout",
	GainExperience:        "GainExperience",
	VehicleDestroy:        "VehicleDestroy",
	Death:                 "Death",
	AchievementEarned:     "AchievementEarned",
	BattleRankUp:          "BattleRankUp",
	ItemAdded:             "ItemAdded",
	Metagame:              "MetagameEvent",
	FacilityControl:       "FacilityControl",
	PlayerFacilityCapture: "PlayerFacilityCapture",
	PlayerFacilityDefend:  "PlayerFacilityDefend",
	SkillAdded:            "SkillAdded",
}

func (t Event) EventName() string { return t.String() }

func (e Event) String() string { return events[e] }

func (e *Event) UnmarshalJSON(data []byte) error {
	data = bytes.Trim(data, "\"")
	for ev, s := range events {
		if bytes.Equal(data, []byte(s)) {
			*e = ev
			return nil
		}
	}
	return fmt.Errorf("event.UnmarshalJSON: invalid value '%s' for event", data)
}

func (e Event) MarshalJSON() ([]byte, error) {
	return []byte("\"" + e.String() + "\""), nil
}

type MetagameEventCategory uint

var categoryData = map[MetagameEventCategory]struct {
	duration        time.Duration
	isTerritory     bool
	isContinentLock bool
}{
	Meltdown:           {90 * time.Minute, true, true},
	UnstableMeltdown:   {45 * time.Minute, true, true},
	KoltyrMeltdown:     {45 * time.Minute, true, true},
	MaximumPressure:    {30 * time.Minute, false, false},
	SuddenDeath:        {15 * time.Minute, false, true},
	AerialAnomalies:    {30 * time.Minute, false, false},
	OutfitwarsPreMatch: {20 * time.Minute, false, false},
	OutfitwarsMatch:    {45 * time.Minute, false, false},
	HauntedBastion:     {15 * time.Minute, false, false},
}

func (ec MetagameEventCategory) Duration() time.Duration {
	return categoryData[ec].duration
}

func (ec MetagameEventCategory) IsTerritory() bool {
	return categoryData[ec].isTerritory
}

func (ec MetagameEventCategory) IsContinentLock() bool {
	return categoryData[ec].isContinentLock
}
