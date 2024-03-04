package honu

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/Travis-Britz/ps2"
)

func GetWorldPop(ctx context.Context, w ps2.WorldID) (p PopResult, err error) {
	url := "https://wt.honu.pw/api/population/" + w.StringID()
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
	err = json.Unmarshal(body, &p)
	return p, err
}

func GetWorldPopMultiple(ctx context.Context, worlds ...ps2.WorldID) (p []PopResult, err error) {
	q := url.Values{}
	for _, w := range worlds {
		q.Add("worldid", w.StringID())
	}
	url := "https://wt.honu.pw/api/population/multiple" + q.Encode()
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
	err = json.Unmarshal(body, &p)
	return p, err
}

type PopResult struct {
	WorldID     ps2.WorldID `json:"worldID"`
	Timestamp   time.Time   `json:"timestamp"`
	CachedUntil time.Time   `json:"cachedUntil"`
	Total       int         `json:"total"`
	Vs          int         `json:"vs"`
	Nc          int         `json:"nc"`
	Tr          int         `json:"tr"`
	Ns          int         `json:"ns"`
	NsVs        int         `json:"ns_vs"`
	NsNc        int         `json:"ns_nc"`
	NsTr        int         `json:"ns_tr"`
	NsOther     int         `json:"nsOther"`
}
