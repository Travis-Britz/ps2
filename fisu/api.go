package fisu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/Travis-Britz/ps2"
)

func GetWorldPop(ctx context.Context, w ps2.WorldID) (p WorldPop, err error) {
	url := "https://ps2.fisu.pw/api/population/?world=" + w.String()
	log.Printf("checking: %s", url)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return p, err
	}
	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		return p, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return p, fmt.Errorf("returned http %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return p, err
	}
	var result popResultSingle
	err = json.Unmarshal(body, &result)
	if len(result.Result) == 0 {
		return p, errors.New("no results")
	}
	return result.Result[0], err
}

// func GetWorldPopMultiple(ctx context.Context, worlds ...ps2.WorldID) (r []WorldPop, err error) {

// 	return r, errors.New("not implemented")
// }

type WorldPop struct {
	WorldID   ps2.WorldID `json:"worldId"`
	Timestamp timestamp   `json:"timestamp"`
	Vs        int         `json:"vs"`
	Nc        int         `json:"nc"`
	Tr        int         `json:"tr"`
	Ns        int         `json:"ns"`
	Unknown   int         `json:"unknown"`
}

type popResultSingle struct {
	Config struct {
		World []ps2.WorldID `json:"world"`
	} `json:"config"`
	Result []WorldPop `json:"result"`
	Timing struct {
		StartMs   int `json:"start-ms"`
		QueryMs   int `json:"query-ms"`
		ProcessMs int `json:"process-ms"`
		TotalMs   int `json:"total-ms"`
	} `json:"timing"`
}

type popResultMultiple struct {
	Config struct {
		World []ps2.WorldID `json:"world"`
	} `json:"config"`
	Result map[string][]WorldPop `json:"result"`
	Timing struct {
		StartMs   int `json:"start-ms"`
		QueryMs   int `json:"query-ms"`
		ProcessMs int `json:"process-ms"`
		TotalMs   int `json:"total-ms"`
	} `json:"timing"`
}

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
