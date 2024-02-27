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
		worlds:                  make(map[ps2.WorldID]*worldStatus),
		activeMetagameEvents:    make(map[ps2.MetagameEventInstanceID]eventInstance),
		ps2alerts:               make(chan ps2alerts.Instance),
		onlinePlayers:           make(map[ps2.CharacterID]trackedPlayer),
		censusPushEvents:        make(chan event.Typer, 1000),
		mapUpdates:              make(chan census.ZoneState, 10),
		zoneLookups:             make(map[serverZone]time.Time),
		characterFactionResults: make(chan characterFaction, 100),
		collections:             db,
		censusClient:            censusClient,
	}
	for _, w := range db.ListWorlds() {
		m.worlds[w.WorldID] = newWorldManager(w, db.ListContinents())
	}
	return m
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

// serverZone uniqely identifies a game zone by its world and zone instance.
type serverZone struct {
	ps2.WorldID
	ps2.ZoneInstanceID
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

// Manager maintains knowledge of worlds, zones, events, and population.
// It starts workers to keep itself updated.
type Manager struct {
	mu                      sync.Mutex
	collections             ps2db
	censusClient            *census.Client
	worlds                  map[ps2.WorldID]*worldStatus
	activeMetagameEvents    map[ps2.MetagameEventInstanceID]eventInstance
	onlinePlayers           map[ps2.CharacterID]trackedPlayer
	ps2alerts               chan ps2alerts.Instance
	mapUpdates              chan census.ZoneState
	censusPushEvents        chan event.Typer
	eventUpdates            chan eventInstance
	continentUnlocks        chan continentUnlock
	zoneLookups             map[serverZone]time.Time // zoneLookups is a cache of queried zone IDs
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
			updateActiveEventInstances(ctx, manager.ps2alerts)
			getMapData(ctx, manager, manager.mapUpdates)
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
				checkZone(ctx, manager, serverZone{event.WorldID, event.ZoneID})
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

// checkZone checks whether a zone should start being actively tracked.
func checkZone(ctx context.Context, manager *Manager, zone serverZone) {

	// we can short-circuit any zones checked recently
	if t := manager.zoneLookups[zone]; time.Since(t) < time.Hour {
		return
	}
	manager.zoneLookups[zone] = time.Now()

	//
	if _, exists := manager.worlds[zone.WorldID]; !exists {
		slog.Warn("creating state tracker during runtime; this should have happened during initialization", "world_id", zone.WorldID)
		w := manager.collections.GetWorld(zone.WorldID)
		if w.WorldID == 0 {
			w.WorldID = zone.WorldID
		}
		manager.worlds[zone.WorldID] = newWorldManager(w, manager.collections.ListContinents())
	}
	// we can skip if the zone is already being tracked
	if _, tracking := manager.worlds[zone.WorldID].zones[zone.ZoneInstanceID]; tracking {
		return
	}

	zoneData := manager.collections.GetContinent(zone.ZoneID())
	if zoneData.ContinentID == 0 {
		zoneData.ContinentID = zone.ZoneID()
	}
	// we're not concerned with tracking non-playable zones like VR-Training
	if !ps2.IsPlayableZone(zoneData.ContinentID) {
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

// trackZone checks if a zone is being tracked and fills zone data if it's not.
func trackZone(m *Manager, z serverZone) {
	ws := m.worlds[z.WorldID]
	if zs, found := ws.zones[z.ZoneInstanceID]; !found {
		zoneData := m.collections.GetContinent(z.ZoneID())
		if zoneData.ContinentID == 0 {
			zoneData.ContinentID = z.ZoneInstanceID.ZoneID()
		}
		zs.Zone = zoneData
		ws.zones[z.ZoneInstanceID] = zs
	}
}

func handleMap(manager *Manager, mapData census.ZoneState) {
	trackZone(manager, serverZone{mapData.WorldID, mapData.ZoneInstanceID})
	ws := manager.worlds[mapData.WorldID]
	zs := ws.zones[mapData.ZoneInstanceID]
	if zs.continentState == locked && !mapData.IsLocked() {
		manager.emitContinentUnlock(continentUnlock{mapData.WorldID, mapData.ZoneID()})
	}
	if mapData.IsUnstable() {
		zs.continentState = unstable
	} else if mapData.IsLocked() {
		zs.continentState = locked
	} else {
		zs.continentState = unlocked
	}
	zs.mapTimestamp = mapData.Timestamp
	ws.zones[mapData.ZoneInstanceID] = zs
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
			p1.zone = e.ZoneID
			p1.team = e.TeamID
			p1.world = e.WorldID
			p1.lastSeen = e.Timestamp
			m.onlinePlayers[e.AttackerCharacterID] = p1
		}
	}

	p2 := m.onlinePlayers[e.CharacterID]
	if e.Timestamp.After(p2.lastSeen) {
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

	case ps2.Restarted:
	case ps2.Cancelled, ps2.Ended:
		ei := m.activeMetagameEvents[e.EventInstanceID()]
		ei.TimeEnded = &e.Timestamp
		m.activeMetagameEvents[e.EventInstanceID()] = ei
	}
	if eventData.Type == 8 {
		// if eventData.isTerritory {
		go func() {
			// give ps2alerts a chance to create the event
			time.Sleep(10 * time.Second)
			ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			i, err := ps2alerts.GetInstanceContext(ctx, ps2alerts.InstanceID(e.EventInstanceID()))
			if err != nil {
				slog.Error("ps2alerts metagame event lookup failed", "error", err)
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
	m.worlds[e.WorldID].LockZone(e.ZoneID, e.Timestamp)
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
	type zoneKey struct {
		ps2.WorldID
		ps2.ZoneInstanceID
	}

	worldCount := make(map[ps2.WorldID]popCounter)
	zoneCount := make(map[zoneKey]popCounter)

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

		z := zoneKey{player.world, player.zone}
		wcount = zoneCount[z]
		wcount[player.team]++
		zoneCount[z] = wcount
	}

	for wid, ws := range m.worlds {
		ws.pop = worldCount[wid]

		for zid, zs := range ws.zones {
			zs.pop = zoneCount[zoneKey{wid, zid}]
			ws.zones[zid] = zs
		}
	}

}
func removeStaleEvents(m *Manager) {
	for id, e := range m.activeMetagameEvents {
		if time.Now().After(e.TimeStarted.Add(e.Duration.Duration() + 5*time.Minute)) {
			delete(m.activeMetagameEvents, id)
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
	id := ps2.MetagameEventInstanceID(ps2aInstance.InstanceID)
	event := manager.activeMetagameEvents[id]
	if event.MetagameEventID == 0 {
		event.MetagameEvent = manager.collections.GetEvent(ps2aInstance.CensusMetagameEventType)
		event.World = manager.collections.GetWorld(ps2aInstance.World)
	}
	event.Instance = ps2aInstance
	manager.activeMetagameEvents[id] = event

	select {
	case manager.eventUpdates <- event:
	default:
	}
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
	for wid, ws := range m.worlds {
		var zones []ps2.ZoneInstanceID
		for zid := range ws.zones {
			zones = append(zones, zid)
		}
		if len(zones) == 0 {
			return
		}
		go func(w ps2.WorldID, zones []ps2.ZoneInstanceID) {
			ctx, stop := context.WithTimeout(ctx, 15*time.Second)
			defer stop()
			zm, err := census.GetMap(ctx, m.censusClient, w, zones...)
			if err != nil {
				slog.Error("failed getting map state from census", "error", err, "zones", zones, "world", w)
				return
			}
			for _, z := range zm {
				results <- z
			}
		}(wid, zones)
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

// func (m *Manager) getEvent(i ps2.MetagameEventInstanceID) (ei eventInstance, err error) {
// 	q := managerQuery[eventInstance]{
// 		query: func(m *Manager) (i eventInstance) {
// 			return m.activeMetagameEvents[ps2.MetagameEventInstanceID(i.InstanceID)]
// 		},
// 		result: make(chan eventInstance, 1),
// 	}

// 	if err := m.query(q); err != nil {
// 		return ei, err
// 	}

// 	r := <-q.result
// 	return r, nil
// }

type infoCardServerStatus struct {
	serverID   ps2.WorldID
	serverName string
	isLocked   bool
	pop        popCounter
	continents []struct {
		contID               ps2.ContinentID
		name                 string
		state                continentState
		pop                  popCounter
		alertID              ps2.MetagameEventID
		alertName            string
		alertDescription     string
		alertIsContinentLock bool
		alertEnd             time.Time
		lockedAt             time.Time
	}
}

func (m *Manager) getServerInfoCard(serverID ps2.WorldID) (infoCardServerStatus, error) {

	q := managerQuery[infoCardServerStatus]{
		query: func(m *Manager) (c infoCardServerStatus) {
			w := m.worlds[serverID]
			c.serverID = w.WorldID
			c.serverName = w.Name.String()
			c.isLocked = w.isLocked
			c.pop = w.pop
			zc := make(map[ps2.ContinentID]int)
			for _, zs := range w.zones {
				zc[zs.ContinentID]++
				var cont struct {
					contID               ps2.ContinentID
					name                 string
					state                continentState
					pop                  popCounter
					alertID              ps2.MetagameEventID
					alertName            string
					alertDescription     string
					alertIsContinentLock bool
					alertEnd             time.Time
					lockedAt             time.Time
				}
				cont.contID = zs.ContinentID
				if zc[zs.ContinentID] == 1 {
					cont.name = zs.Name.String()
				} else {
					cont.name = fmt.Sprintf("%s %d", zs.Name, zc[zs.ContinentID])
				}
				var ei eventInstance
				for _, e := range m.activeMetagameEvents {
					if e.WorldID == serverID && e.ContinentID == zs.ContinentID {
						ei = e
						break
					}
				}
				cont.alertID = ei.MetagameEventID
				cont.alertName = ei.MetagameEvent.Name.String()
				cont.alertDescription = ei.MetagameEvent.Description.String()
				cont.alertIsContinentLock = ps2.IsContinentLock(ei.CensusMetagameEventType)
				cont.pop = zs.pop
				cont.alertEnd = ei.TimeStarted.Add(ei.Duration.Duration())
				cont.lockedAt = zs.lockedAt
				cont.state = zs.continentState
				c.continents = append(c.continents, cont)
			}
			return c
		},
		result: make(chan infoCardServerStatus, 1),
	}

	if err := m.query(q); err != nil {
		return infoCardServerStatus{}, err
	}
	r := <-q.result
	return r, nil
}

type infoCardMetagameEvent struct {
	populationBracket string // high, low, etc.
	population        [5]int
	territory         [5]int
	url               string // event details url, e.g. ps2alerts.com
	imageURL          string
	victor            *ps2.FactionID
	started           time.Time
	ended             *time.Time
	duration          time.Duration
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
func newWorldManager(w census.World, continents []census.Zone) *worldStatus {
	ws := &worldStatus{
		World: w,
		zones: make(map[ps2.ZoneInstanceID]zoneStatus),
	}

	for _, z := range continents {
		ws.zones[ps2.ZoneInstanceID(z.ContinentID)] = zoneStatus{Zone: z}
	}
	return ws
}

type worldStatus struct {
	census.World
	isLocked bool
	zones    map[ps2.ZoneInstanceID]zoneStatus
	pop      popCounter
}

func (w *worldStatus) LockZone(z ps2.ZoneInstanceID, t time.Time) {
	// instanced zones like koltyr or outfit wars should be removed when they lock
	if z.IsInstanced() {
		delete(w.zones, z)
		return
	}
	zs := w.zones[z]
	zs.lockedAt = t
	zs.continentState = locked
	w.zones[z] = zs
}

type zoneStatus struct {
	census.Zone
	continentState continentState
	lockedAt       time.Time
	pop            popCounter
	mapTimestamp   time.Time
}
type eventInstance struct {
	ps2alerts.Instance
	metagameEventData
	census.World
}
type metagameEventData struct {
	census.MetagameEvent
	census.Zone
	ps2.FactionID
}

type managerQuery[T any] struct {
	query  func(*Manager) T
	result chan T
}
type queryResult[T any] struct {
	value T
	err   error
}

func (query managerQuery[T]) Do(manager *Manager) {
	query.result <- query.query(manager)
}

type query interface {
	Do(*Manager)
}

type zoneState struct {
	owner      ps2.FactionID
	lockedAt   time.Time
	state      continentState
	unlockedAt time.Time
}

func getWorldState(w ps2.WorldID) (zs []zoneState, err error) {
	return
}

type characterFaction struct {
	ps2.CharacterID
	ps2.FactionID
}
