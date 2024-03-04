package wsc

import (
	"bytes"
	"fmt"

	"github.com/Travis-Britz/ps2"
)

type commander interface {
	command() command
}

type Subscribe struct {
	Events                         []ps2.Event
	ExperienceIDs                  []ps2.ExperienceID
	Worlds                         []ps2.WorldID
	Characters                     []ps2.CharacterID
	LogicalAndCharactersWithWorlds bool
}

func (s *Subscribe) AllWorlds() *Subscribe {
	s.Worlds = []ps2.WorldID{}
	return s
}
func (s *Subscribe) AddWorld(w ...ps2.WorldID) *Subscribe {
	s.Worlds = append(s.Worlds, w...)
	return s
}

func (s *Subscribe) AllCharacters() *Subscribe {
	s.Characters = []ps2.CharacterID{}
	return s
}

func (s *Subscribe) AllEvents() *Subscribe {
	s.Events = []ps2.Event{}
	return s
}

func (s *Subscribe) All() *Subscribe {
	s.AllWorlds()
	s.AllEvents()
	s.AllCharacters()
	return s
}

func (s Subscribe) command() command {
	c := command{
		Action:                         subscribe,
		Service:                        eventService,
		LogicalAndCharactersWithWorlds: &s.LogicalAndCharactersWithWorlds,
	}

	// When Events, Worlds, or Characters are left nil (the default) we will not explicitly set them.
	// However, when they are explicitly set to a zero-length slice we will take that as the special case to include all.
	for _, eid := range s.ExperienceIDs {
		c.EventNames = append(c.EventNames, fmt.Sprintf("GainExperience_experience_id_%d", eid))
	}

	if s.Events != nil && len(s.Events) == 0 {
		c.EventNames = []string{"all"}
	} else {
		for _, e := range s.Events {
			c.EventNames = append(c.EventNames, e.EventName())
		}
	}
	if s.Characters != nil && len(s.Characters) == 0 {
		c.Characters = []string{"all"}
	} else {
		for _, ch := range s.Characters {
			c.Characters = append(c.Characters, ch.String())
		}
	}
	if s.Worlds != nil && len(s.Worlds) == 0 {
		c.Worlds = []string{"all"}
	} else {
		for _, w := range s.Worlds {
			c.Worlds = append(c.Worlds, w.StringID())
		}
	}
	return c
}

type command struct {
	Action                         action      `json:"action"`
	Service                        service     `json:"service"`
	EventNames                     []string    `json:"eventNames,omitempty"`
	Worlds                         []string    `json:"worlds,omitempty"`
	Characters                     []string    `json:"characters,omitempty"`
	LogicalAndCharactersWithWorlds *bool       `json:"logicalAndCharactersWithWorlds,omitempty"`
	All                            *stringBool `json:"all,omitempty"`
}

type stringBool bool

func (b stringBool) MarshalJSON() ([]byte, error) {
	if b {
		return []byte("\"true\""), nil
	}
	return []byte("\"false\""), nil
}
func (b *stringBool) UnmarshalJSON(data []byte) error {
	data = bytes.Trim(data, "\"")
	if bytes.Equal(data, []byte("true")) {
		*b = true
	}
	return nil
}

func (c command) command() command { return c }

var SubscribeAll = command{
	Service:    eventService,
	Action:     subscribe,
	EventNames: []string{"all"},
	Characters: []string{"all"},
	Worlds:     []string{"all"},
}

var ClearAll = command{
	Service: eventService,
	Action:  clearSubscribe,
	All:     func(b bool) *stringBool { return (*stringBool)(&b) }(true),
}

var ListRecentCharacterIds = command{
	Action:  recentCharacterIds,
	Service: eventService,
}
var CountRecentCharacterIds = command{
	Action:  recentCharacterIdsCount,
	Service: eventService,
}
