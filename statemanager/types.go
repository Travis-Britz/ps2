package statemanager

import (
	"time"

	"github.com/Travis-Britz/ps2"
	"github.com/Travis-Britz/ps2/ps2alerts"
)

type State struct {
	Pop    Pop
	Worlds []WorldState
}

type WorldState struct {
	ps2.World
	IsOffline bool
	Pop       Pop
	Zones     []ZoneState
}

type ZoneState struct {
	Zone
	Pop        Pop
	Event      Event
	MapState   MapState
	LastLocked time.Time
}

func (zs ZoneState) HasEvent() bool { return zs.Event.MetagameEventID != 0 }

type Event struct {
	MetagameEvent
	ps2alerts.Instance
	Result EventResult
}

type Pop [5]int

type MapState uint8

const (
	ContinentLocked MapState = iota
	UnstableWarpgates
	ContinentUnlocked
)

type EventResult struct {
	VS     float64
	NC     float64
	TR     float64
	Victor *ps2.FactionID
	Draw   bool
}

type MetagameEvent struct {
	ps2.MetagameEvent
	MetagameEventCategory
	ps2.Zone
	ps2.FactionID
}

type MetagameEventCategory struct {
	id              int
	name            string
	duration        time.Duration
	isTerritory     bool
	isContinentLock bool
}

type Zone struct {
	ps2.Zone
	IsPermanent bool // zones that are shown on the map at all times, even when they're locked
	IsPlayable  bool // zones that have some form of normal gameplay, including outfit wars matches
}
