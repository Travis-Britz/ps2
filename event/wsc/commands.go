package wsc

import "bytes"

type commander interface {
	command() command
}

type Subscribe struct {
	Events                         []string
	Worlds                         []string
	Characters                     []string
	LogicalAndCharactersWithWorlds bool
}

func (s Subscribe) command() command {
	return command{
		Action:                         subscribe,
		Service:                        eventService,
		EventNames:                     s.Events,
		Worlds:                         s.Worlds,
		Characters:                     s.Characters,
		LogicalAndCharactersWithWorlds: &s.LogicalAndCharactersWithWorlds,
	}
}

type command struct {
	Action                         action      `json:"action,omitempty"`
	Service                        service     `json:"service,omitempty"`
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
