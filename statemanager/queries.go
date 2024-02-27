package statemanager

import (
	"fmt"
	"time"

	"github.com/Travis-Britz/ps2"
)

func (manager *Manager) State() (GlobalState, error) {
	q := managerQuery[GlobalState]{
		query: func(manager *Manager) (gs GlobalState) {
			gs.Worlds = make([]WorldState, 0, len(manager.worlds))
			for worldid, worldstate := range manager.worlds {
				pop := worldpop{}
				pop.NC = worldstate.pop[NC]
				pop.VS = worldstate.pop[VS]
				pop.TR = worldstate.pop[TR]
				pop.NSO = worldstate.pop[NSO]
				pop.Unknown = worldstate.pop[None]
				wstate := WorldState{
					WorldID:    worldstate.WorldID,
					Name:       worldstate.Name.String(),
					Population: pop,
					Zones:      make([]ZoneState, 0, len(worldstate.zones)),
				}
				for zoneid, zonestate := range worldstate.zones {
					var lastLock *time.Time = nil
					if !zonestate.lockedAt.IsZero() {
						lastLock = &zonestate.lockedAt
					}

					zstate := ZoneState{
						ZoneID:         zoneid,
						ZoneName:       zonestate.Name.String(),
						ContinentState: zonestate.continentState,
						Population: zonepop{
							VS: zonestate.pop[VS],
							NC: zonestate.pop[NC],
							TR: zonestate.pop[TR],
						},
						LastLock:   lastLock,
						LastUnlock: nil,
						Event:      nil,
					}
					var ei eventInstance
					for _, e := range manager.activeMetagameEvents {
						if e.WorldID == worldid && e.Instance.Zone == zoneid {
							ei = e
							break
						}
					}
					if ei.MetagameEventID != 0 {
						zstate.Event = &EventState{
							MetagameEventID:  ei.MetagameEventID,
							InstanceID:       ei.CensusInstanceID,
							EventName:        ei.Name.String(),
							EventDescription: ei.MetagameEvent.Description.String(),
							EventDuration:    inSeconds{ei.MetagameEvent.Duration},
							IsContinentLock:  ps2.IsContinentLock(ei.MetagameEventID),
							IsTerritory:      ps2.IsTerritoryAlert(ei.MetagameEventID),
							StartingFaction:  ps2.StartingFaction(ei.MetagameEventID),
							Score: score{
								VS: float64(ei.Result.Vs),
								NC: float64(ei.Result.Nc),
								TR: float64(ei.Result.Tr),
							},
							EventURL: fmt.Sprintf("https://ps2alerts.com/alert/%s", ei.InstanceID),
							Victor:   ei.Result.Victor,
							Started:  ei.TimeStarted,
							Ended:    ei.TimeEnded,
						}
					}

					wstate.Zones = append(wstate.Zones, zstate)
				}
				gs.Worlds = append(gs.Worlds, wstate)
			}
			return gs
		},
		result: make(chan GlobalState, 1),
	}

	if err := manager.query(q); err != nil {
		return GlobalState{}, err
	}
	r := <-q.result
	return r, nil
}
