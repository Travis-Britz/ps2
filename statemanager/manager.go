package statemanager

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"sync"
	"time"

	"github.com/Travis-Britz/ps2"
	"github.com/Travis-Britz/ps2/census"
	"github.com/Travis-Britz/ps2/event"
	"github.com/Travis-Britz/ps2/ps2alerts"
)

const (
	None = ps2.None
	VS   = ps2.VS
	NC   = ps2.NC
	TR   = ps2.TR
	NSO  = ps2.NSO
)

type eventClient interface {
	AddHandler(any)
}

type ps2db interface {
	GetContinent(ps2.ContinentID) census.Zone
	ListContinents() []census.Zone
	GetWorld(ps2.WorldID) census.World
	ListWorlds() []census.World
	GetEvent(ps2.MetagameEventID) census.MetagameEvent
	GetPlayerFaction(ps2.CharacterID) (ps2.FactionID, error)
}

func New(db ps2db, censusClient *census.Client) *Manager {
	m := &Manager{
		// worlds:               make(map[ps2.WorldID]*worldStatus),
		// activeMetagameEvents: make(map[ps2.MetagameEventInstanceID]eventInstance),
		newActiveMetagameEventsRenameThisAndDeleteTheOldOne: make(map[ps2.MetagameEventInstanceID]*EventState),
		ps2alerts:               make(chan ps2alerts.Instance),
		onlinePlayers:           make(map[ps2.CharacterID]trackedPlayer),
		censusPushEvents:        make(chan event.Typer, 1000),
		mapUpdates:              make(chan census.ZoneState, 10),
		zoneLookups:             make(map[uniqueZone]time.Time),
		characterFactionResults: make(chan characterFaction, 100),
		collections:             db,
		censusClient:            censusClient,
	}

	// initialize state for all static zones on all worlds
	for _, world := range db.ListWorlds() {
		for _, cont := range db.ListContinents() {
			m.newGlobalStateRenameThis.trackZone(world, ps2.ZoneInstanceID(cont.ContinentID), cont)
		}
	}

	return m
}

// Manager maintains knowledge of worlds, zones, events, and population.
// It starts workers to keep itself updated.
type Manager struct {
	mu           sync.Mutex
	collections  ps2db
	censusClient *census.Client
	// activeMetagameEvents                                map[ps2.MetagameEventInstanceID]eventInstance
	newActiveMetagameEventsRenameThisAndDeleteTheOldOne map[ps2.MetagameEventInstanceID]*EventState
	newGlobalStateRenameThis                            GlobalState
	onlinePlayers                                       map[ps2.CharacterID]trackedPlayer
	ps2alerts                                           chan ps2alerts.Instance
	mapUpdates                                          chan census.ZoneState
	censusPushEvents                                    chan event.Typer
	// eventUpdates                                        chan eventInstance
	eventUpdates            chan EventState
	continentUnlocks        chan continentUnlock
	zoneLookups             map[uniqueZone]time.Time // zoneLookups is a cache of queried zone IDs
	characterFactionResults chan characterFaction

	queries chan query

	// unavailable is closed when the manager shuts down
	unavailable chan struct{}
}

func (manager *Manager) Register(client eventClient) {
	client.AddHandler(manager.handleLogin)
	client.AddHandler(manager.handleLogout)
	client.AddHandler(manager.handleContinentLock)
	client.AddHandler(manager.handleFacilityControl)
	client.AddHandler(manager.handleDeath)
	client.AddHandler(manager.handleVehicleDestroy)
	client.AddHandler(manager.handleMetagame)
	client.AddHandler(manager.handleGainExperience)
}

func (manager *Manager) Run(ctx context.Context) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.ps2alerts = make(chan ps2alerts.Instance)

	cronTasks := time.NewTicker(15 * time.Second)
	defer cronTasks.Stop()

	go func() {
		for {
			getMapData(ctx, manager, manager.mapUpdates)
			updateActiveEventInstances(ctx, manager.ps2alerts)
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Minute):
			}
		}
	}()
	manager.queries = make(chan query)
	manager.unavailable = make(chan struct{})
	defer close(manager.unavailable)

	for {
		select {
		case <-ctx.Done():
			if manager.eventUpdates != nil {
				close(manager.eventUpdates)
			}
			if manager.continentUnlocks != nil {
				close(manager.continentUnlocks)
			}
			return
		case alertData := <-manager.ps2alerts:
			handleInstance(manager, alertData)
		case mapData := <-manager.mapUpdates:
			handleMap(manager, mapData)
		case result := <-manager.characterFactionResults:
			handleCharacterFactionResult(manager, result)
		case e := <-manager.censusPushEvents:
			switch event := e.(type) {
			case event.ContinentLock:
				handleLock(manager, event)
			case event.PlayerLogout:
				handleLogout(manager, event)
			case event.PlayerLogin:
				handleLogin(manager, event)
			case event.MetagameEvent:
				handleMetagame(ctx, manager, event, manager.ps2alerts)
			case event.Death:
				handleDeath(manager, event)
			case event.VehicleDestroy:
				handleVehicleDestroy(manager, event)
			case event.GainExperience:
				handleGainExperience(manager, event)
			case event.FacilityControl:
				checkZone(ctx, manager, uniqueZone{event.WorldID, event.ZoneID})
				// handleFacilityControl(manager, event) // when warpgates change, send to unlocks channel
			}
		case <-cronTasks.C:
			countPlayers(manager)
			removeStaleEvents(manager)
		case q := <-manager.queries:
			q.Do(manager)
		}
	}
}

type trackedPlayer struct {
	// homeFaction needs to be populated by looking up character IDs on the census api
	// it's used for world population tracking, where NSO don't have a team assigned.
	homeFaction ps2.FactionID

	// team is the current faction as determined by incoming kill events
	team     ps2.FactionID
	world    ps2.WorldID
	zone     ps2.ZoneInstanceID
	lastSeen time.Time
}

// popCounter maintains a faction population counter, where the index is a ps2.FactionID.
type popCounter [5]int

func (pc popCounter) Sum(factions ...ps2.FactionID) (sum int) {
	if len(factions) == 0 {
		factions = []ps2.FactionID{None, VS, NC, TR, NSO}
	}
	for _, f := range factions {
		sum += pc[f]
	}
	return sum
}

// checkZone checks whether a zone should start being actively tracked.
func checkZone(ctx context.Context, manager *Manager, zone uniqueZone) {
	// we can short-circuit any zones checked recently
	if t := manager.zoneLookups[zone]; time.Since(t) < time.Hour {
		return
	}
	manager.zoneLookups[zone] = time.Now()

	// we're not concerned with tracking non-playable zones like VR-Training
	if !ps2.IsPlayableZone(zone.ZoneID()) {
		return
	}

	// if the zone is being tracked we don't need to do anything
	if manager.newGlobalStateRenameThis.isTracking(zone) {
		return
	}

	// if other checks passed, then send it to the census api.
	// active zones will be sent back on the mapData channel and be intitialized for tracking in the consumer of that channel.
	go func() {
		ctx, stop := context.WithTimeout(ctx, 30*time.Second)
		defer stop()
		zm, err := census.GetMap(ctx, manager.censusClient, zone.WorldID, zone.ZoneInstanceID)
		if err != nil {
			slog.Error("zone map lookup failed", "error", err, "zone", zone)
			return
		}
		for _, z := range zm {
			select {
			case manager.mapUpdates <- z:
			case <-ctx.Done():
				return
			}
		}
	}()
}

type continentUnlock struct {
	ps2.WorldID
	ps2.ContinentID
}

func (m *Manager) emitContinentUnlock(cu continentUnlock) {
	select {
	case m.continentUnlocks <- cu:
	default:
	}
}

func handleMap(manager *Manager, mapData census.ZoneState) {
	id := uniqueZone{mapData.WorldID, mapData.ZoneInstanceID}
	trackZone(manager, id)

	zone := manager.newGlobalStateRenameThis.getZoneptr(id)
	if zone == nil {
		slog.Debug("returned zone pointer was nil; zone should have been initialized already", "id", id, "manager", manager, "map_data", mapData)
		return
	}

	// check for a lock state change on the map
	if zone.LastLock != nil && !mapData.IsLocked() {
		//todo: re-implement unlock events
		// 	manager.emitContinentUnlock(continentUnlock{mapData.WorldID, mapData.ZoneID()})
	}

	if mapData.IsUnstable() {
		zone.ContinentState = unstable
	} else if mapData.IsLocked() {
		zone.ContinentState = locked
	} else {
		zone.ContinentState = unlocked
	}
	//todo: re-implement map update timestamps
	zone.Regions = make([]RegionState, 0, len(mapData.Regions))
	for _, regionData := range mapData.Regions {
		zone.Regions = append(zone.Regions, RegionState{
			Region:  regionData.RegionID,
			Faction: regionData.FactionID,
		})
	}
	zone.MapTimestamp = mapData.Timestamp
}

// trackZone checks if a zone is being tracked and fills zone data if it's not.
func trackZone(manager *Manager, zone uniqueZone) {
	if !manager.newGlobalStateRenameThis.isTracking(zone) {
		slog.Debug("creating state tracker during runtime; this should have happened during initialization", "world_id", zone.WorldID)
		w := manager.collections.GetWorld(zone.WorldID)
		cont := manager.collections.GetContinent(zone.ZoneID())
		if cont.ContinentID == 0 {
			cont.ContinentID = zone.ZoneID()
		}
		if w.WorldID == 0 {
			w.WorldID = zone.WorldID
		}
		manager.newGlobalStateRenameThis.trackZone(w, zone.ZoneInstanceID, cont)
	}
}

func handleFacilityControl(m *Manager, e event.FacilityControl) {
	// w := m.worlds[e.WorldID]
	// zs, ok := w.zones[e.ZoneID]
	// if !ok {
	// 	zs.Zone = m.collections.continents.Get(e.ZoneID.ZoneID())
	// 	w.zones[e.ZoneID] = zs
	// }

	// if the zone is a warpgate
	// if the zone was locked
	// if the last change was more than 5 minutes before the timestamp
	// then emit an unlock message
	//
	// if the zone was unlocked
	// if the last change was more than 5 minutes before the timestamp
	// then emit a warpgate rotation message
}
func handleGainExperience(m *Manager, e event.GainExperience) {
	p := m.onlinePlayers[e.CharacterID]
	p.zone = e.ZoneID
	p.team = e.TeamID
	p.lastSeen = e.Timestamp
	m.onlinePlayers[e.CharacterID] = p
}
func handleVehicleDestroy(m *Manager, e event.VehicleDestroy) {
	if e.AttackerCharacterID != 0 {
		p1 := m.onlinePlayers[e.AttackerCharacterID]
		if e.Timestamp.After(p1.lastSeen) {
			p1.zone = e.ZoneID
			p1.team = e.TeamID
			p1.world = e.WorldID
			p1.lastSeen = e.Timestamp
			m.onlinePlayers[e.AttackerCharacterID] = p1
		}
	}

	// p2 := m.onlinePlayers[e.CharacterID]
	// if e.Timestamp.After(p2.lastSeen) {
	// 	p2.zone = e.ZoneID
	// 	p2.team = e.TeamID
	// 	p2.world = e.WorldID
	// 	p2.lastSeen = e.Timestamp
	// 	m.onlinePlayers[e.CharacterID] = p2
	// }
}
func handleDeath(m *Manager, e event.Death) {
	if e.AttackerCharacterID != 0 {
		p1 := m.onlinePlayers[e.AttackerCharacterID]
		if e.Timestamp.After(p1.lastSeen) {
			p1.homeFaction = ps2.LoadoutFaction(e.AttackerLoadoutID)
			p1.zone = e.ZoneID
			p1.team = e.TeamID
			p1.world = e.WorldID
			p1.lastSeen = e.Timestamp
			m.onlinePlayers[e.AttackerCharacterID] = p1
		}
	}

	p2 := m.onlinePlayers[e.CharacterID]
	if e.Timestamp.After(p2.lastSeen) {
		p2.homeFaction = ps2.LoadoutFaction(e.CharacterLoadoutID)
		p2.zone = e.ZoneID
		p2.team = e.TeamID
		p2.world = e.WorldID
		p2.lastSeen = e.Timestamp
		m.onlinePlayers[e.CharacterID] = p2
	}
}
func handleMetagame(ctx context.Context, m *Manager, e event.MetagameEvent, ch chan<- ps2alerts.Instance) {
	eventData := m.collections.GetEvent(e.MetagameEventID)

	switch e.MetagameEventState {
	case ps2.Started:
		//todo: did I used to have code here?
	case ps2.Restarted:
	case ps2.Cancelled, ps2.Ended:
		event := m.newActiveMetagameEventsRenameThisAndDeleteTheOldOne[e.EventInstanceID()]
		if event == nil {
			return
		}
		event.Ended = &e.Timestamp
	}
	if ps2.IsTerritoryAlert(eventData.MetagameEventID) {
		go func() {
			// give ps2alerts a chance to create the event
			time.Sleep(10 * time.Second)
			ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			i, err := ps2alerts.GetInstanceContext(ctx, e.EventInstanceID())
			if err != nil {
				slog.Debug("ps2alerts metagame event lookup failed", "error", err)
				return
			}
			select {
			case ch <- i:
			case <-ctx.Done():
				return
			}
		}()
	}
}
func handleLock(m *Manager, e event.ContinentLock) {

}

func handleLogin(m *Manager, e event.PlayerLogin) {
	p := m.onlinePlayers[e.CharacterID]
	p.world = e.WorldID
	p.lastSeen = e.Timestamp
	m.onlinePlayers[e.CharacterID] = p
}
func handleLogout(m *Manager, e event.PlayerLogout) {
	delete(m.onlinePlayers, e.CharacterID)
}
func (m *Manager) handleFacilityControl(e event.FacilityControl) {
	select {
	case m.censusPushEvents <- e:
	case <-m.unavailable:
		return
	}
}
func (m *Manager) handleGainExperience(e event.GainExperience) {
	select {
	case m.censusPushEvents <- e:
	case <-m.unavailable:
		return
	}
}
func (m *Manager) handleMetagame(e event.MetagameEvent) {
	select {
	case m.censusPushEvents <- e:
	case <-m.unavailable:
		return
	}
}
func (m *Manager) handleVehicleDestroy(e event.VehicleDestroy) {
	select {
	case m.censusPushEvents <- e:
	case <-m.unavailable:
		return
	}
}
func (m *Manager) handleDeath(e event.Death) {
	select {
	case m.censusPushEvents <- e:
	case <-m.unavailable:
		return
	}
}
func (m *Manager) handleContinentLock(e event.ContinentLock) {
	select {
	case m.censusPushEvents <- e:
	case <-m.unavailable:
		return
	}
}
func (m *Manager) handleLogin(e event.PlayerLogin) {
	select {
	case m.censusPushEvents <- e:
	case <-m.unavailable:
		return
	}
}

func countPlayers(m *Manager) {
	worldCount := make(map[ps2.WorldID]popCounter)
	zoneCount := make(map[uniqueZone]popCounter)

	for _, player := range m.onlinePlayers {

		// if we haven't seen any events for a player in more than X hours,
		// then we will assume that there is some kind of error in receiving events like logouts
		// and we'll exclude the player from the population counts.
		if time.Since(player.lastSeen) > 2*time.Hour {
			continue
		}
		wcount := worldCount[player.world]
		wcount[player.homeFaction]++
		worldCount[player.world] = wcount

		z := uniqueZone{player.world, player.zone}
		wcount = zoneCount[z]
		wcount[player.team]++
		zoneCount[z] = wcount
	}

	for _, ws := range m.newGlobalStateRenameThis.Worlds {
		wid := ws.WorldID
		m.newGlobalStateRenameThis.setWorldPop(wid, worldCount[ws.WorldID])

		for _, zs := range ws.Zones {
			id := uniqueZone{WorldID: wid, ZoneInstanceID: zs.MapID}
			m.newGlobalStateRenameThis.setZonePop(id, zoneCount[id])
		}
	}

}
func removeStaleEvents(m *Manager) {
	for id, e := range m.newActiveMetagameEventsRenameThisAndDeleteTheOldOne {
		if time.Now().After(e.Started.Add(e.EventDuration + 5*time.Minute)) {
			zid := uniqueZone{
				WorldID:        e.ID.WorldID,
				ZoneInstanceID: e.MapID,
			}
			m.newGlobalStateRenameThis.setEvent(zid, nil)
			delete(m.newActiveMetagameEventsRenameThisAndDeleteTheOldOne, id)
		}
	}
}

func (m *Manager) handleLogout(e event.PlayerLogout) {
	select {
	case m.censusPushEvents <- e:
	case <-m.unavailable:
		return
	}
}

func handleInstance(manager *Manager, ps2aInstance ps2alerts.Instance) {
	id := ps2aInstance.InstanceID
	event := manager.newActiveMetagameEventsRenameThisAndDeleteTheOldOne[id]
	if event == nil {
		eventData := manager.collections.GetEvent(ps2aInstance.CensusMetagameEventType)
		event = &EventState{
			ID:               id,
			MapID:            ps2aInstance.Zone,
			MetagameEventID:  eventData.MetagameEventID,
			EventName:        eventData.Name.String(),
			EventDescription: eventData.Description.String(),
			EventDuration:    eventData.Duration,
			IsContinentLock:  ps2.IsContinentLock(eventData.MetagameEventID),
			IsTerritory:      ps2.IsTerritoryAlert(eventData.MetagameEventID),
			StartingFaction:  ps2.StartingFaction(eventData.MetagameEventID),
			EventURL:         fmt.Sprintf("https://ps2alerts.com/alert/%s", id),
			Started:          ps2aInstance.TimeStarted,
		}
		manager.newActiveMetagameEventsRenameThisAndDeleteTheOldOne[id] = event
		zid := uniqueZone{
			WorldID:        ps2aInstance.World,
			ZoneInstanceID: ps2aInstance.Zone,
		}
		manager.newGlobalStateRenameThis.setEvent(zid, event)
	}

	event.Score = score{
		NC: float64(ps2aInstance.Result.Nc),
		TR: float64(ps2aInstance.Result.Tr),
		VS: float64(ps2aInstance.Result.Vs),
	}

	if ps2aInstance.Result.Victor != nil {
		event.Victor = *ps2aInstance.Result.Victor
	}
	event.Ended = ps2aInstance.TimeEnded

	// select {
	// case manager.eventUpdates <- event: // this is where I would broadcast that event data is updated and e.g. update discord messages
	// default:
	// }
}

func handleCharacterFactionResult(manager *Manager, result characterFaction) {
	player, found := manager.onlinePlayers[result.CharacterID]
	if !found {
		// there could exist a valid condition where a faction lookup succeeds after a character logs out,
		// but other cases would probably be a bug
		slog.Debug("handling faction result for character that's not being tracked", "character", result.CharacterID)
		return
	}
	player.homeFaction = result.FactionID
	manager.onlinePlayers[result.CharacterID] = player
}

func getMapData(ctx context.Context, m *Manager, results chan<- census.ZoneState) {
	worldZones := m.newGlobalStateRenameThis.listZones()
	for world, zones := range worldZones {
		// removed concurrency:
		//go
		func(w ps2.WorldID, zones []ps2.ZoneInstanceID) {
			if len(zones) == 0 {
				return
			}
			ctx, stop := context.WithTimeout(ctx, 30*time.Second)
			defer stop()
			zm, err := census.GetMap(ctx, m.censusClient, w, zones...)
			if err != nil {
				slog.Error("failed getting map state from census", "error", err, "zones", zones, "world", w)
				return
			}
			for _, z := range zm {
				results <- z
			}
		}(world, zones)
	}
}

// func updateInstance(ctx context.Context, i ps2alerts.InstanceID, ch chan<- ps2alerts.Instance) {
// 	instance, err := ps2alerts.GetInstanceContext(ctx, i)
// 	if err != nil {
// 		slog.Error("failed getting ps2alerts event instance", "error", err)
// 		return
// 	}
// 	select {
// 	case ch <- instance:
// 	case <-ctx.Done():
// 		return
// 	}
// }

func updateActiveEventInstances(ctx context.Context, ch chan<- ps2alerts.Instance) {
	events, err := ps2alerts.GetActiveContext(ctx)
	if err != nil {
		log.Printf("updateActiveEventInstances: %v", err)
		return
	}
	for _, i := range events {
		select {
		case ch <- i:
		case <-ctx.Done():
			return
		}
	}
}

var errGoneHome = errors.New("manager is not running")

func (m *Manager) query(q query) error {
	select {
	case m.queries <- q:
		return nil
	case <-m.unavailable:
		return errGoneHome
	}
}

// func (w *worldStatus) LockZone(z ps2.ZoneInstanceID, t time.Time) {
// 	// instanced zones like koltyr or outfit wars should be removed when they lock
// 	if z.IsInstanced() {
// 		delete(w.zones, z)
// 		return
// 	}
// 	zs := w.zones[z]
// 	zs.lockedAt = t
// 	zs.continentState = locked
// 	w.zones[z] = zs
// }

type managerQuery[T any] struct {
	query  func(*Manager) T
	result chan T // result must be buffered or the responses may get dropped
}

func (question managerQuery[T]) Do(manager *Manager) {
	select {
	case question.result <- question.query(manager):
	default:
	}

}

type query interface {
	Do(*Manager)
}

type characterFaction struct {
	ps2.CharacterID
	ps2.FactionID
}

func newEvent(id ps2.MetagameEventInstanceID, eventID ps2.MetagameEventID, start time.Time, db ps2db) *EventState {
	eventData := db.GetEvent(eventID)
	event := &EventState{
		ID:               id,
		MetagameEventID:  eventID,
		EventName:        eventData.Name.String(),
		EventDescription: eventData.Description.String(),
		EventDuration:    eventData.Duration,
		IsContinentLock:  ps2.IsContinentLock(eventID),
		IsTerritory:      ps2.IsTerritoryAlert(eventID),
		StartingFaction:  ps2.StartingFaction(eventID),
		Started:          start,
	}
	return event
}
