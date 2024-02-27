package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/Travis-Britz/ps2/census"
)

var client *census.Client

func main() {
	var typ census.Zone
	var censusKey string
	flag.StringVar(&censusKey, "key", "example", "Census API client key")
	flag.Parse()

	client = &census.Client{
		Key: censusKey,
	}

	if err := SaveCollectionToFile(".", typ); err != nil {
		log.Fatal("couldn't save file: ", err)
	}

}

func SaveCollectionToFile[T collection](dir string, typ T) error {
	collection := make([]T, 0, 500)
	var n T
	collectionName := n.CollectionName()

	if err := census.LoadCollection(context.Background(), client, &collection); err != nil {
		return err
	}
	fullpath := filepath.Join(dir, collectionName+".json")
	f, err := os.Create(fullpath)
	if err != nil {
		return fmt.Errorf("SaveCollectionToFile: could not create file %q: %w", fullpath, err)
	}
	defer f.Close()
	var b []byte
	b, err = json.MarshalIndent(collection, "", "    ")
	if err != nil {
		return fmt.Errorf("SaveCollectionToFile: unable to marshal collection to bytes: %w", err)
	}
	f.Write(b)
	return nil
}

type collection interface {
	CollectionName() string
}
