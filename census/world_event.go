package census

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Travis-Britz/ps2"
	"github.com/Travis-Britz/ps2/event"
)

var maxTime = time.Unix(1<<63-62135596801, 999999999)

func GetFacilityControlEvents(ctx context.Context, c *Client, env ps2.Environment, before *time.Time, after *time.Time, worlds ...ps2.WorldID) (events []event.FacilityControl, first time.Time, last time.Time, err error) {
	var response worldEventResponse
	events = make([]event.FacilityControl, 0, 1000)
	last = time.Unix(0, 0).UTC()

	q := "world_event?type=FACILITY&c:limit=1000"

	if before != nil {
		q += "&before=" + strconv.FormatInt(before.Unix(), 10)
	}
	if after != nil {
		q += "&after=" + strconv.FormatInt(after.Unix(), 10)
	}

	if worlds != nil {
		s := make([]string, 0, 10)
		for _, w := range worlds {
			s = append(s, w.String())
		}
		q += "&world_id=" + strings.Join(s, ",")
	}

	if err = c.Get(ctx, env, q, &response); err != nil {
		return
	}
	for _, ev := range response.WorldEventList {
		untyped := ev.Raw.Event()
		e, ok := untyped.(event.FacilityControl)
		if !ok {
			err = fmt.Errorf("unexpected event type '%T'", untyped)
			return
		}
		if e.Timestamp.Before(last) {
			last = e.Timestamp
		}
		if e.Timestamp.After(first) {
			first = e.Timestamp
		}
		events = append(events, e)
	}
	return
}

type worldEventResponse struct {
	WorldEventList []struct {
		event.Raw
		ObjectiveID int    `json:"objective_id,string"`
		TableType   string `json:"table_type"`
	} `json:"world_event_list"`
	Returned int `json:"returned"`
}

// duration represents a census duration for unmarshaling, which is in seconds.
type duration time.Duration

func (d *duration) UnmarshalJSON(b []byte) error {
	var dd time.Duration
	if err := json.Unmarshal(b, &dd); err != nil {
		return fmt.Errorf("unmarshal census.duration: %w", err)
	}
	*d = duration(dd) * 1e9
	return nil
}
func (d *duration) Use(dd time.Duration) {
	*d = duration(dd)
}

func (d duration) Duration() time.Duration { return time.Duration(d) }

type timestamp time.Time

func (t *timestamp) UnmarshalJSON(b []byte) error {
	var i int64
	if err := json.Unmarshal(b, &i); err != nil {
		return fmt.Errorf("fisu.timestamp.UnmarshalJSON: %w", err)
	}
	*t = timestamp(time.Unix(i, 0).UTC())
	return nil
}

func (t timestamp) Time() time.Time { return time.Time(t) }
