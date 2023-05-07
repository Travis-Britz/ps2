package event

import (
	"bytes"
	"time"

	"github.com/Travis-Britz/ps2"
)

type Typer interface {
	Type() ps2.Event
}

type Raw struct {
	AchievementId          ps2.AchievementID        `json:"achievement_id,string"`
	BattleRank             uint8                    `json:"battle_rank,string"`
	AttackerFireModeId     ps2.FireModeID           `json:"attacker_fire_mode_id,string"`
	CharacterLoadoutId     ps2.LoadoutID            `json:"character_loadout_id,string"`
	IsCritical             stringNumericBool        `json:"is_critical,string"`
	IsHeadshot             stringNumericBool        `json:"is_headshot,string"`
	Amount                 int                      `json:"amount"`
	ExperienceId           ps2.ExperienceID         `json:"experience_id,string"`
	LoadoutId              ps2.LoadoutID            `json:"loadout_id,string"`
	OtherId                int64                    `json:"other_id,string"`
	Context                string                   `json:"context"`
	ItemCount              int                      `json:"item_count,string"`
	ItemId                 ps2.ItemID               `json:"item_id,string"`
	SkillId                ps2.SkillID              `json:"skill_id,string"`
	AttackerCharacterId    ps2.CharacterID          `json:"attacker_character_id,string"`
	AttackerLoadoutId      ps2.LoadoutID            `json:"attacker_loadout_id,string"`
	AttackerVehicleId      ps2.VehicleID            `json:"attacker_vehicle_id,string"`
	AttackerWeaponId       ps2.ItemID               `json:"attacker_weapon_id,string"`
	AttackerTeamId         ps2.FactionID            `json:"attacker_team_id,string"`
	CharacterId            ps2.CharacterID          `json:"character_id,string"`
	FactionId              ps2.FactionID            `json:"faction_id,string"`
	VehicleId              ps2.VehicleID            `json:"vehicle_id,string"`
	TriggeringFaction      ps2.FactionID            `json:"triggering_faction,string"`
	PreviousFaction        ps2.FactionID            `json:"previous_faction,string"`
	VsPopulation           int                      `json:"vs_population,string"`
	NcPopulation           int                      `json:"nc_population,string"`
	TrPopulation           int                      `json:"tr_population,string"`
	EventType              ps2.Event                `json:"event_type"`
	OldFactionId           ps2.FactionID            `json:"old_faction_id,string"`
	OutfitId               ps2.OutfitID             `json:"outfit_id,string"`
	NewFactionId           ps2.FactionID            `json:"new_faction_id,string"`
	FacilityId             ps2.FacilityID           `json:"facility_id,string"`
	DurationHeld           int64                    `json:"duration_held,string"`
	EventName              ps2.Event                `json:"event_name"` // GainExperience, Death, etc.
	Timestamp              int64                    `json:"timestamp,string"`
	WorldId                ps2.WorldID              `json:"world_id,string"` // Emerald, Cobalt, etc.
	ExperienceBonus        float64                  `json:"experience_bonus,string"`
	FactionNc              float64                  `json:"faction_nc,string"`
	FactionTr              float64                  `json:"faction_tr,string"`
	FactionVs              float64                  `json:"faction_vs,string"`
	MetagameEventId        ps2.MetagameEventID      `json:"metagame_event_id,string"`    // Maximum Pressure, Indar Enlightenment, Indar Superiority, etc.
	MetagameEventState     ps2.MetagameEventStateID `json:"metagame_event_state,string"` // started, restarted, cancelled, ended, xp bonus changed
	MetagameEventStateName string                   `json:"metagame_event_state_name"`
	TeamId                 ps2.FactionID            `json:"team_id,string"`
	ZoneId                 ps2.ZoneInstanceID       `json:"zone_id,string"`     // Indar, Hossin, VR Training (NC), etc.
	InstanceId             ps2.InstanceID           `json:"instance_id,string"` // used in alert identification
}

// stringNumericBool is a bool value represented as "0" or "1" with json.
type stringNumericBool bool

func (b *stringNumericBool) UnmarshalJSON(data []byte) error {
	data = bytes.Trim(data, "\"")
	if bytes.Equal(data, []byte("1")) {
		*b = true
	}
	return nil
}

var handlers = map[ps2.Event]func(Raw) Typer{
	ps2.PlayerLogin: func(r Raw) Typer {
		return PlayerLogin{
			CharacterID: r.CharacterId,
			Timestamp:   time.Unix(r.Timestamp, 0).UTC(),
			WorldID:     r.WorldId,
		}
	},
	ps2.PlayerLogout: func(r Raw) Typer {
		return PlayerLogout{
			CharacterID: r.CharacterId,
			Timestamp:   time.Unix(r.Timestamp, 0).UTC(),
			WorldID:     r.WorldId,
		}
	},
	ps2.GainExperience: func(r Raw) Typer {
		return GainExperience{
			Amount:       r.Amount,
			CharacterID:  r.CharacterId,
			ExperienceID: r.ExperienceId,
			LoadoutID:    r.LoadoutId,
			OtherID:      r.OtherId,
			Timestamp:    time.Unix(r.Timestamp, 0).UTC(),
			WorldID:      r.WorldId,
			ZoneID:       r.ZoneId,
			TeamID:       r.TeamId,
		}
	},
	ps2.VehicleDestroy: func(r Raw) Typer {
		return VehicleDestroy{
			AttackerCharacterID: r.AttackerCharacterId,
			AttackerLoadoutID:   r.AttackerLoadoutId,
			AttackerVehicleID:   r.AttackerVehicleId,
			AttackerWeaponID:    r.AttackerWeaponId,
			AttackerTeamID:      r.AttackerTeamId,
			CharacterID:         r.CharacterId,
			FacilityID:          r.FacilityId,
			FactionID:           r.FactionId,
			TeamID:              r.TeamId,
			Timestamp:           time.Unix(r.Timestamp, 0).UTC(),
			VehicleID:           r.VehicleId,
			WorldID:             r.WorldId,
			ZoneID:              r.ZoneId,
		}
	},
	ps2.Death: func(r Raw) Typer {
		return Death{
			AttackerCharacterID: r.AttackerCharacterId,
			AttackerFireModeID:  r.AttackerFireModeId,
			AttackerLoadoutID:   r.AttackerLoadoutId,
			AttackerVehicleID:   r.AttackerVehicleId,
			AttackerWeaponID:    r.AttackerWeaponId,
			AttackerTeamID:      r.AttackerTeamId,
			CharacterID:         r.CharacterId,
			CharacterLoadoutID:  r.CharacterLoadoutId,
			TeamID:              r.TeamId,
			IsCritical:          false,
			IsHeadshot:          bool(r.IsHeadshot),
			Timestamp:           time.Unix(r.Timestamp, 0).UTC(),
			VehicleID:           r.VehicleId,
			WorldID:             r.WorldId,
			ZoneID:              r.ZoneId,
		}
	},
	ps2.AchievementEarned: func(r Raw) Typer {
		return AchievementEarned{
			CharacterID:   r.CharacterId,
			Timestamp:     time.Unix(r.Timestamp, 0).UTC(),
			WorldID:       r.WorldId,
			AchievementID: r.AchievementId,
			ZoneID:        r.ZoneId,
		}
	},
	ps2.BattleRankUp: func(r Raw) Typer {
		return BattleRankUp{
			CharacterID: r.CharacterId,
			BattleRank:  r.BattleRank,
			Timestamp:   time.Unix(r.Timestamp, 0).UTC(),
			WorldID:     r.WorldId,
			ZoneID:      r.ZoneId,
		}
	},
	ps2.ItemAdded: func(r Raw) Typer {
		return ItemAdded{
			CharacterID: r.CharacterId,
			Context:     r.Context,
			ItemCount:   r.ItemCount,
			ItemID:      r.ItemId,
			Timestamp:   time.Unix(r.Timestamp, 0).UTC(),
			WorldID:     r.WorldId,
			ZoneID:      r.ZoneId,
		}
	},
	ps2.Metagame: func(r Raw) Typer {
		return MetagameEvent{
			ExperienceBonus:        r.ExperienceBonus,
			FactionNC:              r.FactionNc,
			FactionTR:              r.FactionTr,
			FactionVS:              r.FactionVs,
			MetagameEventID:        r.MetagameEventId,
			MetagameEventState:     r.MetagameEventState,
			MetagameEventStateName: r.MetagameEventStateName,
			Timestamp:              time.Unix(r.Timestamp, 0).UTC(),
			WorldID:                r.WorldId,
			ZoneID:                 r.ZoneId,
			InstanceID:             r.InstanceId,
		}
	},
	ps2.FacilityControl: func(r Raw) Typer {
		return FacilityControl{
			DurationHeld: time.Duration(r.DurationHeld) * time.Second,
			FacilityID:   r.FacilityId,
			NewFactionID: r.NewFactionId,
			OldFactionID: r.OldFactionId,
			OutfitID:     r.OutfitId,
			Timestamp:    time.Unix(r.Timestamp, 0).UTC(),
			WorldID:      r.WorldId,
			ZoneID:       r.ZoneId,
		}
	},
	ps2.PlayerFacilityCapture: func(r Raw) Typer {
		return PlayerFacilityCapture{
			CharacterID: r.CharacterId,
			FacilityID:  r.FacilityId,
			OutfitID:    r.OutfitId,
			Timestamp:   time.Unix(r.Timestamp, 0).UTC(),
			WorldID:     r.WorldId,
			ZoneID:      r.ZoneId,
		}
	},
	ps2.PlayerFacilityDefend: func(r Raw) Typer {
		return PlayerFacilityDefend{
			CharacterID: r.CharacterId,
			FacilityID:  r.FacilityId,
			OutfitID:    r.OutfitId,
			Timestamp:   time.Unix(r.Timestamp, 0).UTC(),
			WorldID:     r.WorldId,
			ZoneID:      r.ZoneId,
		}
	},
	ps2.SkillAdded: func(r Raw) Typer {
		return SkillAdded{
			CharacterID: r.CharacterId,
			SkillID:     r.SkillId,
			Timestamp:   time.Unix(r.Timestamp, 0).UTC(),
			WorldID:     r.WorldId,
			ZoneID:      r.ZoneId,
		}
	},
	ps2.ContinentLock: func(r Raw) Typer {
		return ContinentLock{
			Timestamp:         time.Unix(r.Timestamp, 0).UTC(),
			WorldID:           r.WorldId,
			ZoneID:            r.ZoneId,
			TriggeringFaction: r.TriggeringFaction,
			PreviousFaction:   r.PreviousFaction,
			PopulationVS:      r.VsPopulation,
			PopulationNC:      r.NcPopulation,
			PopulationTR:      r.TrPopulation,
			MetagameEventID:   r.MetagameEventId,
		}
	},
	ps2.ContinentUnlock: func(r Raw) Typer {
		return ContinentUnlock{
			Timestamp:         time.Unix(r.Timestamp, 0).UTC(),
			WorldID:           r.WorldId,
			ZoneID:            r.ZoneId,
			TriggeringFaction: r.TriggeringFaction,
			PreviousFaction:   r.PreviousFaction,
			PopulationVS:      r.VsPopulation,
			PopulationNC:      r.NcPopulation,
			PopulationTR:      r.TrPopulation,
			MetagameEventID:   r.MetagameEventId,
		}
	},
}

func (r Raw) Event() Typer {
	h := handlers[r.EventType]
	return h(r)
}

type ContinentLock struct {
	Timestamp         time.Time
	WorldID           ps2.WorldID
	ZoneID            ps2.ZoneInstanceID
	TriggeringFaction ps2.FactionID
	PreviousFaction   ps2.FactionID

	PopulationVS    int // seems to be territory control in the event stream (0,0,100)
	PopulationNC    int
	PopulationTR    int
	MetagameEventID ps2.MetagameEventID
}

func (ContinentLock) Type() ps2.Event { return ps2.ContinentLock }

type ContinentUnlock struct {
	Timestamp         time.Time
	WorldID           ps2.WorldID
	ZoneID            ps2.ZoneInstanceID
	TriggeringFaction ps2.FactionID
	PreviousFaction   ps2.FactionID
	PopulationVS      int
	PopulationNC      int
	PopulationTR      int
	MetagameEventID   ps2.MetagameEventID
}

func (ContinentUnlock) Type() ps2.Event { return ps2.ContinentUnlock }

type PlayerLogin struct {
	CharacterID ps2.CharacterID
	Timestamp   time.Time
	WorldID     ps2.WorldID
}

func (PlayerLogin) Type() ps2.Event { return ps2.PlayerLogin }

type PlayerLogout struct {
	CharacterID ps2.CharacterID
	Timestamp   time.Time
	WorldID     ps2.WorldID
}

func (PlayerLogout) Type() ps2.Event { return ps2.PlayerLogout }

type GainExperience struct {
	Amount       int
	CharacterID  ps2.CharacterID
	ExperienceID ps2.ExperienceID
	LoadoutID    ps2.LoadoutID
	OtherID      int64 // player id, vehicle id, etc.
	Timestamp    time.Time
	WorldID      ps2.WorldID
	ZoneID       ps2.ZoneInstanceID
	TeamID       ps2.FactionID
}

func (GainExperience) Type() ps2.Event { return ps2.GainExperience }

type VehicleDestroy struct {
	AttackerCharacterID ps2.CharacterID
	AttackerLoadoutID   ps2.LoadoutID
	AttackerVehicleID   ps2.VehicleID
	AttackerWeaponID    ps2.ItemID
	AttackerTeamID      ps2.FactionID
	CharacterID         ps2.CharacterID
	FacilityID          ps2.FacilityID // only populated for base turrets
	FactionID           ps2.FactionID
	TeamID              ps2.FactionID
	Timestamp           time.Time
	VehicleID           ps2.VehicleID
	WorldID             ps2.WorldID
	ZoneID              ps2.ZoneInstanceID
}

func (VehicleDestroy) Type() ps2.Event { return ps2.VehicleDestroy }

type Death struct {
	AttackerCharacterID ps2.CharacterID
	AttackerFireModeID  ps2.FireModeID
	AttackerLoadoutID   ps2.LoadoutID
	AttackerVehicleID   ps2.VehicleID
	AttackerWeaponID    ps2.ItemID
	AttackerTeamID      ps2.FactionID
	CharacterID         ps2.CharacterID
	CharacterLoadoutID  ps2.LoadoutID
	TeamID              ps2.FactionID
	IsCritical          bool
	IsHeadshot          bool
	Timestamp           time.Time
	VehicleID           ps2.VehicleID
	WorldID             ps2.WorldID
	ZoneID              ps2.ZoneInstanceID
}

func (e Death) IsSuicide() bool       { return e.AttackerCharacterID == e.CharacterID }
func (e Death) IsRoadkill() bool      { return e.AttackerVehicleID != 0 && e.AttackerWeaponID == 0 }
func (e Death) IsNaturalCauses() bool { return e.AttackerCharacterID == 0 }
func (Death) Type() ps2.Event         { return ps2.Death }

// AchievementEarned represents weapon medals or service ribbons.
type AchievementEarned struct {
	CharacterID   ps2.CharacterID
	Timestamp     time.Time
	WorldID       ps2.WorldID
	AchievementID ps2.AchievementID
	ZoneID        ps2.ZoneInstanceID
}

func (AchievementEarned) Type() ps2.Event { return ps2.AchievementEarned }

type BattleRankUp struct {
	CharacterID ps2.CharacterID
	BattleRank  uint8
	Timestamp   time.Time
	WorldID     ps2.WorldID
	ZoneID      ps2.ZoneInstanceID
}

func (BattleRankUp) Type() ps2.Event { return ps2.BattleRankUp }

type ItemAdded struct {
	CharacterID ps2.CharacterID
	Context     string
	ItemCount   int
	ItemID      ps2.ItemID
	Timestamp   time.Time
	WorldID     ps2.WorldID
	ZoneID      ps2.ZoneInstanceID
}

func (ItemAdded) Type() ps2.Event { return ps2.ItemAdded }

type MetagameEvent struct {
	ExperienceBonus        float64
	FactionNC              float64 // for event ended, value is territory control (continent lock), kills (sudden death), and likely the event scores for other types like aerial anomalies
	FactionTR              float64
	FactionVS              float64
	MetagameEventID        ps2.MetagameEventID
	MetagameEventStateName string
	MetagameEventState     ps2.MetagameEventStateID
	Timestamp              time.Time
	WorldID                ps2.WorldID
	ZoneID                 ps2.ZoneInstanceID
	InstanceID             ps2.InstanceID
}

func (me MetagameEvent) EventInstanceID() ps2.MetagameEventInstanceID {
	return ps2.MetagameEventInstanceID{WorldID: me.WorldID, InstanceID: me.InstanceID}
}

func (MetagameEvent) Type() ps2.Event { return ps2.Metagame }

type FacilityControl struct {
	DurationHeld time.Duration
	FacilityID   ps2.FacilityID
	NewFactionID ps2.FactionID
	OldFactionID ps2.FactionID
	OutfitID     ps2.OutfitID
	Timestamp    time.Time
	WorldID      ps2.WorldID
	ZoneID       ps2.ZoneInstanceID
}

func (FacilityControl) Type() ps2.Event { return ps2.FacilityControl }

type PlayerFacilityCapture struct {
	CharacterID ps2.CharacterID
	FacilityID  ps2.FacilityID

	// OutfitID appears to represent the outfit of the player receiving the event.
	// Some sources say it's supposed to be the outfit that owns the facility.
	OutfitID  ps2.OutfitID
	Timestamp time.Time
	WorldID   ps2.WorldID
	ZoneID    ps2.ZoneInstanceID
}

func (PlayerFacilityCapture) Type() ps2.Event { return ps2.PlayerFacilityCapture }

type PlayerFacilityDefend struct {
	CharacterID ps2.CharacterID
	FacilityID  ps2.FacilityID

	// OutfitID appears to represent the outfit of the player receiving the event.
	// Some sources say it's supposed to be the outfit that owns the facility.
	OutfitID  ps2.OutfitID
	Timestamp time.Time
	WorldID   ps2.WorldID
	ZoneID    ps2.ZoneInstanceID
}

func (PlayerFacilityDefend) Type() ps2.Event { return ps2.PlayerFacilityDefend }

type SkillAdded struct {
	CharacterID ps2.CharacterID
	SkillID     ps2.SkillID
	Timestamp   time.Time
	WorldID     ps2.WorldID
	ZoneID      ps2.ZoneInstanceID
}

func (SkillAdded) Type() ps2.Event { return ps2.SkillAdded }
