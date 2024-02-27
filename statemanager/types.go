package statemanager

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Travis-Britz/ps2"
)

type GlobalState struct {
	Worlds []WorldState `json:"worlds"`
}

type WorldState struct {
	WorldID    ps2.WorldID `json:"world_id"`
	Name       string      `json:"name"`
	Population worldpop    `json:"population"`
	Zones      []ZoneState `json:"zones"`
}

func (original WorldState) Clone() (new WorldState) {
	new = original
	new.Zones = make([]ZoneState, 0, len(original.Zones))
	for i, zone := range original.Zones {
		new.Zones[i] = zone.Clone()
	}
	return new
}

type continentState int

const (
	locked continentState = iota
	unstable
	unlocked
)

func (state continentState) String() string {
	switch state {
	case locked:
		return "locked"
	case unlocked:
		return "unlocked"
	case unstable:
		return "unstable"
	default:
		return fmt.Sprintf("invalid_state(%d)", int(state))
	}
}

type worldpop struct {
	zonepop
	NSO     int `json:"nso"`
	Unknown int `json:"unknown"`
}
type zonepop struct {
	VS int `json:"vs"`
	NC int `json:"nc"`
	TR int `json:"tr"`
}

type score struct {
	VS float64 `json:"vs"`
	NC float64 `json:"nc"`
	TR float64 `json:"tr"`
}

type ZoneState struct {
	ZoneID         ps2.ZoneInstanceID `json:"zone_id"`
	OwningFaction  ps2.FactionID      `json:"owning_faction"`
	ZoneName       string             `json:"name"`
	ContinentState continentState     `json:"continent_state"`
	Population     zonepop            `json:"population"`
	LastLock       *time.Time         `json:"last_lock"`
	LastUnlock     *time.Time         `json:"last_unlock"`
	Event          *EventState        `json:"event"`
}

func (original ZoneState) Clone() (new ZoneState) {
	new = original
	if original.Event != nil {
		e := *original.Event
		e = e.Clone()
		new.Event = &e
	}
	if original.LastLock != nil {
		l := *original.LastLock
		new.LastLock = &l
	}
	if original.LastUnlock != nil {
		l := *original.LastUnlock
		new.LastUnlock = &l
	}
	return new
}

type EventState struct {
	MetagameEventID  ps2.MetagameEventID `json:"metagame_event_id"`
	InstanceID       ps2.InstanceID      `json:"instance_id"`
	EventName        string              `json:"name"`
	EventDescription string              `json:"description"`
	EventDuration    inSeconds           `json:"duration"`
	IsContinentLock  bool                `json:"is_continent_lock"`
	IsTerritory      bool                `json:"is_territory"`
	StartingFaction  ps2.FactionID       `json:"starting_faction"`
	Score            score               `json:"score"`
	EventURL         string              `json:"event_url"`
	Victor           *ps2.FactionID      `json:"victor"`
	Started          time.Time           `json:"started"`
	Ended            *time.Time          `json:"ended"`
}

func (original EventState) Clone() (new EventState) {
	new = original
	if original.Victor != nil {
		v := *original.Victor
		new.Victor = &v
	}
	if original.Ended != nil {
		e := *original.Ended
		new.Ended = &e
	}
	return new
}

// inSeconds is a time.Duration that marshals itself to seconds in JSON
type inSeconds struct {
	time.Duration
}

func (d inSeconds) MarshalJSON() ([]byte, error) {
	s := d.Seconds()
	return json.Marshal(int64(s))
}

func (original GlobalState) Clone() (new GlobalState) {
	new.Worlds = make([]WorldState, 0, len(original.Worlds))
	for i, state := range original.Worlds {
		new.Worlds[i] = state.Clone()
	}
	return new
}
