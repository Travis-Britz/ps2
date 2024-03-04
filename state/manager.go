package state

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

type gameDataStore interface {
	GetContinent(ps2.ContinentID) census.Zone
	ListContinents() []census.Zone
	GetWorld(ps2.WorldID) census.World
	ListWorlds() []census.World
	GetEvent(ps2.MetagameEventID) census.MetagameEvent
	GetFacility(ps2.FacilityID) census.Facility
	GetPlayerFaction(ps2.CharacterID) ps2.FactionID
	SavePlayerFaction(ps2.CharacterID, ps2.FactionID)
}

func New(db gameDataStore, censusClient *census.Client) *Manager {
	factionLookups := make(chan ps2.CharacterID, 10)
	m := &Manager{
		gameData:     db,
		census:       censusClient,
		alerts:       make(map[ps2.MetagameEventInstanceID]*EventState),
		alertUpdates: make(chan ps2alerts.Alert),
		players: onlinePlayerStore{
			players:        make(map[ps2.CharacterID]onlinePlayerState),
			factionLookups: factionLookups,
			saver:          db,
		},
		censusPushEvents:        make(chan event.Typer, 5000),
		mapUpdates:              make(chan census.ZoneState, 10),
		facilityUpdates:         make(chan internalFacilityUpdate, 500),
		zoneLookups:             make(map[uniqueZone]time.Time),
		characterFactionResults: make(chan factionResult, 10),
		characterFactionLookups: factionLookups,
		queryQueue:              make(chan query),
	}

	// initialize state for all static zones on all worlds
	for _, world := range db.ListWorlds() {
		for _, cont := range db.ListContinents() {
			m.state.trackZone(world, ps2.ZoneInstanceID(cont.ContinentID), cont)
		}
	}

	return m
}

// Manager maintains knowledge of worlds, zones, events, and population.
// It starts workers to keep itself updated.
type Manager struct {
	mu                      sync.Mutex
	gameData                gameDataStore
	census                  *census.Client
	alerts                  map[ps2.MetagameEventInstanceID]*EventState
	state                   GlobalState
	players                 onlinePlayerStore
	alertUpdates            chan ps2alerts.Alert
	mapUpdates              chan census.ZoneState
	facilityUpdates         chan internalFacilityUpdate
	censusPushEvents        chan event.Typer
	zoneLookups             map[uniqueZone]time.Time // zoneLookups is a cache of queried zone IDs
	characterFactionResults chan factionResult
	characterFactionLookups chan ps2.CharacterID
	queryQueue              chan query    // queryQueue is a channel of external requests to access the Manager
	unavailable             chan struct{} // unavailable is closed when the manager shuts down
}

// AttachHandlers attaches the required handlers to client.
func (manager *Manager) AttachHandlers(client eventClient) {
	client.AddHandler(manager.handleLogin)
	client.AddHandler(manager.handleLogout)
	client.AddHandler(manager.handleContinentLock)
	client.AddHandler(manager.handleFacilityControl)
	client.AddHandler(manager.handleDeath)
	client.AddHandler(manager.handleVehicleDestroy)
	client.AddHandler(manager.handleMetagame)
	client.AddHandler(manager.handleGainExperience)
}

// Run starts the Manager,
// blocking until ctx is cancelled.
func (manager *Manager) Run(ctx context.Context) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	everyFifteenSeconds := time.NewTicker(15 * time.Second)
	defer everyFifteenSeconds.Stop()

	go func() {
		for {
			getMapData(ctx, manager, manager.mapUpdates)
			updateActiveEventInstances(ctx, manager.alertUpdates)
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Minute):
			}
		}
	}()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case character := <-manager.characterFactionLookups:
				faction := manager.gameData.GetPlayerFaction(character)
				manager.characterFactionResults <- factionResult{CharacterID: character, FactionID: faction}
			}
		}
	}()
	manager.unavailable = make(chan struct{})
	defer close(manager.unavailable)

	for {
		select {
		case <-ctx.Done():
			return
		case alertData := <-manager.alertUpdates:
			handlePS2AlertsResponse(manager, alertData)
		case mapData := <-manager.mapUpdates:
			handleMap(manager, mapData)
		case facilityControl := <-manager.facilityUpdates:
			handleFacilityUpdate(manager, facilityControl)
		case result := <-manager.characterFactionResults:
			manager.players.factionUpdate(result.CharacterID, result.FactionID)
		case e := <-manager.censusPushEvents:
			switch event := e.(type) {
			case event.ContinentLock:
				handleLock(manager, event)
			case event.PlayerLogout:
				handleLogout(manager, event)
			case event.PlayerLogin:
				handleLogin(manager, event)
			case event.MetagameEvent:
				checkZone(ctx, manager, uniqueZone{event.WorldID, event.ZoneID})
				// if the zone needs to be initialized,
				// then this won't immediately track the alert.
				// handleMetagame will spawn a goroutine to fill data from ps2alerts though, which could also fail.
				// if the census api fails, the alert might not be initialized until one of the polls to /active on ps2alerts,
				// assuming their site is functioning and it's a territory alert.
				handleMetagame(ctx, manager, event, manager.alertUpdates)
			case event.Death:
				handleDeath(manager, event)
			case event.VehicleDestroy:
				handleVehicleDestroy(manager, event)
			case event.GainExperience:
				handleGainExperience(manager, event)
			case event.FacilityControl:
				checkZone(ctx, manager, uniqueZone{event.WorldID, event.ZoneID})
				handleFacilityControl(manager, event) // when warpgates change, send to unlocks channel
			}
		case <-everyFifteenSeconds.C:
			countPlayers(manager)
			removeStaleEvents(manager)
		case query := <-manager.queryQueue:
			query.Ask(manager)
		}
	}
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
func (m *Manager) handleLogout(e event.PlayerLogout) {
	select {
	case m.censusPushEvents <- e:
	case <-m.unavailable:
		return
	}
}

type factionSaver interface {
	SavePlayerFaction(ps2.CharacterID, ps2.FactionID)
}

// onlinePlayerStore holds the last known state of all online players
type onlinePlayerStore struct {
	players        map[ps2.CharacterID]onlinePlayerState
	factionLookups chan<- ps2.CharacterID
	saver          factionSaver
}

func (store *onlinePlayerStore) receivedEvent(id ps2.CharacterID, world ps2.WorldID, zone ps2.ZoneInstanceID, team ps2.FactionID, loadout ps2.LoadoutID, timestamp time.Time) {
	if id == 0 {
		return
	}

	if world == 0 {
		slog.Warn(
			"unusual player event; world should never be 0",
			"world", world,
			"timestamp", timestamp.Unix(),
			"zone", zone,
			"character", id,
			"loadout", loadout,
			"team", team,
		)
		return
	}

	p, found := store.players[id]
	if timestamp.Before(p.lastSeen) {
		return
	}

	if p.homeFaction == 0 && loadout != 0 {
		p.homeFaction = ps2.LoadoutFaction(loadout)
	}

	if team != 0 {
		p.team = team
	}

	if !found && p.homeFaction == 0 {
		store.factionLookups <- id
	}

	if !p.saved && p.homeFaction != 0 {
		p.saved = true
		store.saver.SavePlayerFaction(id, p.homeFaction)
		// m.gameData.SavePlayerFaction(e.CharacterID, p2.homeFaction)
	}

	store.players[id] = p
}
func (store *onlinePlayerStore) factionUpdate(id ps2.CharacterID, faction ps2.FactionID) {
	if faction == 0 {
		return
	}
	if p, found := store.players[id]; found {
		p.homeFaction = faction
		store.players[id] = p
	}
}

type onlinePlayerState struct {
	homeFaction ps2.FactionID // homeFaction is 0 until an event containing a ps2.ProfileID is seen, then saved
	team        ps2.FactionID // team is the current faction as determined by incoming kill events
	world       ps2.WorldID
	zone        ps2.ZoneInstanceID
	lastSeen    time.Time // timestamp of last event mentioning this player
	saved       bool      // track whether faction has been saved to database this session
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
	if manager.state.isTracking(zone) {
		return
	}

	// if other checks passed, then send it to the census api.
	// active zones will be sent back on the mapData channel and be intitialized for tracking in the consumer of that channel.
	go func() {
		ctx, stop := context.WithTimeout(ctx, 30*time.Second)
		defer stop()
		zm, err := census.GetMap(ctx, manager.census, zone.WorldID, zone.ZoneInstanceID)
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

func handleMap(manager *Manager, mapData census.ZoneState) {
	id := uniqueZone{mapData.WorldID, mapData.ZoneInstanceID}
	trackZone(manager, id)

	zone := manager.state.getZoneptr(id)
	if zone == nil {
		slog.Debug("returned zone pointer was nil; zone should have been initialized already", "id", id, "manager", manager, "map_data", mapData)
		return
	}

	// check for a lock state change on the map
	if zone.LastLock != nil && !mapData.IsLocked() {
		emitContinentUnlock(manager, continentUnlock{mapData.WorldID, mapData.ZoneID()})
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
	if manager.state.isTracking(zone) {
		return
	}
	slog.Debug("creating state tracker during runtime; this should have happened during initialization", "world_id", zone.WorldID)
	w := manager.gameData.GetWorld(zone.WorldID)
	cont := manager.gameData.GetContinent(zone.ZoneID())
	if cont.ContinentID == 0 {
		cont.ContinentID = zone.ZoneID()
		cont.Name.Set(fmt.Sprintf("Zone-%s", zone.ZoneID()))
		cont.Description.Set("Data was unavailable")
	}
	if w.WorldID == 0 {
		w.WorldID = zone.WorldID
		w.Name.Set(fmt.Sprintf("World-%s", zone.WorldID))
		w.Description.Set("Data was unavailable")
	}
	manager.state.trackZone(w, zone.ZoneInstanceID, cont)
}

// handleFacilityControl specifically handles push events from the websocket connection.
// it mostly just grabs fields from the event and sends them to a different facility control channel.
func handleFacilityControl(m *Manager, e event.FacilityControl) {
	update := internalFacilityUpdate{
		Faction:   e.NewFactionID,
		World:     e.WorldID,
		Zone:      e.ZoneID,
		Facility:  e.FacilityID,
		Outfit:    e.OutfitID,
		Timestamp: e.Timestamp,
	}
	m.facilityUpdates <- update
}

type internalFacilityUpdate struct {
	Faction   ps2.FactionID
	World     ps2.WorldID
	Zone      ps2.ZoneInstanceID
	Facility  ps2.FacilityID
	Outfit    ps2.OutfitID
	Timestamp time.Time // timestamp is the time of last known value, not necessarily when the territory flipped
}

// handleFacilityUpdate handles the internal parsed event changes that come from different sources
func handleFacilityUpdate(manager *Manager, update internalFacilityUpdate) {
	zoneID := uniqueZone{WorldID: update.World, ZoneInstanceID: update.Zone}
	zone := manager.state.getZoneptr(zoneID)
	if zone == nil {
		// skip untracked zones
		// facility updates come in for zones like the tutorial all the time
		return
	}
	facilityData := manager.gameData.GetFacility(update.Facility)
	if facilityData.FacilityID == 0 {
		slog.Debug("no facility data found", "update", update)
		return
	}

	// facility change events for warpgates happen on continent lock, unlock, and rotation
	if ps2.Warpgate == facilityData.FacilityType {
		// if the last change was more than 5 minutes before the timestamp
		// then emit an unlock message
		//
		// if the zone was unlocked
		// if the last change was more than 5 minutes before the timestamp
		// then emit a warpgate rotation message
		if zone.ContinentState == locked {
			zone.ContinentState = unstable
			// i had a reason for checking if the map timestamp was older than 5 minutes but i forgot what it
			// if zone.MapTimestamp.Before(time.Now().Add(time.Minute * -5)) {
			// 	emitContinentUnlock(manager, continentUnlock{WorldID: update.world, ContinentID: update.zone.ZoneID()})
			// }
		}
	}
}

func handleMetagame(ctx context.Context, manager *Manager, e event.MetagameEvent, ch chan<- ps2alerts.Alert) {
	eventData := manager.gameData.GetEvent(e.MetagameEventID)
	switch e.MetagameEventState {
	case ps2.Started:
		eventData := manager.gameData.GetEvent(e.MetagameEventID)
		event := newEvent(e.EventInstanceID(), e.ZoneID, eventData.MetagameEventID, e.Timestamp, manager.gameData)
		manager.alerts[e.EventInstanceID()] = event
		zid := uniqueZone{
			WorldID:        e.WorldID,
			ZoneInstanceID: e.ZoneID,
		}
		manager.state.setEvent(zid, event)
	case ps2.Restarted:
	case ps2.Cancelled, ps2.Ended:
		event := manager.alerts[e.EventInstanceID()]
		if event == nil {
			return
		}
		//todo: emit event update
		event.Score.NC = e.FactionNC
		event.Score.VS = e.FactionVS
		event.Score.TR = e.FactionTR
		event.Ended = &e.Timestamp
	}
	if ps2.IsTerritoryAlert(eventData.MetagameEventID) {
		go func() {
			// give ps2alerts a chance to create the event
			time.Sleep(20 * time.Second)
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
func handleLock(manager *Manager, e event.ContinentLock) {
	id := uniqueZone{
		WorldID:        e.WorldID,
		ZoneInstanceID: e.ZoneID,
	}

	zone := manager.state.getZoneptr(id)
	if zone == nil {
		// continent lock events come in for tutorial zones all the time
		// just ignore any we aren't already tracking
		return
	}
	zone.ContinentState = locked
	zone.OwningFaction = e.TriggeringFaction
	if zone.Event != nil {
		zone.Event.Victor = e.TriggeringFaction
	}
}

func handleLogin(m *Manager, e event.PlayerLogin) {
	m.players.receivedEvent(
		e.CharacterID,
		e.WorldID,
		0,
		0,
		0,
		e.Timestamp,
	)

}
func handleLogout(m *Manager, e event.PlayerLogout) {
	delete(m.players.players, e.CharacterID)
}
func handleGainExperience(m *Manager, e event.GainExperience) {
	m.players.receivedEvent(
		e.CharacterID,
		e.WorldID,
		e.ZoneID,
		e.TeamID,
		e.LoadoutID,
		e.Timestamp,
	)
}
func handleVehicleDestroy(m *Manager, e event.VehicleDestroy) {
	m.players.receivedEvent(
		e.AttackerCharacterID,
		e.WorldID,
		e.ZoneID,
		e.AttackerTeamID,
		e.AttackerLoadoutID,
		e.Timestamp,
	)
}
func handleDeath(m *Manager, e event.Death) {
	m.players.receivedEvent(
		e.AttackerCharacterID,
		e.WorldID,
		e.ZoneID,
		e.AttackerTeamID,
		e.AttackerLoadoutID,
		e.Timestamp,
	)
	m.players.receivedEvent(
		e.CharacterID,
		e.WorldID,
		e.ZoneID,
		e.TeamID,
		e.CharacterLoadoutID,
		e.Timestamp,
	)
}

// popCounter maintains a faction population counter, where the index is a ps2.FactionID.
type popCounter [5]int

func countPlayers(m *Manager) {
	worldCount := make(map[ps2.WorldID]popCounter)
	zoneCount := make(map[uniqueZone]popCounter)

	for id, player := range m.players.players {

		// if we haven't seen any events for a player in more than X hours,
		// then we will assume that there is some kind of error in receiving events like logouts
		// and we'll exclude the player from the population counts.
		if time.Since(player.lastSeen) > 2*time.Hour {
			// if they were still online they'll just get added back to tracking the next time an event comes in
			delete(m.players.players, id)
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

	for _, ws := range m.state.Worlds {
		wid := ws.WorldID
		m.state.setWorldPop(wid, worldCount[ws.WorldID])

		for _, zs := range ws.Zones {
			id := uniqueZone{WorldID: wid, ZoneInstanceID: zs.MapID}
			m.state.setZonePop(id, zoneCount[id])
		}
	}

}
func removeStaleEvents(m *Manager) {
	for eventID, event := range m.alerts {
		deletionTime := event.Started.Add(event.EventDuration + 10*time.Minute)
		if time.Now().After(deletionTime) {
			zone := uniqueZone{WorldID: event.ID.WorldID, ZoneInstanceID: event.MapID}
			m.state.deleteEvent(zone)
			delete(m.alerts, eventID)
		}
	}
}

func handlePS2AlertsResponse(manager *Manager, ps2aInstance ps2alerts.Alert) {
	id := ps2aInstance.InstanceID
	event := manager.alerts[id]
	if event == nil {
		eventData := manager.gameData.GetEvent(ps2aInstance.CensusMetagameEventType)
		event = newEvent(id, ps2aInstance.Zone, eventData.MetagameEventID, ps2aInstance.TimeStarted, manager.gameData)
		manager.alerts[id] = event
		zid := uniqueZone{
			WorldID:        ps2aInstance.World,
			ZoneInstanceID: ps2aInstance.Zone,
		}
		manager.state.setEvent(zid, event)
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

func getMapData(ctx context.Context, m *Manager, results chan<- census.ZoneState) {
	worldZones := m.state.listZones()
	for world, zones := range worldZones {
		// removed concurrency:
		//go
		func(w ps2.WorldID, zones []ps2.ZoneInstanceID) {
			if len(zones) == 0 {
				return
			}
			ctx, stop := context.WithTimeout(ctx, 30*time.Second)
			defer stop()
			zm, err := census.GetMap(ctx, m.census, w, zones...)
			if err != nil {
				slog.Warn("failed getting map state from census", "error", err, "zones", zones, "world", w)
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

func updateActiveEventInstances(ctx context.Context, ch chan<- ps2alerts.Alert) {
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

// errGoneHome is returned when the manager isn't working anymore
var errGoneHome = errors.New("manager is not running")

// query adds a query to the Manager's queue.
// It returns errGoneHome if the manager isn't available.
func (m *Manager) query(q query) error {
	select {
	case m.queryQueue <- q:
		return nil
	case <-m.unavailable:
		return errGoneHome
	}
}

// managerQuery holds a queued function to perform against a Manager and a buffered channel for the result.
type managerQuery[T any] struct {
	queryFn func(*Manager) T
	result  chan T // result must be buffered or the responses may get dropped
}

func (question managerQuery[T]) Ask(manager *Manager) {
	select {
	case question.result <- question.queryFn(manager):
	default:
		slog.Debug("dropped manager query result; result channel should be buffered")
	}

}

type query interface {
	Ask(*Manager)
}

type factionResult struct {
	ps2.CharacterID
	ps2.FactionID
}

func newEvent(id ps2.MetagameEventInstanceID, zone ps2.ZoneInstanceID, eventID ps2.MetagameEventID, start time.Time, db gameDataStore) *EventState {
	eventData := db.GetEvent(eventID)
	// event := &EventState{
	// 	ID:               id,
	// 	MetagameEventID:  eventID,
	// 	EventName:        eventData.Name.String(),
	// 	EventDescription: eventData.Description.String(),
	// 	EventDuration:    eventData.Duration,
	// 	IsContinentLock:  ps2.IsContinentLock(eventID),
	// 	IsTerritory:      ps2.IsTerritoryAlert(eventID),
	// 	StartingFaction:  ps2.StartingFaction(eventID),
	// 	Started:          start,
	// }
	// return event

	return &EventState{
		ID:               id,
		MapID:            zone,
		MetagameEventID:  eventData.MetagameEventID,
		EventName:        eventData.Name.String(),
		EventDescription: eventData.Description.String(),
		EventDuration:    eventData.Duration,
		IsContinentLock:  ps2.IsContinentLock(eventData.MetagameEventID),
		IsTerritory:      ps2.IsTerritoryAlert(eventData.MetagameEventID),
		StartingFaction:  ps2.StartingFaction(eventData.MetagameEventID),
		EventURL:         fmt.Sprintf("https://ps2alerts.com/alert/%s", id),
		Started:          start,
	}
}
