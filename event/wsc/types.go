package wsc

import (
	"bytes"
	"fmt"
)

type service uint8

const (
	eventService service = iota
	push
)

var services = map[service]string{
	eventService: "event",
	push:         "push",
}

func (e *service) UnmarshalJSON(data []byte) error {
	data = bytes.Trim(data, "\"")
	for ev, s := range services {
		if bytes.Equal(data, []byte(s)) {
			*e = ev
			return nil
		}
	}
	return fmt.Errorf("endpoint.UnmarshalJSON: invalid value '%s' for endpoint", data)
}
func (s service) String() string { return services[s] }

func (s service) MarshalJSON() ([]byte, error) { return []byte("\"" + s.String() + "\""), nil }

type messageType uint8

func (mt *messageType) UnmarshalJSON(data []byte) error {
	data = bytes.Trim(data, "\"")
	for ev, s := range messageTypes {
		if bytes.Equal(data, []byte(s)) {
			*mt = ev
			return nil
		}
	}
	return fmt.Errorf("messageType.UnmarshalJSON: invalid value '%s' for messageType", data)
}

func (mt messageType) String() string { return messageTypes[mt] }

const (
	connectionStateChanged messageType = iota
	heartbeat
	serviceStateChanged
	serviceMessage
)

var messageTypes = map[messageType]string{
	connectionStateChanged: "connectionStateChanged",
	heartbeat:              "heartbeat",
	serviceMessage:         "serviceMessage",
	serviceStateChanged:    "serviceStateChanged",
}

type action uint8

const (
	subscribe action = iota
	clearSubscribe
	recentCharacterIds
	recentCharacterIdsCount
	echo
	help
)

func (a action) String() string {
	switch a {
	case subscribe:
		return "subscribe"
	case clearSubscribe:
		return "clearSubscribe"
	case recentCharacterIds:
		return "recentCharacterIds"
	case recentCharacterIdsCount:
		return "recentCharacterIdsCount"
	case echo:
		return "echo"
	default:
		return ""
	}
}
func (a action) MarshalJSON() ([]byte, error) {
	return []byte("\"" + a.String() + "\""), nil
}

type endpoint int

const (
	connery  endpoint = 1
	miller   endpoint = 10
	cobalt   endpoint = 13
	emerald  endpoint = 17
	jaeger   endpoint = 19
	soltech  endpoint = 40
	genudine endpoint = 1000
	ceres    endpoint = 2000
	lithcorp endpoint = 2001
	rashnu   endpoint = 2002
)

var endpoints = map[endpoint]string{
	connery:  "EventServerEndpoint_Connery_1",
	cobalt:   "EventServerEndpoint_Cobalt_13",
	emerald:  "EventServerEndpoint_Emerald_17",
	jaeger:   "EventServerEndpoint_Jaeger_19",
	miller:   "EventServerEndpoint_Miller_10",
	soltech:  "EventServerEndpoint_Soltech_40",
	genudine: "EventServerEndpoint_1000_Genudine_1001_Palos_1002_Crux_1003_Searhus_1004_Xelas",
	ceres:    "EventServerEndpoint_2000_Ceres",
	lithcorp: "EventServerEndpoint_2001_Lithcorp",
	rashnu:   "EventServerEndpoint_2002_Rashnu",
}

func (e *endpoint) UnmarshalJSON(data []byte) error {
	data = bytes.Trim(data, "\"")
	for ev, s := range endpoints {
		if bytes.Equal(data, []byte(s)) {
			*e = ev
			return nil
		}
	}
	return fmt.Errorf("endpoint.UnmarshalJSON: invalid value '%s' for endpoint", data)
}
