package statemanager

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/Travis-Britz/ps2"
	"github.com/Travis-Britz/ps2/census"
)

type GlobalState struct {
	Worlds []WorldState `json:"worlds"`
}

func (state *GlobalState) trackWorld(world census.World) {
	if world.WorldID == 0 {
		// this condition should not be possible if everything else is working properly,
		// so if it happens we need the stack trace
		panic("attempted to track empty world")
	}
	for _, w := range state.Worlds {
		if w.WorldID == world.WorldID {
			slog.Debug("world is already being tracked", "world", w)
			return
		}
	}
	new := WorldState{
		WorldID: world.WorldID,
		Name:    world.Name.String(),
	}
	state.Worlds = append(state.Worlds, new)
}

func (state *GlobalState) trackZone(world census.World, id ps2.ZoneInstanceID, cont census.Zone) {

	for i, w := range state.Worlds {
		if w.WorldID == world.WorldID {
			state.Worlds[i].trackZone(id, cont)
			return
		}
	}
	state.trackWorld(world)
	state.trackZone(world, id, cont) // is there any version of input that can infinitely recurse? idk i'm too tired to think about it right now
}

func (state *GlobalState) setWorldPop(id ps2.WorldID, count popCounter) {
	for i, w := range state.Worlds {
		if w.WorldID == id {
			pop := worldpop{}
			pop.Unknown = count[None]
			pop.NC = count[NC]
			pop.VS = count[VS]
			pop.TR = count[TR]
			pop.NSO = count[NSO]
			state.Worlds[i].Population = pop
		}
	}
}

func (state *GlobalState) setZonePop(id uniqueZone, count popCounter) {
	for i, world := range state.Worlds {
		if world.WorldID == id.WorldID {
			for j, zone := range world.Zones {
				if zone.MapID == id.ZoneInstanceID {
					pop := zonepop{
						VS: count[VS],
						NC: count[NC],
						TR: count[TR],
					}
					state.Worlds[i].Zones[j].Population = pop
					return
				}
			}
		}
	}
}

// setEvent is only used to attach new events or set an event to nil.
// To edit an event, maintain a reference to the given pointer.
func (state *GlobalState) setEvent(id uniqueZone, event *EventState) {

	if id.WorldID == 0 || id.ZoneInstanceID.ZoneID() == 0 {
		// I need a stack trace if this condition happens
		panic("attempted to set event on an empty id")
	}
	for i, world := range state.Worlds {
		if world.WorldID == id.WorldID {
			for j, zone := range world.Zones {
				if zone.MapID == id.ZoneInstanceID {
					state.Worlds[i].Zones[j].Event = event
					return
				}
			}
		}
	}
}

func (state GlobalState) isTracking(id uniqueZone) bool {
	zone := state.getZone(id)
	if zone.MapID == 0 {
		return false
	}
	return true
}

func (state GlobalState) getZone(id uniqueZone) ZoneState {
	world := state.getWorld(id.WorldID)
	return world.getZone(id.ZoneInstanceID)
}
func (state GlobalState) listZones() map[ps2.WorldID][]ps2.ZoneInstanceID {
	result := make(map[ps2.WorldID][]ps2.ZoneInstanceID)

	for _, world := range state.Worlds {
		zones := make([]ps2.ZoneInstanceID, 0, len(world.Zones))
		for _, zone := range world.Zones {
			zones = append(zones, zone.MapID)
		}
		result[world.WorldID] = zones
	}
	return result
}
func (state GlobalState) getZoneptr(id uniqueZone) *ZoneState {
	for i, world := range state.Worlds {
		if world.WorldID == id.WorldID {
			for j, zone := range world.Zones {
				if zone.MapID == id.ZoneInstanceID {
					return &state.Worlds[i].Zones[j]
				}
			}
		}
	}
	return nil
}

func (state GlobalState) getEvent(id uniqueZone) *EventState {
	world := state.getWorld(id.WorldID)
	zone := world.getZone(id.ZoneInstanceID)
	return zone.Event
}

func (state GlobalState) getWorld(id ps2.WorldID) (ws WorldState) {
	for _, world := range state.Worlds {
		if id == world.WorldID {
			ws = world
			break
		}
	}
	return ws
}

func (state WorldState) getZone(id ps2.ZoneInstanceID) (zs ZoneState) {
	for _, zone := range state.Zones {
		if id == zone.MapID {
			zs = zone
			break
		}
	}
	return zs
}

type uniqueZone struct {
	ps2.WorldID
	ps2.ZoneInstanceID
}

type WorldState struct {
	WorldID    ps2.WorldID `json:"world_id"`
	Name       string      `json:"name"`
	Population worldpop    `json:"population"`
	Zones      []ZoneState `json:"zones"`
}

func (state *WorldState) trackZone(id ps2.ZoneInstanceID, zoneData census.Zone) {
	if zoneData.ZoneID == 0 {
		// if we ever reach this condition,
		// we need a stack trace
		panic("attempted to track empty zone")
	}
	for _, zone := range state.Zones {
		if zone.MapID == id {
			slog.Debug("zone is already being tracked", "zone_state", zone)
			return
		}
	}
	new := ZoneState{
		MapID:    id,
		ZoneID:   zoneData.ZoneID,
		ZoneName: zoneData.Name.String(),
	}
	state.Zones = append(state.Zones, new)
}

func (original WorldState) Clone() (new WorldState) {
	new = original
	new.Zones = make([]ZoneState, len(original.Zones))
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

func (state continentState) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%q", state.String())), nil
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

// score represents the score for a metagame event.
// territory alerts use percentages.
// sudden death uses kill counts.
// there are other types.
type score struct {
	VS float64 `json:"vs"`
	NC float64 `json:"nc"`
	TR float64 `json:"tr"`
}

type ZoneState struct {
	MapID          ps2.ZoneInstanceID `json:"census_map_id"`
	ZoneID         ps2.ZoneID         `json:"zone_id"`
	OwningFaction  ps2.FactionID      `json:"owning_faction"`
	ZoneName       string             `json:"name"`
	ContinentState continentState     `json:"continent_state"`
	Population     zonepop            `json:"population"`
	LastLock       *time.Time         `json:"last_lock"`
	LastUnlock     *time.Time         `json:"last_unlock"`
	Regions        []RegionState      `json:"region_control"`
	MapTimestamp   time.Time          `json:"map_timestamp"`
	Event          *EventState        `json:"event"`
}

type RegionState struct {
	Region  ps2.RegionID
	Faction ps2.FactionID
}

type MapState struct {
	Regions []RegionState
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

	new.Regions = make([]RegionState, len(original.Regions))
	copy(new.Regions, original.Regions)
	return new
}

type EventState struct {
	ID               ps2.MetagameEventInstanceID `json:"id"`
	MapID            ps2.ZoneInstanceID          `json:"-"` //todo: make a delete event function and remove this field
	MetagameEventID  ps2.MetagameEventID         `json:"metagame_event_id"`
	EventName        string                      `json:"name"`
	EventDescription string                      `json:"description"`
	EventDuration    time.Duration               `json:"duration"` // displayed in seconds
	IsContinentLock  bool                        `json:"is_continent_lock"`
	IsTerritory      bool                        `json:"is_territory"`
	StartingFaction  ps2.FactionID               `json:"starting_faction"` // 0 for event types that aren't started by a faction
	Score            score                       `json:"score"`
	EventURL         string                      `json:"event_url"` // url to a page displaying event information, such as a ps2alerts.com link
	Victor           ps2.FactionID               `json:"victor"`    // faction will be 0 when ended is nil
	Started          time.Time                   `json:"started"`
	Ended            *time.Time                  `json:"ended"`
}

func (event *EventState) MarshalJSON() ([]byte, error) {
	type shadowType EventState // prevent recursion
	shadowCopy := shadowType(*event)
	shadowCopy.EventDuration /= time.Second // this is the reason we need to change behavior for marshaling
	return json.Marshal(shadowCopy)
}

func (original EventState) Clone() (new EventState) {
	new = original
	if original.Ended != nil {
		e := *original.Ended
		new.Ended = &e
	}
	return new
}

// Clone creates a deep copy of original for passing state to another function
// that should not have permission to modify  the state.
// Clone should only be used for this purpose;
// external structures referencing EventState pointers will only reference the original.
func (original GlobalState) Clone() (new GlobalState) {
	new.Worlds = make([]WorldState, len(original.Worlds))
	for i, state := range original.Worlds {
		new.Worlds[i] = state.Clone()
	}
	return new
}
