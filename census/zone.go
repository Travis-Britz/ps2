package census

import (
	"encoding/json"
	"fmt"

	"github.com/Travis-Britz/ps2"
)

// Zone is the struct returned by the official Census API for a zone.
//
// There is some complexity around ZoneID and ContinentID.
// The code is complex because the reality is complex.
// See the docs for [ps2.ContinentID] for more information.
//
// When querying the census api,
// use the ZoneID field.
//
// When storing records in a database,
// use ContinentID as a surrogate key.
// When looking up a zone from a realtime event,
// query the local database by the ContinentID.
type Zone struct {
	// ContinentID is the ID used by the realtime events service to identify a zone.
	// This is the field that should be stored in local databases to query events against.
	ContinentID ps2.ContinentID `json:"-"`

	// ZoneID is the Zone ID used internally by planetside
	ZoneID      ps2.ZoneID        `json:"zone_id,string"`
	Code        string            `json:"code"`
	HexSize     int               `json:"hex_size,string"`
	Name        ps2.Localization  `json:"name"`
	Description ps2.Localization  `json:"description"`
	GeometryID  ps2.GeometryID    `json:"geometry_id,string"`
	Dynamic     stringNumericBool `json:"dynamic"`
}

func (Zone) CollectionName() string { return "zone" }

// UnmarshalJSON implements json.Unmarshaler
func (z *Zone) UnmarshalJSON(b []byte) error {
	type zone Zone // aliased type to prevent recursion
	var shadow zone
	if err := json.Unmarshal(b, &shadow); err != nil {
		return fmt.Errorf("census.Zone.UnmarshalJSON: %w", err)
	}

	if shadow.Dynamic {
		shadow.ContinentID = ps2.ContinentID(shadow.GeometryID)
	} else {
		shadow.ContinentID = ps2.ContinentID(shadow.ZoneID)
	}

	*z = Zone(shadow)
	return nil
}
