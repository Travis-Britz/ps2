package ps2alerts

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/Travis-Britz/ps2"
)

func Get(w ps2.WorldID, i ps2.InstanceID) (instance Instance, err error) {
	return GetInstance(InstanceID{w, i})
}

func GetInstance(id InstanceID) (i Instance, err error) {
	return GetInstanceContext(context.Background(), id)
}

func GetInstanceContext(ctx context.Context, id InstanceID) (i Instance, err error) {
	i.InstanceID = id
	url := "https://api.ps2alerts.com/instances/" + id.String()
	slog.Info("ps2alerts query", "url", url)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return i, err
	}
	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		return i, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return i, fmt.Errorf("returned http %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return i, err
	}
	err = json.Unmarshal(body, &i)
	return i, err
}

func GetActive() (i []Instance, err error) {
	return GetActiveContext(context.Background())
}

func GetActiveContext(ctx context.Context) (i []Instance, err error) {
	url := "https://api.ps2alerts.com/instances/active"
	slog.Info("ps2alerts query", "url", url)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("returned http %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return i, err
	}
	err = json.Unmarshal(body, &i)
	return i, err
}
