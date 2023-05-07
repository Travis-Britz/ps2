package statemanager

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
)

type eventClient interface {
	AddHandler(any)
}

type ps2db interface {
	GetZoneByID(ps2.ZoneID) (planetside.Zone, error)
	ListContinents() []planetside.Zone
	ListWorlds() []ps2.World
}

func New(db ps2db, censusClient *census.Client) *Manager {
	m := &Manager{
		worlds:       make(map[ps2.WorldID]*worldStatus),
		events:       make(map[ps2.MetagameEventInstanceID]eventInstance),
		ps2alerts:    make(chan ps2alerts.Instance),
		players:      make(map[ps2.CharacterID]trackedPlayer),
		censusEvents: make(chan event.Typer, 1000),
		mapData:      make(chan census.ZoneMap, 5),
		zoneLookups:  make(map[serverZone]time.Time),
		collections:  db,
		censusClient: censusClient,
	}
	for _, w := range db.ListWorlds() {
		m.worlds[w.WorldID] = newWorldManager(w, db.ListContinents())
	}
	return m
}

type trackedPlayer struct {
	team     ps2.FactionID
	world    ps2.WorldID
	zone     ps2.ZoneInstanceID
	lastSeen time.Time
}

type serverZone struct {
	ps2.WorldID
	ps2.ZoneInstanceID
}

// Manager maintains knowledge of worlds, zones, events, and population.
// It starts workers to keep it updated.
type Manager struct {
	collections      ps2db
	censusClient     *census.Client
	worker           sync.Mutex
	pop              popCounter
	worlds           map[ps2.WorldID]*worldStatus
	events           map[ps2.MetagameEventInstanceID]eventInstance
	players          map[ps2.CharacterID]trackedPlayer
	ps2alerts        chan ps2alerts.Instance
	mapData          chan census.ZoneMap
	censusEvents     chan event.Typer
	eventUpdates     chan eventInstance
	continentUnlocks chan continentUnlock
	zoneLookups      map[serverZone]time.Time // zoneLookups is a cache of

	queries     chan query
	unavailable chan struct{}
}

func (m *Manager) Register(c eventClient) {
	c.AddHandler(m.handleLogin)
	c.AddHandler(m.handleLogout)
	c.AddHandler(m.handleContinentUnlock)
	c.AddHandler(m.handleContinentLock)
	c.AddHandler(m.handleFacilityControl)
	c.AddHandler(m.handleDeath)
	c.AddHandler(m.handleVehicleDestroy)
	c.AddHandler(m.handleMetagame)
	c.AddHandler(m.handleGainExperience)
}

func (m *Manager) run(ctx context.Context) {
	m.worker.Lock()
	defer m.worker.Unlock()
	m.ps2alerts = make(chan ps2alerts.Instance)

	summaryTicker := time.NewTicker(15 * time.Second)
	defer summaryTicker.Stop()

	go func() {
		for {
			updateActiveEventInstances(ctx, m.ps2alerts)
			getMapData(ctx, m, m.mapData)
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Minute):
			}
		}
	}()
	m.queries = make(chan query)
	m.unavailable = make(chan struct{})
	defer close(m.unavailable)

	for {
		select {
		case <-ctx.Done():
			if m.eventUpdates != nil {
				close(m.eventUpdates)
			}
			if m.continentUnlocks != nil {
				close(m.continentUnlocks)
			}
			return
		case i := <-m.ps2alerts:
			handleInstance(m, i)
		case md := <-m.mapData:
			handleMap(m, md)
		case e := <-m.censusEvents:
			switch v := e.(type) {
			case event.ContinentLock:
				handleLock(m, v)
			case event.ContinentUnlock:
				handleUnlock(m, v)
			case event.PlayerLogout:
				handleLogout(m, v)
			case event.PlayerLogin:
				handleLogin(m, v)
			case event.MetagameEvent:
				handleMetagame(ctx, m, v, m.ps2alerts)
			case event.Death:
				handleDeath(m, v)
			case event.VehicleDestroy:
				handleVehicleDestroy(m, v)
			case event.GainExperience:
				handleGainExperience(m, v)
			case event.FacilityControl:
				handleFacilityControl(m, v) // when warpgates change, send to unlocks channel
				checkZone(ctx, m, serverZone{v.WorldID, v.ZoneID})
			}
		case <-summaryTicker.C:
			countPlayers(m)
			removeStaleEvents(m)
		case q := <-m.queries:
			q.Do(m)
		}
	}
}

// checkZone checks a zone to see if it should be added to active tracking.
func checkZone(ctx context.Context, m *Manager, z serverZone) {
	if t := m.zoneLookups[z]; time.Since(t) < time.Hour {
		return
	}
	m.zoneLookups[z] = time.Now()

	if _, tracking := m.worlds[z.WorldID].zones[z.ZoneInstanceID]; tracking {
		return
	}

	zoneData, _ := m.collections.GetZoneByID(z.ZoneID())
	if !zoneData.IsPlayable {
		return
	}

	// if other checks passed, then send it to the census api.
	// active zones will be sent back on the mapData channel and be intitialized for tracking in the consumer of that channel.
	go func() {
		ctx, stop := context.WithTimeout(ctx, 15*time.Second)
		defer stop()
		zm, err := census.GetMap(ctx, m.censusClient, z.WorldID, z.ZoneInstanceID)
		if err != nil {
			log.Printf("error: serverManager.checkZone: census.GetMap: %v", err)
			return
		}
		for _, z := range zm {
			select {
			case m.mapData <- z:
			case <-ctx.Done():
				return
			}
		}
	}()
}

type continentUnlock struct {
	ps2.WorldID
	ps2.ZoneID
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
		zone := m.collections.zones.Get(z.ZoneID())
		zs.Zone = zone.Zone
		ws.zones[z.ZoneInstanceID] = zs
	}
}

func handleMap(m *Manager, md census.ZoneMap) {
	trackZone(m, serverZone{md.WorldID, md.ZoneInstanceID})
	ws := m.worlds[md.WorldID]
	zs := ws.zones[md.ZoneInstanceID]
	if zs.continentState == continentStateLocked && !md.IsLocked() {
		m.emitContinentUnlock(continentUnlock{md.WorldID, md.ZoneID()})
	}
	if md.IsUnstable() {
		zs.continentState = continentStateUnstable
	} else if md.IsLocked() {
		zs.continentState = continentStateLocked
	} else {
		zs.continentState = continentStateUnlocked
	}
	zs.mapTimestamp = md.Timestamp
	ws.zones[md.ZoneInstanceID] = zs
}
func handleUnlock(m *Manager, e event.ContinentUnlock) {

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
	p := m.players[e.CharacterID]
	p.zone = e.ZoneID
	p.team = e.TeamID
	p.lastSeen = e.Timestamp
	m.players[e.CharacterID] = p
}
func handleVehicleDestroy(m *Manager, e event.VehicleDestroy) {
	p1 := m.players[e.AttackerCharacterID]
	p1.zone = e.ZoneID
	p1.team = e.TeamID
	p1.world = e.WorldID
	p1.lastSeen = e.Timestamp
	m.players[e.AttackerCharacterID] = p1

	p2 := m.players[e.CharacterID]
	p2.zone = e.ZoneID
	p2.team = e.TeamID
	p2.world = e.WorldID
	p2.lastSeen = e.Timestamp
	m.players[e.CharacterID] = p2
}
func handleDeath(m *Manager, e event.Death) {
	p1 := m.players[e.AttackerCharacterID]
	p1.zone = e.ZoneID
	p1.team = e.TeamID
	p1.world = e.WorldID
	p1.lastSeen = e.Timestamp
	m.players[e.AttackerCharacterID] = p1

	p2 := m.players[e.CharacterID]
	p2.zone = e.ZoneID
	p2.team = e.TeamID
	p2.world = e.WorldID
	p2.lastSeen = e.Timestamp
	m.players[e.CharacterID] = p2
}
func handleMetagame(ctx context.Context, m *Manager, e event.MetagameEvent, ch chan<- ps2alerts.Instance) {
	ed := m.collections.events.Get(e.MetagameEventID)

	switch e.MetagameEventState {
	case ps2.MetagameEventStarted:

	case ps2.MetagameEventRestarted:
	case ps2.MetagameEventCancelled, ps2.MetagameEventEnded:
		ei := m.events[e.EventInstanceID()]
		ei.TimeEnded = &e.Timestamp
		m.events[e.EventInstanceID()] = ei
	}
	if ed.isTerritory {
		go func() {
			// give ps2alerts a chance to create the event
			time.Sleep(10 * time.Second)
			ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			i, err := ps2alerts.GetInstanceContext(ctx, ps2alerts.InstanceID(e.EventInstanceID()))
			if err != nil {
				log.Printf("handleMetagame: failed to get instance: %v", err)
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
	p := m.players[e.CharacterID]
	p.world = e.WorldID
	p.lastSeen = e.Timestamp
	m.players[e.CharacterID] = p
}
func handleLogout(m *Manager, e event.PlayerLogout) {
	delete(m.players, e.CharacterID)
}
func (m *Manager) handleFacilityControl(e event.FacilityControl) {
	select {
	case m.censusEvents <- e:
	case <-m.unavailable:
		return
	}
}
func (m *Manager) handleGainExperience(e event.GainExperience) {
	select {
	case m.censusEvents <- e:
	case <-m.unavailable:
		return
	}
}
func (m *Manager) handleMetagame(e event.MetagameEvent) {
	select {
	case m.censusEvents <- e:
	case <-m.unavailable:
		return
	}
}
func (m *Manager) handleVehicleDestroy(e event.VehicleDestroy) {
	select {
	case m.censusEvents <- e:
	case <-m.unavailable:
		return
	}
}
func (m *Manager) handleDeath(e event.Death) {
	select {
	case m.censusEvents <- e:
	case <-m.unavailable:
		return
	}
}
func (m *Manager) handleContinentLock(e event.ContinentLock) {
	select {
	case m.censusEvents <- e:
	case <-m.unavailable:
		return
	}
}
func (m *Manager) handleContinentUnlock(e event.ContinentUnlock) {
	select {
	case m.censusEvents <- e:
	case <-m.unavailable:
		return
	}
}
func (m *Manager) handleLogin(e event.PlayerLogin) {
	select {
	case m.censusEvents <- e:
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

	for _, p := range m.players {

		// if we haven't seen any events for a player in more than X hours,
		// then we will assume that there is some kind of error in receiving events like logouts
		// and we'll exclude the player from the population counts.
		if time.Since(p.lastSeen) > 2*time.Hour {
			continue
		}
		pc := worldCount[p.world]
		pc[p.team]++
		worldCount[p.world] = pc

		z := zoneKey{p.world, p.zone}
		pc = zoneCount[z]
		pc[p.team]++
		zoneCount[z] = pc
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
	for id, e := range m.events {
		if time.Now().After(e.TimeStarted.Add(e.duration + 5*time.Minute)) {
			delete(m.events, id)
		}
	}
}

func (m *Manager) handleLogout(e event.PlayerLogout) {
	select {
	case m.censusEvents <- e:
	case <-m.unavailable:
		return
	}
}

func handleInstance(m *Manager, i ps2alerts.Instance) {
	id := ps2.MetagameEventInstanceID(i.InstanceID)
	ei := m.events[id]
	if ei.MetagameEventID == 0 {
		ei.metagameEventData = m.collections.events.Get(i.CensusMetagameEventType)
		ei.World = m.collections.servers.Get(i.World)
	}
	ei.Instance = i
	m.events[id] = ei

	select {
	case m.eventUpdates <- ei:
	default:
	}
}

func getMapData(ctx context.Context, m *Manager, results chan<- census.ZoneMap) {
	for wid, ws := range m.worlds {
		var zones []ps2.ZoneInstanceID
		for zid := range ws.zones {
			zones = append(zones, zid)
		}
		go func(w ps2.WorldID, zones []ps2.ZoneInstanceID) {
			ctx, stop := context.WithTimeout(ctx, 15*time.Second)
			defer stop()
			zm, err := census.GetMap(ctx, m.censusClient, w, zones...)
			if err != nil {
				log.Printf("error getting map state: %v", err)
				return
			}
			for _, z := range zm {
				results <- z
			}
		}(wid, zones)
	}
}

func updateInstance(ctx context.Context, i ps2alerts.InstanceID, ch chan<- ps2alerts.Instance) {
	instance, err := ps2alerts.GetInstance(i)
	if err != nil {
		log.Printf("updateInstance: %v", err)
		return
	}
	select {
	case ch <- instance:
	case <-ctx.Done():
		return
	}
}

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

func (m *Manager) getEvent(i ps2.MetagameEventInstanceID) (ei eventInstance, err error) {
	q := managerQuery[eventInstance]{
		query: func(m *Manager) (i eventInstance) {
			return m.events[ps2.MetagameEventInstanceID(i.InstanceID)]
		},
		result: make(chan eventInstance, 1),
	}

	if err := m.query(q); err != nil {
		return ei, err
	}

	r := <-q.result
	return r, nil
}

func (m *Manager) getServerInfoCard(serverID ps2.WorldID) (infoCardServerStatus, error) {

	q := managerQuery[infoCardServerStatus]{
		query: func(m *Manager) (c infoCardServerStatus) {
			w := m.worlds[serverID]
			c.serverID = w.WorldID
			c.serverName = w.Name.String()
			c.isLocked = w.isLocked
			c.pop = w.pop
			zc := make(map[ps2.ZoneID]int)
			for _, zs := range w.zones {
				zc[zs.ZoneID]++
				var cont struct {
					zoneID               ps2.ZoneID
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
				cont.zoneID = zs.ZoneID
				if zc[zs.ZoneID] == 1 {
					cont.name = zs.Name.String()
				} else {
					cont.name = fmt.Sprintf("%s %d", zs.Name, zc[zs.ZoneID])
				}
				var ei eventInstance
				for _, e := range m.events {
					if e.WorldID == serverID && e.ZoneID == zs.ZoneID {
						ei = e
						break
					}
				}
				cont.alertID = ei.MetagameEventID
				cont.alertName = ei.MetagameEvent.Name.String()
				cont.alertDescription = ei.MetagameEvent.Description.String()
				cont.alertIsContinentLock = ei.isContinentLock
				cont.pop = zs.pop
				cont.alertEnd = ei.TimeStarted.Add(ei.duration)
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

func eventInfoCard(ei eventInstance) (c infoCardMetagameEvent) {
	c.zone = ei.ZoneID
	c.eventName = ei.MetagameEvent.Name.String()
	c.eventDescription = ei.MetagameEvent.Description.String()
	c.triggeringFaction = ei.FactionID
	c.serverName = ei.World.Name.String()
	c.eventType = ei.metagameEventData.eventCategory.id
	c.populationBracket = ei.Instance.Bracket.String()
	// c.population = [5]int{}
	c.isTerritory = ei.isTerritory
	c.isContinentLock = ei.isContinentLock
	// c.territory = [5]int{}
	c.territory[VS] = ei.Result.Vs
	c.territory[NC] = ei.Result.Nc
	c.territory[TR] = ei.Result.Tr
	c.url = fmt.Sprintf("https://ps2alerts.com/alert/%s", ei.InstanceID)
	// c.imageURL = ""
	c.started = ei.TimeStarted
	c.ended = ei.TimeEnded
	c.duration = ei.duration
	c.victor = ei.Result.Victor
	return c
}

var goneHome = errors.New("manager is not running")

func (m *Manager) query(q query) error {
	select {
	case m.queries <- q:
		return nil
	case <-m.unavailable:
		return goneHome
	}
}
func newWorldManager(w ps2.World, continents []planetside.Zone) *worldStatus {
	ws := &worldStatus{
		World: w,
		zones: make(map[ps2.ZoneInstanceID]zoneStatus),
	}

	for _, z := range continents {
		ws.zones[ps2.ZoneInstanceID(z.ZoneID)] = zoneStatus{Zone: z.Zone}
	}
	return ws
}

type worldStatus struct {
	ps2.World
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
	zs.continentState = continentStateLocked
	w.zones[z] = zs
}

type zoneStatus struct {
	ps2.Zone
	continentState continentState
	lockedAt       time.Time
	pop            popCounter
	mapTimestamp   time.Time
}

type eventInstance struct {
	ps2alerts.Instance
	metagameEventData
	ps2.World
}

type managerQuery[T any] struct {
	query  func(*Manager) T
	result chan T
}
type queryResult[T any] struct {
	value T
	err   error
}

func (mq managerQuery[T]) Do(m *Manager) {
	mq.result <- mq.query(m)
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
