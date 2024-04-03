package state

import (
	"github.com/Travis-Britz/ps2"
	"github.com/Travis-Britz/ps2/psmap"
)

//todo: emit alert starts

type WorldPopulation struct {
	World worldpop
	Zones map[ps2.ZoneID]zonepop
}

type PopulationTotal map[ps2.WorldID]WorldPopulation

// OnPopulationTotal adds a function that will be called every time populations are summed.
func (manager *Manager) OnPopulationTotal(f func(PopulationTotal)) {
	manager.populationHandlers = append(manager.populationHandlers, f)
}

func emitPopulationSums(manager *Manager) {
	pt := manager.state.Population()
	for _, f := range manager.populationHandlers {
		f(pt)
	}
}

type TerritoryChange struct {
	WorldID ps2.WorldID
	ZoneID  ps2.ZoneInstanceID
	Regions map[ps2.RegionID]ps2.FactionID
	Cutoff  map[ps2.RegionID]bool
}

func (manager *Manager) OnTerritoryChange(f func(TerritoryChange)) {
	manager.territoryChangeHandlers = append(manager.territoryChangeHandlers, f)
}
func emitTerritoryChange(manager *Manager, zone uniqueZone, territory map[ps2.RegionID]ps2.FactionID, cutoff map[ps2.RegionID]bool) {
	tc := TerritoryChange{
		WorldID: zone.WorldID,
		ZoneID:  zone.ZoneInstanceID,
		Regions: territory,
		Cutoff:  cutoff,
	}
	for _, f := range manager.territoryChangeHandlers {
		f(tc)
	}
}

type ZoneStatusChange struct {
	WorldID ps2.WorldID
	ZoneID  ps2.ZoneInstanceID
	Status  psmap.Status
}

func (manager *Manager) OnZoneStatusChange(f func(ZoneStatusChange)) {
	manager.zoneStatusChangeHandlers = append(manager.zoneStatusChangeHandlers, f)
}
func emitZoneStateChange(manager *Manager, id uniqueZone, status psmap.Status) {
	for _, f := range manager.zoneStatusChangeHandlers {
		f(ZoneStatusChange{
			WorldID: id.WorldID,
			ZoneID:  id.ZoneInstanceID,
			Status:  status,
		})
	}
}

func (manager *Manager) OnEventUpdate(f func(EventState)) {
	manager.eventUpdateHandlers = append(manager.eventUpdateHandlers, f)
}
func emitEventUpdate(manager *Manager, event EventState) {
	for _, f := range manager.eventUpdateHandlers {
		f(event)
	}
}
