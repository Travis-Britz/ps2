package ps2alerts

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/Travis-Britz/ps2"
)

type InstanceID ps2.MetagameEventInstanceID

func (i InstanceID) String() string { return fmt.Sprintf("%d-%d", i.WorldID, i.InstanceID) }
func (i *InstanceID) UnmarshalJSON(b []byte) (err error) {
	b = bytes.Trim(b, "\"")
	*i, err = parseInstance(b)
	if err != nil {
		return fmt.Errorf("ps2alerts.Instance.UnmarshalJSON: %w", err)
	}
	return nil
}

func (i *InstanceID) Scan(src any) (err error) {
	var v []byte
	switch src.(type) {
	case string:
		v = []byte(src.(string))
	case []byte:
		v = src.([]byte)
	default:
		return fmt.Errorf("ps2alerts.Instance.Scan: unhandled type '%T'", src)
	}
	if *i, err = parseInstance([]byte(v)); err != nil {
		return fmt.Errorf("ps2alerts.Instance.Scan: %w", err)
	}
	return nil
}
func parseInstance(b []byte) (i InstanceID, err error) {
	world, instance, found := bytes.Cut(b, []byte("-"))
	if !found {
		return i, fmt.Errorf("missing separator in instance id '%s'", b)
	}
	var worldid ps2.WorldID
	if err := json.Unmarshal(world, &worldid); err != nil {
		return i, fmt.Errorf("error unmarshaling world: %w", err)
	}
	var instanceid ps2.InstanceID
	if err := json.Unmarshal(instance, &instanceid); err != nil {
		return i, fmt.Errorf("error unmarshaling instance: %w", err)
	}
	i.WorldID = worldid
	i.InstanceID = instanceid
	return i, nil
}
func (i InstanceID) Value() (driver.Value, error) {
	return i.String(), nil
}
func (i InstanceID) MarshalJSON() (json []byte, err error) {
	json = append(json, '"')
	json = append(json, i.String()...)
	json = append(json, '"')
	return json, nil
}

type eventType int

func (t eventType) String() string {
	// https://github.com/ps2alerts/constants/blob/main/ps2AlertsEventType.ts
	switch t {
	case 1:
		return "Live Metagame"
	case 2:
		return "Outfit Wars 2022"
	default:
		return fmt.Sprintf("Unknown type %d", t)
	}
}

type eventState int

func (s eventState) String() string {
	// https://github.com/ps2alerts/constants/blob/main/ps2AlertsEventState.ts
	switch s {
	case 0:
		return "Starting"
	case 1:
		return "Started"
	case 2:
		return "Ended"
	default:
		return fmt.Sprintf("Unknown state %d", s)
	}
}

type Bracket int

const (
	dead Bracket = iota + 1
	low
	medium
	high
	prime
)

func (b Bracket) String() string {
	// https://github.com/ps2alerts/constants/blob/main/bracket.ts
	switch b {
	case -1:
		return "Unknown"
	case dead:
		return "Dead"
	case low:
		return "Low"
	case medium:
		return "Medium"
	case high:
		return "High"
	case prime:
		return "Prime"
	default:
		return "Undefined-" + strconv.Itoa(int(b))
	}
}

func (b Bracket) Min() int {
	const platoon = 48
	switch b {
	case prime:
		return 4 * platoon
	case high:
		return 3 * platoon
	case medium:
		return 2 * platoon
	case low:
		return 1 * platoon
	default:
		return 1
	}
}

type Instance struct {
	ID                      string              `json:"_id"`
	World                   ps2.WorldID         `json:"world"`
	CensusInstanceID        ps2.InstanceID      `json:"censusInstanceId"`
	InstanceID              InstanceID          `json:"instanceId"`
	Zone                    ps2.ZoneInstanceID  `json:"zone"`
	TimeStarted             time.Time           `json:"timeStarted"`
	TimeEnded               *time.Time          `json:"timeEnded"`
	CensusMetagameEventType ps2.MetagameEventID `json:"censusMetagameEventType"`
	Duration                duration            `json:"duration"`
	State                   eventState          `json:"state"`
	Ps2AlertsEventType      eventType           `json:"ps2AlertsEventType"`
	Bracket                 Bracket             `json:"bracket"`
	MapVersion              string              `json:"mapVersion"`
	Result                  struct {
		Vs                int            `json:"vs"`
		Nc                int            `json:"nc"`
		Tr                int            `json:"tr"`
		Cutoff            int            `json:"cutoff"`
		OutOfPlay         int            `json:"outOfPlay"`
		Victor            *ps2.FactionID `json:"victor"`
		Draw              bool           `json:"draw"`
		PerBasePercentage float64        `json:"perBasePercentage"`
	} `json:"result"`
	Features struct {
		CaptureHistory bool `json:"captureHistory"`
		Xpm            bool `json:"xpm"`
	} `json:"features"`
}

// duration represents a ps2alerts duration for unmarshaling, which is in milliseconds.
type duration time.Duration

func (d *duration) UnmarshalJSON(b []byte) error {
	var dd time.Duration
	if err := json.Unmarshal(b, &dd); err != nil {
		return fmt.Errorf("unmarshal ps2alerts.duration: %w", err)
	}
	*d = duration(dd) * 1e6
	return nil
}
func (d *duration) Use(dd time.Duration) {
	*d = duration(dd)
}

func (d duration) Duration() time.Duration { return time.Duration(d) }
