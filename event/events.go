package event

import (
	"bytes"
	"encoding/binary"
	"time"

	"github.com/Travis-Britz/ps2"
)

type Typer interface {
	Type() ps2.Event
}

// UniqueKey is used to uniquely identify an event across event types.
//
// This is needed because the planetside event stream can send duplicated events.
type UniqueKey [25]byte

func makeKey(t time.Time, e ps2.Event, id1 int64, id2 int64) UniqueKey {
	var k UniqueKey
	binary.PutVarint(k[0:8], t.Unix())
	k[8] = byte(e)
	binary.PutVarint(k[9:17], id1)
	binary.PutVarint(k[17:25], id2)
	return k
}

type UniqueKeyer interface {
	Key() UniqueKey
}

type Timestamper interface {
	Time() time.Time
}

// Raw is used to parse the raw payload section of an incoming websocket message.
type Raw struct {
	AchievementId          ps2.AchievementID        `json:"achievement_id,string"`
	BattleRank             uint8                    `json:"battle_rank,string"`
	AttackerFireModeId     ps2.FireModeID           `json:"attacker_fire_mode_id,string"`
	CharacterLoadoutId     ps2.LoadoutID            `json:"character_loadout_id,string"`
	IsCritical             stringNumericBool        `json:"is_critical,string"`
	IsHeadshot             stringNumericBool        `json:"is_headshot,string"`
	Amount                 float64                  `json:"amount,string"`
	ExperienceId           ps2.ExperienceID         `json:"experience_id,string"`
	LoadoutId              ps2.LoadoutID            `json:"loadout_id,string"`
	OtherId                ps2.EntityID             `json:"other_id,string"`
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
	EventType              ps2.Event                `json:"event_type"` // used by ContinentLock?
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
	FishId                 ps2.FishID               `json:"fish_id,string"`
}

// stringNumericBool is a bool value represented as "0" or "1" with json.
type stringNumericBool bool

type Number interface {
	int64 | float64
}

func (b *stringNumericBool) UnmarshalJSON(data []byte) error {
	data = bytes.Trim(data, "\"")
	if bytes.Equal(data, []byte("1")) {
		*b = true
	}
	return nil
}

var handlers = map[ps2.Event]func(Raw) Typer{
	ps2.Unknown: func(r Raw) Typer {
		return nil
	},
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
			IsCritical:          bool(r.IsCritical),
			IsHeadshot:          bool(r.IsHeadshot),
			Timestamp:           time.Unix(r.Timestamp, 0).UTC(),
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
	ps2.FishScan: func(r Raw) Typer {
		return FishScan{
			CharacterID: r.CharacterId,
			FishID:      r.FishId,
			LoadoutID:   r.LoadoutId,
			TeamID:      r.TeamId,
			Timestamp:   time.Unix(r.Timestamp, 0).UTC(),
			WorldID:     r.WorldId,
			ZoneID:      r.ZoneId,
		}
	},
}

func (r Raw) Event() Typer {
	h := handlers[r.EventName]
	if h == nil {
		panic("nil handler returned for type " + r.EventName.String())
	}
	return h(r)
}

type ContinentLock struct {
	Timestamp         time.Time
	WorldID           ps2.WorldID
	ZoneID            ps2.ZoneInstanceID
	TriggeringFaction ps2.FactionID // this might be the alert
	PreviousFaction   ps2.FactionID

	PopulationVS    int // seems to be population percentage at the time of lock
	PopulationNC    int
	PopulationTR    int
	MetagameEventID ps2.MetagameEventID // I have not seen any metagame event IDs that were not 0
}

func (ContinentLock) Type() ps2.Event   { return ps2.ContinentLock }
func (e ContinentLock) Time() time.Time { return e.Timestamp }
func (e ContinentLock) Key() UniqueKey {
	return makeKey(e.Timestamp, e.Type(), int64(e.WorldID), int64(e.ZoneID))
}

type PlayerLogin struct {
	CharacterID ps2.CharacterID
	Timestamp   time.Time
	WorldID     ps2.WorldID
}

func (e PlayerLogin) Time() time.Time { return e.Timestamp }
func (e PlayerLogin) Key() UniqueKey {
	return makeKey(e.Timestamp, e.Type(), int64(e.CharacterID), 0)
}

func (PlayerLogin) Type() ps2.Event { return ps2.PlayerLogin }

type PlayerLogout struct {
	CharacterID ps2.CharacterID
	Timestamp   time.Time
	WorldID     ps2.WorldID
}

func (PlayerLogout) Type() ps2.Event   { return ps2.PlayerLogout }
func (e PlayerLogout) Time() time.Time { return e.Timestamp }
func (e PlayerLogout) Key() UniqueKey {
	return makeKey(e.Timestamp, e.Type(), int64(e.CharacterID), 0)
}

type GainExperience struct {
	Amount       float64
	CharacterID  ps2.CharacterID
	ExperienceID ps2.ExperienceID
	LoadoutID    ps2.LoadoutID
	OtherID      ps2.EntityID
	Timestamp    time.Time
	WorldID      ps2.WorldID
	ZoneID       ps2.ZoneInstanceID
	TeamID       ps2.FactionID
}

func (GainExperience) Type() ps2.Event   { return ps2.GainExperience }
func (e GainExperience) Time() time.Time { return e.Timestamp }
func (e GainExperience) Key() UniqueKey {
	// devs have claimed a player can't have the same experience ID twice in the same second,
	// unless there are weird edge cases that haven't been considered
	return makeKey(e.Timestamp, e.Type(), int64(e.CharacterID), int64(e.ExperienceID))
}

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

func (VehicleDestroy) Type() ps2.Event   { return ps2.VehicleDestroy }
func (e VehicleDestroy) Time() time.Time { return e.Timestamp }
func (e VehicleDestroy) Key() UniqueKey {
	// an attacker could destroy two vehicles in the same second,
	// so we make the event unique on the character that lost the vehicle.
	// I don't think a player can own two vehicles at the same time?
	// I'm not sure who the owner ID is for abandoned vehicles.
	// TODO: get ingame and check who owns a destroyed abandoned vehicle.
	return makeKey(e.Timestamp, e.Type(), int64(e.CharacterID), int64(e.VehicleID))
}

type Death struct {
	AttackerCharacterID ps2.CharacterID
	AttackerFireModeID  ps2.FireModeID // AttackerFireModeID may be 0 in rare cases when AttackerCharacterID != 0
	AttackerLoadoutID   ps2.LoadoutID  // AttackerLoadoutID may be 0 in rare cases when AttackerCharacterID != 0
	AttackerVehicleID   ps2.VehicleID
	AttackerWeaponID    ps2.ItemID
	AttackerTeamID      ps2.FactionID // AttackerTeamID may be 0 in rare cases when AttackerCharacterID != 0
	CharacterID         ps2.CharacterID
	CharacterLoadoutID  ps2.LoadoutID
	TeamID              ps2.FactionID
	IsCritical          bool
	IsHeadshot          bool
	Timestamp           time.Time
	WorldID             ps2.WorldID
	ZoneID              ps2.ZoneInstanceID
}

func (e Death) IsSuicide() bool  { return e.AttackerCharacterID == e.CharacterID }
func (e Death) IsRoadkill() bool { return e.AttackerVehicleID != 0 && e.AttackerWeaponID == 0 }

// func (e Death) IsNaturalCauses() bool { return e.AttackerCharacterID == 0 }

// IsMagic is a rare case where the attacker ID is known but the method is not.
// I suspect these are things like dying to fall damage after being damaged by someone,
// but I haven't done testing.
// Some (all?) of these magic deaths will be missing attacker team and attacker loadout,
// which is annoying.
//
//	func (e Death) IsMagic() bool {
//		return e.AttackerCharacterID != 0 && e.AttackerVehicleID == 0 && e.AttackerWeaponID == 0
//	}
func (Death) Type() ps2.Event   { return ps2.Death }
func (e Death) Time() time.Time { return e.Timestamp }
func (e Death) Key() UniqueKey  { return makeKey(e.Timestamp, e.Type(), int64(e.CharacterID), 0) }

// AchievementEarned represents weapon medals or service ribbons.
type AchievementEarned struct {
	CharacterID   ps2.CharacterID
	Timestamp     time.Time
	WorldID       ps2.WorldID
	AchievementID ps2.AchievementID
	ZoneID        ps2.ZoneInstanceID
}

func (AchievementEarned) Type() ps2.Event   { return ps2.AchievementEarned }
func (e AchievementEarned) Time() time.Time { return e.Timestamp }
func (e AchievementEarned) Key() UniqueKey {
	return makeKey(e.Timestamp, e.Type(), int64(e.CharacterID), int64(e.AchievementID))
}

type BattleRankUp struct {
	CharacterID ps2.CharacterID
	BattleRank  uint8
	Timestamp   time.Time
	WorldID     ps2.WorldID
	ZoneID      ps2.ZoneInstanceID
}

func (BattleRankUp) Type() ps2.Event   { return ps2.BattleRankUp }
func (e BattleRankUp) Time() time.Time { return e.Timestamp }
func (e BattleRankUp) Key() UniqueKey {
	return makeKey(e.Timestamp, e.Type(), int64(e.CharacterID), int64(e.BattleRank))
}

type ItemAdded struct {
	CharacterID ps2.CharacterID
	Context     string
	ItemCount   int
	ItemID      ps2.ItemID
	Timestamp   time.Time
	WorldID     ps2.WorldID
	ZoneID      ps2.ZoneInstanceID
}

func (ItemAdded) Type() ps2.Event   { return ps2.ItemAdded }
func (e ItemAdded) Time() time.Time { return e.Timestamp }
func (e ItemAdded) Key() UniqueKey {
	return makeKey(e.Timestamp, e.Type(), int64(e.CharacterID), int64(e.ItemID))
}

type MetagameEvent struct {
	ExperienceBonus        float64
	FactionNC              float64 // for event ended, value is territory control (continent lock), kills (sudden death), and likely the event scores for other types like aerial anomalies
	FactionTR              float64
	FactionVS              float64
	InstanceID             ps2.InstanceID
	MetagameEventID        ps2.MetagameEventID
	MetagameEventState     ps2.MetagameEventStateID
	MetagameEventStateName string
	Timestamp              time.Time
	WorldID                ps2.WorldID
	ZoneID                 ps2.ZoneInstanceID
}

func (me MetagameEvent) EventInstanceID() ps2.MetagameEventInstanceID {
	return ps2.MetagameEventInstanceID{WorldID: me.WorldID, InstanceID: me.InstanceID}
}

func (MetagameEvent) Type() ps2.Event   { return ps2.Metagame }
func (e MetagameEvent) Time() time.Time { return e.Timestamp }
func (e MetagameEvent) Key() UniqueKey {
	return makeKey(e.Timestamp, e.Type(), int64(e.WorldID), int64(e.InstanceID))
}

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

func (e FacilityControl) Time() time.Time { return e.Timestamp }
func (e FacilityControl) Key() UniqueKey {
	return makeKey(e.Timestamp, e.Type(), int64(e.ZoneID), int64(e.FacilityID))
}

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

func (PlayerFacilityCapture) Type() ps2.Event   { return ps2.PlayerFacilityCapture }
func (e PlayerFacilityCapture) Time() time.Time { return e.Timestamp }
func (e PlayerFacilityCapture) Key() UniqueKey {
	return makeKey(e.Timestamp, e.Type(), int64(e.CharacterID), int64(e.FacilityID))
}

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

func (PlayerFacilityDefend) Type() ps2.Event   { return ps2.PlayerFacilityDefend }
func (e PlayerFacilityDefend) Time() time.Time { return e.Timestamp }
func (e PlayerFacilityDefend) Key() UniqueKey {
	return makeKey(e.Timestamp, e.Type(), int64(e.CharacterID), int64(e.FacilityID))
}

type SkillAdded struct {
	CharacterID ps2.CharacterID
	SkillID     ps2.SkillID
	Timestamp   time.Time
	WorldID     ps2.WorldID
	ZoneID      ps2.ZoneInstanceID
}

func (SkillAdded) Type() ps2.Event   { return ps2.SkillAdded }
func (e SkillAdded) Time() time.Time { return e.Timestamp }
func (e SkillAdded) Key() UniqueKey {
	return makeKey(e.Timestamp, e.Type(), int64(e.CharacterID), int64(e.SkillID))
}

type FishScan struct {
	CharacterID ps2.CharacterID
	FishID      ps2.FishID
	LoadoutID   ps2.LoadoutID
	TeamID      ps2.FactionID
	Timestamp   time.Time
	WorldID     ps2.WorldID
	ZoneID      ps2.ZoneInstanceID
}

func (FishScan) Type() ps2.Event   { return ps2.FishScan }
func (e FishScan) Time() time.Time { return e.Timestamp }
func (e FishScan) Key() UniqueKey {
	return makeKey(e.Timestamp, e.Type(), int64(e.CharacterID), int64(e.FishID))
}
