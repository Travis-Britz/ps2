package wsc

import (
	"encoding/json"

	"github.com/Travis-Britz/ps2/event"
)

type rawMessage struct {
	Service service     `json:"service"`
	Type    messageType `json:"type"`
	heartbeatMessage
	serviceStateChangedMessage
	connectionStateChangedMessage
	subscriptionMessage
	eventServiceMessage
}

func (m *rawMessage) UnmarshalJSON(data []byte) error {
	var tmp map[string]json.RawMessage

	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}

	if tmp["service"] == nil && tmp["type"] == nil && tmp["subscription"] != nil {
		return json.Unmarshal(tmp["subscription"], &m.subscriptionMessage)
	}

	if err := json.Unmarshal(tmp["service"], &m.Service); err != nil {
		return err
	}
	if err := json.Unmarshal(tmp["type"], &m.Type); err != nil {
		return err
	}

	switch {
	case m.Service == eventService && m.Type == serviceMessage:
		return json.Unmarshal(data, &m.eventServiceMessage)
	case m.Service == eventService && m.Type == heartbeat:
		return json.Unmarshal(data, &m.heartbeatMessage)
	case m.Service == eventService && m.Type == serviceStateChanged:
		return json.Unmarshal(data, &m.serviceStateChangedMessage)
	case m.Service == push && m.Type == connectionStateChanged:
		return json.Unmarshal(data, &m.connectionStateChangedMessage)
	}

	return nil
}

func (m rawMessage) message() any {
	switch {
	case m.Service == eventService && m.Type == serviceMessage:
		return m.eventServiceMessage.Payload.Event()
	case m.Service == eventService && m.Type == heartbeat:
		return m.heartbeatMessage
	case m.Service == eventService && m.Type == serviceStateChanged:
		return m.serviceStateChangedMessage
	case m.Service == push && m.Type == connectionStateChanged:
		return m.connectionStateChangedMessage
	case !m.subscriptionMessage.IsEmpty():
		return m.subscriptionMessage
	}
	return nil
}

type heartbeatMessage struct {
	Online map[string]stringBool `json:"online"`
}

type serviceStateChangedMessage struct {
	Detail endpoint   `json:"detail"`
	Online stringBool `json:"online"`
}

type connectionStateChangedMessage struct {
	Connected stringBool `json:"connected"`
}

type subscriptionMessage struct {
	Subscription struct {
		Characters                     []string `json:"characters"`
		EventNames                     []string `json:"eventNames"`
		LogicalAndCharactersWithWorlds bool     `json:"logicalAndCharactersWithWorlds"`
		Worlds                         []string `json:"worlds"`
	} `json:"subscription"`
}

func (s subscriptionMessage) IsEmpty() bool {
	return s.Subscription.Characters == nil && s.Subscription.EventNames == nil && s.Subscription.Worlds == nil && s.Subscription.LogicalAndCharactersWithWorlds == false
}

type eventServiceMessage struct {
	Payload event.Raw `json:"payload"`
}
