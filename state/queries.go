package state

import (
	"fmt"

	"github.com/Travis-Britz/ps2"
)

func (manager *Manager) State() (GlobalState, error) {
	question := managerQuery[GlobalState]{
		queryFn: func(manager *Manager) GlobalState {
			return manager.state.Clone()
		},
		result: make(chan GlobalState, 1),
	}

	if err := manager.query(question); err != nil {
		return GlobalState{}, err
	}
	result := <-question.result
	return result, nil
}

func (manager *Manager) WorldState(world ps2.WorldID) (WorldState, error) {
	gState, err := manager.State()
	if err != nil {
		return WorldState{}, err
	}
	wState := gState.getWorld(world)
	if wState.WorldID == 0 {
		return WorldState{}, fmt.Errorf("manager.WorldState: world %d not found", world)
	}
	return wState, nil
}
