package ps2

import (
	"bytes"
	"fmt"
)

// Environment represents a game server production environment.
//
// Values are PC, Playstation 4 (US), and Playstation 4 (EU).
// The default is PC for newly initialized structs.
type Environment uint8

func (e Environment) String() string {
	switch e {
	case PC:
		return "ps2"
	case PS4US:
		return "ps2ps4us"
	case PS4EU:
		return "ps2ps4eu"
	default:
		return ""
	}
}

func (e Environment) GoString() string {
	switch e {
	case PC:
		return "ps2.PC"
	case PS4US:
		return "ps2.PS4US"
	case PS4EU:
		return "ps2.PS4EU"
	default:
		return fmt.Sprintf("ps2.Environment(%d)", int(e))
	}
}

type Event uint8

const (
	Unknown Event = iota
	ContinentLock
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
	FishScan
)

var events = map[Event]string{
	// Unknown:               "Unknown",
	ContinentLock:         "ContinentLock",
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
	FishScan:              "FishScan",
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
	return fmt.Errorf("event.UnmarshalJSON: invalid value %q for event", data)
}

func (e Event) MarshalJSON() ([]byte, error) {
	return []byte("\"" + e.String() + "\""), nil
}
