package census

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"

	"github.com/Travis-Britz/ps2"
)

func GetCharacterIDByName(ctx context.Context, client *Client, e ps2.Environment, name string) (ps2.CharacterID, error) {
	if client == nil {
		client = DefaultClient
	}
	r := struct {
		CharacterNameList []struct {
			CharacterID ps2.CharacterID `json:"character_id,string"`
		} `json:"character_name_list"`
	}{}
	err := client.Get(
		ctx,
		e,
		fmt.Sprintf("character_name?name.first_lower=%s&c:limit=1&c:case=false", url.QueryEscape(name)),
		&r,
	)
	if err != nil {
		return 0, fmt.Errorf("census.GetCharacterIDByName: %w for \"%s\"", err, name)
	}
	if len(r.CharacterNameList) == 0 {
		return 0, noResults{q: name}
	}
	return r.CharacterNameList[0].CharacterID, nil

}

type collectionNamer interface {
	CollectionName() string
}

func LoadCollection[T collectionNamer](ctx context.Context, client *Client, collected *[]T) error {
	if client == nil {
		client = DefaultClient
	}
	var n T
	collection := n.CollectionName()
	const perPage = 5000
	for start, more := 0, true; more; start += perPage {
		var result map[string]json.RawMessage
		err := client.Get(ctx, ps2.PC, fmt.Sprintf("%s?c:limit=%d&c:start=%d", collection, perPage, start), &result)
		if err != nil {
			return err
		}
		if _, exists := result[collection+"_list"]; !exists {
			return errors.New("response didn't contain the expected collection")
		}

		rawList := result[collection+"_list"]
		pageResults := make([]T, 0, perPage)
		if err = json.Unmarshal(rawList, &pageResults); err != nil {
			return err
		}
		more = false
		if len(pageResults) == perPage {
			more = true
		}
		*collected = append(*collected, pageResults...)
	}
	return nil
}
