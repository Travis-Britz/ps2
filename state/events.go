package state

import (
	"time"

	"github.com/Travis-Britz/ps2"
)

//todo: emit alert starts

//todo: emit alert ends

//todo: emit alert score updates (can this combine starts and ends?)

//todo: emit population totals

//todo: maybe emit territory control?

// todo: emit continent unlocks
type continentUnlock struct {
	ps2.WorldID
	ps2.ContinentID
}

// todo: emit outfit capture events. or not? there's no reason code that cares couldn't just attach a handler directly to the websocket.
type outfitFacilityCapture struct {
	outfit   ps2.OutfitID
	facility ps2.FacilityID
	zone     ps2.ZoneInstanceID
	world    ps2.WorldID
	time     time.Time
}

func emitContinentUnlock(m *Manager, cu continentUnlock) {
	// todo: re-implement
}
