package census

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/Travis-Britz/ps2"
)

func Namespace(e ps2.Environment) string {
	switch e {
	case ps2.EnvPC:
		return "ps2:v2"
	case ps2.EnvPS4US:
		return "ps2ps4us:v2"
	case ps2.EnvPS4EU:
		return "ps2ps4eu:v2"
	default:
		return ""
	}
}

var defaultClient = &Client{Key: "example"}

var DefaultClient = defaultClient

type Client struct {
	Key string
}

func (c Client) Get(ctx context.Context, env ps2.Environment, query string, result any) error {
	const apiBase = "https://census.daybreakgames.com"

	url := fmt.Sprintf("%s/s:%s/get/%s/%s", apiBase, c.Key, Namespace(env), query)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	log.Println("checking:", url)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("census.Client.Get: client.Do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("census.Client.Get: returned http %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("census.Client.Get: read body:%w", err)
	}
	if err = json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("census.Client.Get: UnmarshalJSON: %w", err)
	}
	return nil
}
