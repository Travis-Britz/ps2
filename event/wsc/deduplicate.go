package wsc

import (
	"slices"

	"github.com/Travis-Britz/ps2/event"
)

// deduplicator is a naive implementation for deduplicating events.
// A more efficient implementation would use a circular buffer sorted by timestamp.
type deduplicator []event.UniqueKey

type uniqueTimestampedEvent interface {
	event.Typer
	event.UniqueKeyer
	event.Timestamper
}

func (d *deduplicator) InsertFresh(k uniqueTimestampedEvent) bool {
	if slices.Contains(*d, k.Key()) {
		return false
	}
	d.Purge()
	*d = append(*d, k.Key())
	return true
}

// Purge checks if the deduplicator is full,
// and if it is,
// it will remove half of the events and resize the slice.
func (d *deduplicator) Purge() {
	sli := *d
	if len(sli) != cap(sli) {
		return
	}
	newSize := len(sli) / 2
	n := copy(sli[0:newSize], sli[len(sli)-newSize:])
	*d = sli[0:n]
}
