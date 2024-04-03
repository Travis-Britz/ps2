package state

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/Travis-Britz/ps2"
	"github.com/Travis-Britz/ps2/census"
	"github.com/Travis-Britz/ps2/event"
	"github.com/Travis-Britz/ps2/ps2alerts"
	"github.com/Travis-Britz/ps2/psmap"
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
	GetFacilityRegion(ps2.FacilityID) ps2.RegionID
	GetMap(id ps2.ContinentID) (psmap.Map, error)
}

func New(db gameDataStore, censusClient *census.Client) *Manager {
	factionLookups := make(chan ps2.CharacterID, 10)
	m := &Manager{
		logf:         func(string, ...any) {},
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
	mu                       sync.Mutex
	logf                     func(string, ...any)
	gameData                 gameDataStore
	census                   *census.Client
	alerts                   map[ps2.MetagameEventInstanceID]*EventState
	state                    GlobalState
	players                  onlinePlayerStore
	alertUpdates             chan ps2alerts.Alert
	mapUpdates               chan census.ZoneState
	censusPushEvents         chan event.Typer
	zoneLookups              map[uniqueZone]time.Time // zoneLookups is a cache of queried zone IDs
	characterFactionResults  chan factionResult
	characterFactionLookups  chan ps2.CharacterID
	queryQueue               chan query    // queryQueue is a channel of external requests to access the Manager
	unavailable              chan struct{} // unavailable is closed when the manager shuts down
	populationHandlers       []func(PopulationTotal)
	territoryChangeHandlers  []func(TerritoryChange)
	zoneStatusChangeHandlers []func(ZoneStatusChange)
	eventUpdateHandlers      []func(EventState)
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
	manager.unavailable = make(chan struct{})
	defer close(manager.unavailable)

	go getMapData(ctx, manager, manager.mapUpdates)
	go updateActiveEventInstances(ctx, manager.alertUpdates)
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

	for {
		select {
		case <-ctx.Done():
			return
		case alertData := <-manager.alertUpdates:
			handlePS2AlertsResponse(manager, alertData)
		case mapData := <-manager.mapUpdates:
			handleMap(manager, mapData)
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
				handleMetagame(ctx, manager, event)
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
		// slog.Warn(
		// 	"unusual player event; world should never be 0",
		// 	"world", world,
		// 	"timestamp", timestamp.Unix(),
		// 	"zone", zone,
		// 	"character", id,
		// 	"loadout", loadout,
		// 	"team", team,
		// )
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
	}
	p.world = world
	p.zone = zone
	p.lastSeen = timestamp
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
			// slog.Error("zone map lookup failed", "error", err, "zone", zone)
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
		return
	}
	for _, region := range mapData.Regions {
		zone.Regions[region.RegionID] = region.FactionID
	}
	zone.MapTimestamp = time.Now()
	mapp, err := manager.gameData.GetMap(id.ZoneID())
	if err != nil {
		return
	}
	summary, err := psmap.Summarize(mapp, zone.Regions)
	if err != nil {
		return
	}
	emitZoneStateChange(manager, id, summary.Status)
	zone.ContinentState = summary.Status
	zone.Cutoff = summary.Cutoff
	if zone.ContinentState != psmap.Locked {
		emitTerritoryChange(manager, id, zone.Regions, zone.Cutoff)
	}
}

// trackZone checks if a zone is being tracked and fills zone data if it's not.
func trackZone(manager *Manager, zone uniqueZone) {
	if manager.state.isTracking(zone) {
		return
	}
	// manager.logf("creating state tracker during runtime; this should have happened during initialization", "world_id", zone.WorldID)
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

// handleFacilityControl handles push events from the websocket connection.
func handleFacilityControl(manager *Manager, e event.FacilityControl) {
	// for now we don't care about facility defense events
	if e.NewFactionID == e.OldFactionID {
		return
	}
	zoneID := uniqueZone{WorldID: e.WorldID, ZoneInstanceID: e.ZoneID}
	zone := manager.state.getZoneptr(zoneID)
	if zone == nil {
		// skip untracked zones
		// facility updates come in for zones like the tutorial all the time
		return
	}
	regionID := manager.gameData.GetFacilityRegion(e.FacilityID)
	if regionID == 0 {
		return
	}
	zone.Regions[regionID] = e.NewFactionID
	mapp, err := manager.gameData.GetMap(zoneID.ZoneID())
	if err != nil {
		return
	}
	summary, err := psmap.Summarize(mapp, zone.Regions)
	if err != nil {
		return
	}

	// check for a state change
	if zone.ContinentState != summary.Status {
		emitZoneStateChange(manager, zoneID, summary.Status)

		// if the old state was locked then territories from the last owner won't emit facility control events
		if psmap.Locked == zone.ContinentState {
			// this check will emit two events because it triggers during warpgate flips,
			// but that shouldn't matter
			unflipped := map[ps2.RegionID]ps2.FactionID{}
			for r, f := range zone.Regions {
				if f == e.OldFactionID {
					unflipped[r] = f
				}
			}
			emitTerritoryChange(
				manager,
				zoneID,
				unflipped,
				summary.Cutoff,
			)
		}
	}

	zone.ContinentState = summary.Status
	zone.Cutoff = summary.Cutoff
	zone.MapTimestamp = e.Timestamp

	emitTerritoryChange(
		manager,
		zoneID,
		map[ps2.RegionID]ps2.FactionID{regionID: e.NewFactionID},
		summary.Cutoff,
	)

	event := zone.Event
	if event != nil {
		if event.IsTerritory && event.Ended == nil {
			event.Score.VS = float64(summary.Territory[VS])
			event.Score.NC = float64(summary.Territory[NC])
			event.Score.TR = float64(summary.Territory[TR])
			// emit territory percents
			emitEventUpdate(manager, (*event).Clone())
		}
	}
}

func handleMetagame(_ context.Context, manager *Manager, e event.MetagameEvent) {
	switch e.MetagameEventState {
	case ps2.Started:
		// if the zone has any existing events we need to remove them
		// e.g. sudden death started immediately after a meltdown tie
		for alertID, alertData := range manager.alerts {
			if alertID.WorldID == e.WorldID && alertData.MapID == e.ZoneID {
				// no need to set the zone's EventState to nil because we'll overwrite it in the next block
				delete(manager.alerts, alertID)
			}
		}

		eventData := manager.gameData.GetEvent(e.MetagameEventID)
		event := newEvent(e.EventInstanceID(), e.ZoneID, eventData.MetagameEventID, e.Timestamp, manager.gameData)
		manager.alerts[e.EventInstanceID()] = event
		zid := uniqueZone{
			WorldID:        e.WorldID,
			ZoneInstanceID: e.ZoneID,
		}
		manager.state.setEvent(zid, event)
		emitEventUpdate(manager, (*event).Clone())
	case ps2.Restarted:
	case ps2.Cancelled, ps2.Ended:
		// events can end much earlier than their duration in the case of server shutdown.
		// there are messages ingame that the server will be shutting down and the alert timer will change ingame.
		// there are no events emitted from the census push service.
		event := manager.alerts[e.EventInstanceID()]
		if event == nil {
			return
		}
		event.Ended = &e.Timestamp
		event.Timestamp = e.Timestamp
		nc := event.Score.NC
		vs := event.Score.VS
		tr := event.Score.TR

		if nc > vs && nc > tr {
			event.Victor = NC
		}
		if vs > nc && vs > tr {
			event.Victor = VS
		}
		if tr > nc && tr > vs {
			event.Victor = TR
		}
		emitEventUpdate(manager, (*event).Clone())
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
	zone.ContinentState = psmap.Locked
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
	emitPopulationSums(m)
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

	emitEventUpdate(manager, (*event).Clone())
}

func getMapData(ctx context.Context, m *Manager, results chan<- census.ZoneState) {
	worldZones := m.state.listZones()
	for world, zones := range worldZones {
		go func(w ps2.WorldID, zones []ps2.ZoneInstanceID) {
			if len(zones) == 0 {
				return
			}
			ctx, stop := context.WithTimeout(ctx, 30*time.Second)
			defer stop()
			zm, err := census.GetMap(ctx, m.census, w, zones...)
			if err != nil {
				// slog.Warn("failed getting map state from census", "error", err, "zones", zones, "world", w)
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
		manager.logf("dropped manager query result; result channel should be buffered")
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
		Timestamp:        time.Now(),
	}
}
