package ps2

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strconv"
)

var DefaultLocale Locale = En

type RewardID int
type AchievementID int
type ExperienceID int
type ExperienceAwardTypeID int
type SkillID int
type VehicleID uint16

func (v VehicleID) GoString() string {
	switch v {
	case Flash:
		return "ps2.Flash"
	case Sunderer:
		return "ps2.Sunderer"
	case Lightning:
		return "ps2.Lightning"
	case Magrider:
		return "ps2.Magrider"
	case Vanguard:
		return "ps2.Vanguard"
	case Prowler:
		return "ps2.Prowler"
	case Scythe:
		return "ps2.Scythe"
	case Reaver:
		return "ps2.Reaver"
	case Mosquito:
		return "ps2.Mosquito"
	case Liberator:
		return "ps2.Liberator"
	case Galaxy:
		return "ps2.Galaxy"
	case Harasser:
		return "ps2.Harasser"
	case Valkyrie:
		return "ps2.Valkyrie"
	case ANT:
		return "ps2.ANT"
	case Colossus:
		return "ps2.Colossus"
	case Bastion:
		return "ps2.Bastion"
	case Javelin:
		return "ps2.Javelin"
	case Dervish:
		return "ps2.Dervish"
	case Chimera:
		return "ps2.Chimera"
	case Corsair:
		return "ps2.Corsair"
	default:
		return fmt.Sprintf("%d", int(v))
	}
}

type FacilityID int
type MetagameEventStateID int

// CharacterID is a character's ID (duh).
// IDs are definitely 64-bit types,
// but it is unclear whether they are signed or unsigned in the game's database.
// No existing IDs use the 64th/leftmost/highest bit,
// but nearly all use the 63rd (leftmost/highest positive signed bit is 1).
// The characters database is known to be backed by a cluster of at least 20 databases,
// and the leftmost/highest N bits in a character ID appear to be a bitmasked database shard ID.
// (The rightmost/lowest bit is always 1 - see [EntityID].)
// This is only relevant in the sense that,
// out of the current existing shards,
// none of them appear to have an ID that would set the signed bit to 1,
// and it is unlikely for new shards to be introduced at any point in the future.
// Therefore even if character IDs were backed by a signed integer type internally,
// they could never actually go negative.
type CharacterID uint64

func (id CharacterID) String() string   { return strconv.FormatUint(uint64(id), 10) }
func (id CharacterID) GoString() string { return strconv.FormatUint(uint64(id), 10) }

// NPCID is a non-globally unique NPC ID such as a spawned sunderer, construction object, beacon, and many other game objects.
// An ID is unique as long as the object is alive,
// but once the object dies the ID may be re-used after an unknown amount of time.
// NPCIDs are bitmasked; the rightmost four (?) bits may have special meaning for vehicle categorization.
// The rules or categories are unknown and may not be useful.
type NPCID uint64

func (id NPCID) GoString() string { return strconv.FormatUint(uint64(id), 10) }

// EntityID represents a game entity: either a CharacterID or NPCID.
// This type is used primarily for GainExperience events from the Planetside 2 event streaming API in the "other_id" field.
type EntityID uint64

func (id EntityID) GoString() string {
	if id == 0 {
		return "0"
	}
	idd, _ := id.ID()
	switch (idd).(type) {
	// this formatting is really verbose,
	// but I want the output to be a valid Go representation while still showing the type of entity
	case CharacterID:
		return fmt.Sprintf("ps2.EntityID(ps2.CharacterID(%d))", id)
	case NPCID:
		return fmt.Sprintf("ps2.EntityID(ps2.NPCID(%d))", id)
	}

	return strconv.FormatUint(uint64(id), 10)
}

// ID returns either a CharacterID or NPCID if set is true, and nil if set is false.
// The result must be type checked.
func (e EntityID) ID() (id any, set bool) {
	if e == 0 {
		return nil, false
	}
	// even numbers are NPC IDs
	if e%2 == 0 {
		return NPCID(e), true
	}
	// odd numbers are Character IDs
	return CharacterID(e), true
}

type OutfitID int64

// ContinentID is a pseudo-ID type that represents either a ZoneID or GeometryID.
// It is more of a conceptual type of ID than a direct implementation.
// It does not exist as a distinct type anywhere in the game or in the census API,
// but it is effectively the "true type" of ID given in census events.
//
// ContinentID is the type that should be used when storing/querying a local database.
//
// There is some complexity between ContinentID, ZoneID, ZoneInstanceID, and GeometryID.
// The code is complex because the reality is complex.
// The biggest problem is terminology,
// but also because there are at least three representations of Zone ID used by the game.
//
// The census API zone collection is unique by zone_id,
// but it is not technically unique by geometry_id.
// GeometryID refers to the terrain mesh being used on a map.
//
// When the realtime events API spits out a zone_id in an event,
// that zone_id field is what we refer to as a ZoneInstanceID in this package.
// ZoneInstanceID is a bitmasked ID containing the ID for the map (either ZoneID or Geometry, depending) and possibly an instance counter.
// When the zone is a static zone (Hossin, Indar, etc.) then that ID is the zone_id as reported by Census.
// When the zone is a dynamic zone (Desolation, Nexus, Koltyr, etc.) then that ID is a bitmasked field of an internal incrementing instance counter
// along with the zone's geometry_id as reported by Census.
// The realtime "/map" census endpoint is one of the few places that requires this ephemeral ZoneInstanceID directly.
//
// All of this is to say that, annoyingly,
// there is no unique ID that we can use to look up a zone when an event comes in,
// not without combining ZoneID and GeometryID into a distinct surrogate/pseudo ID.
// Functions that expect a ZoneID would break if given a GeometryID.
// Even if a GeometryID were cast to a ZoneID,
// the values would be wrong.
// Therefore this type is needed to maintain type safety in the values being passed between functions.
//
// # Conversions
//
// Conversions cannot go both ways.
// To go from ZoneID (or GeometryID) to a ContinentID, the "dynamic" attribute of a zone must be known.
// The only way to convert back from a ContinentID to ZoneID (or GeometryID) is by storing a lookup table.
// A ContinentID cannot, for instance, be used to query the census API because we don't know whether to query by zone_id or geometry_id.
// ZoneInstanceID can convert itself to ZoneID, GeometryID, or ContinentID because it has the dynamic property embedded.
// None of the three can convert back to a ZoneInstanceID because the ephemeral instance counter is lost.
type ContinentID uint16

func (c ContinentID) String() string { return strconv.Itoa(int(c)) }
func (c ContinentID) GoString() string {
	switch c {
	case Indar:
		return "ps2.Indar"
	case Hossin:
		return "ps2.Hossin"
	case Amerish:
		return "ps2.Amerish"
	case Esamir:
		return "ps2.Esamir"
	case Nexus:
		return "ps2.Nexus"
	case Extinction:
		return "ps2.Extinction"
	case Desolation2:
		return "ps2.Desolation2"
	case Ascension:
		return "ps2.Ascension"
	case Koltyr:
		return "ps2.Koltyr"
	case Oshur:
		return "ps2.Oshur"
	case Desolation:
		return "ps2.Desolation"
	case Sanctuary:
		return "ps2.Sanctuary"
	case Tutorial:
		return "ps2.Tutorial"
	default:
		return strconv.Itoa(int(c))
	}
}

// ZoneID is the ID used internally by the game to identify a zone like Sanctuary, Hossin, VR Training, etc.
// See the docs for [ContinentID].
type ZoneID uint16

func (z ZoneID) String() string   { return strconv.Itoa(int(z)) }
func (z ZoneID) GoString() string { return strconv.Itoa(int(z)) }

// GeometryID represents a zone mesh and is found in the census zone collection.
// See the docs for [ContinentID].
type GeometryID uint16

func (g GeometryID) GoString() string { return strconv.Itoa(int(g)) }
func (g GeometryID) String() string   { return strconv.Itoa(int(g)) }

// ZoneInstanceID represents a (possibly) instanced Continent ID.
//
// When the instance counter is set,
// the identifier given will be a GeometryID for a dynamic zone.
// When the instance counter is zero,
// the identifier given will be a ZoneID for a static zone.
// https://github.com/cooltrain7/Planetside-2-API-Tracker/wiki/Tutorial:-Zone-IDs
type ZoneInstanceID uint32

func (id ZoneInstanceID) String() string {
	return strconv.FormatInt(int64(id), 10)
}

// ZoneID returns the continent ID
func (id ZoneInstanceID) ZoneID() ContinentID { return ContinentID(id & 0x0000FFFF) }

// Instance is an incrementing counter to differentiate zones with multiple instanced copies running.
//
// Instance is unique per server and resets when the server restarts.
func (id ZoneInstanceID) Instance() uint16  { return uint16(uint(id&0xFFFF0000) >> 16) }
func (id ZoneInstanceID) IsInstanced() bool { return id.Instance() != 0 }

// DefinitionID offers a more precise way to check which ID is used for a zone,
// but at the cost of needing to type check the result to determine whether it is a GeometryID or ZoneID.
// The term Definition ID comes from the linked github wiki under [ZoneInstanceID].
func (id ZoneInstanceID) DefinitionID() any {
	if id.IsInstanced() {
		return GeometryID(id.ZoneID())
	}
	return ZoneID(id.ZoneID())
}

func (id ZoneInstanceID) GoString() string {
	if id.IsInstanced() {
		return fmt.Sprintf("ps2.ZoneInstanceID(%d<<16|ps2.GeometryID(%d))", id.Instance(), id.DefinitionID())
	}
	return fmt.Sprintf("ps2.ZoneInstanceID(%s)", id.ZoneID().GoString())
}

// WorldID is the ID for a server like Emerald, Cobalt, etc.
type WorldID uint16

func (w WorldID) String() string { return strconv.Itoa(int(w)) }
func (w WorldID) GoString() string {
	switch w {
	case Connery:
		return "ps2.Connery"
	case Miller:
		return "ps2.Miller"
	case Cobalt:
		return "ps2.Cobalt"
	case Emerald:
		return "ps2.Emerald"
	case Jaeger:
		return "ps2.Jaeger"
	case Apex:
		return "ps2.Apex"
	case Briggs:
		return "ps2.Briggs"
	case SolTech:
		return "ps2.SolTech"
	case Genudine:
		return "ps2.Genudine"
	case Palos:
		return "ps2.Palos"
	case Crux:
		return "ps2.Crux"
	case Searhus:
		return "ps2.Searhus"
	case Xelas:
		return "ps2.Xelas"
	case Ceres:
		return "ps2.Ceres"
	case Lithcorp:
		return "ps2.Lithcorp"
	case Rashnu:
		return "ps2.Rashnu"
	default:
		return fmt.Sprintf("WorldID(%d)", int(w))
	}
}

type ItemID int
type FactionID uint8

func (f FactionID) GoString() string {
	switch f {
	case None:
		return "ps2.None"
	case VS:
		return "ps2.VS"
	case NC:
		return "ps2.NC"
	case TR:
		return "ps2.TR"
	case NSO:
		return "ps2.NSO"
	default:
		return fmt.Sprintf("ps2.FactionID(%d)", int(f))
	}
}

func (f FactionID) String() string {
	switch f {
	case None:
		return "None"
	case VS:
		return "VS"
	case NC:
		return "NC"
	case TR:
		return "TR"
	case NSO:
		return "NSO"
	default:
		return "Undefined"
	}
}

// UnmarshalJSON implements json.Unmarshaler.
//
// The default unmarshaling behavior would normally be enough,
// but FactionID being used as an array index in multiple locations might cause a panic if an out of range value were somehow returned.
func (id *FactionID) UnmarshalJSON(data []byte) error {
	var i uint8
	if err := json.Unmarshal(data, &i); err != nil {
		return fmt.Errorf("ps2.FactionID.UnmarshalJSON: %w", err)
	}
	if i > uint8(NSO) {
		return fmt.Errorf("ps2.FactionID.UnmarshalJSON: value '%d' is out of range for FactionID", i)
	}
	*id = FactionID(i)
	return nil
}

type ItemTypeID int

type ItemCategoryID int

// InstanceID is an incrementing ID counter for a metagame event,
// and is unique within a world.
type InstanceID uint32

// MetagameEventInstanceID represents a unique metagame event.
//
// https://census.daybreakgames.com/get/ps2:v2/world_event?type=METAGAME
type MetagameEventInstanceID struct {
	WorldID
	InstanceID
}

func (i MetagameEventInstanceID) String() string {
	return fmt.Sprintf("%d-%d", i.WorldID, i.InstanceID)
}
func (i *MetagameEventInstanceID) UnmarshalJSON(b []byte) (err error) {
	b = bytes.Trim(b, "\"")
	*i, err = parseInstance(b)
	if err != nil {
		return fmt.Errorf("parse error: %w", err)
	}
	return nil
}

func (i *MetagameEventInstanceID) Scan(src any) (err error) {
	var v []byte
	switch src.(type) {
	case string:
		v = []byte(src.(string))
	case []byte:
		v = src.([]byte)
	default:
		return fmt.Errorf("Scan: unhandled type '%T'", src)
	}
	if *i, err = parseInstance([]byte(v)); err != nil {
		return fmt.Errorf("parse error: %w", err)
	}
	return nil
}
func parseInstance(b []byte) (i MetagameEventInstanceID, err error) {
	world, instance, found := bytes.Cut(b, []byte("-"))
	if !found {
		return i, fmt.Errorf("missing separator in instance id '%s'", b)
	}
	var worldid WorldID
	if err := json.Unmarshal(world, &worldid); err != nil {
		return i, fmt.Errorf("error unmarshaling world: %w", err)
	}
	var instanceid InstanceID
	if err := json.Unmarshal(instance, &instanceid); err != nil {
		return i, fmt.Errorf("error unmarshaling instance: %w", err)
	}
	i.WorldID = worldid
	i.InstanceID = instanceid
	return i, nil
}
func (i MetagameEventInstanceID) Value() (driver.Value, error) {
	return i.String(), nil
}
func (i MetagameEventInstanceID) MarshalJSON() (json []byte, err error) {
	json = append(json, '"')
	json = append(json, i.String()...)
	json = append(json, '"')
	return json, nil
}

// MetagameEventID represents type type of event,
// such as Amerish Liberation, Indar Aerial Anomalies, Amerish Forgotten Fleet Carrier, etc.
type MetagameEventID int

// MetagameEventType seems to be the win condition or scoring mechanism.
// It is not directly useful most of the time,
// but might be useful as a fallback for checking the alert type if new metagame events are added.
// Known types are listed in constants.go.
type MetagameEventType int

type MapHexType uint8

func (t MapHexType) GoString() string {
	return fmt.Sprintf("%d", int(t))
}

type RewardCurrencyID int
type FacilityTypeID int

type CurrencyID int
type Currency struct {
	CurrencyID   CurrencyID   `json:"currency_id,string"`
	Name         Localization `json:"name"`
	IconID       string       `json:"icon_id"`
	InventoryCap string       `json:"inventory_cap"`
}
type FireModeID int
type RegionID int

type Locale string
type Localization map[Locale]string

// Set will set the value of the default locale to s.
func (l *Localization) Set(s string) {
	if *l == nil {
		*l = make(Localization)
	}
	(*l)[DefaultLocale] = s
}

func (l Localization) String() string { return l[DefaultLocale] }

type ResourceID int
type ObjectiveGroupID int
type ArmorInfoID int
type ArmorFacingID int
type ImageID int
type ImageSetID int
type ImageTypeID int
type LoadoutID int

func (l LoadoutID) GoString() string {
	switch l {
	case InfiltratorNC:
		return "ps2.InfiltratorNC"
	case LightAssaultNC:
		return "ps2.LightAssaultNC"
	case MedicNC:
		return "ps2.MedicNC"
	case EngineerNC:
		return "ps2.EngineerNC"
	case HeavyAssaultNC:
		return "ps2.HeavyAssaultNC"
	case MaxNC:
		return "ps2.MaxNC"
	case InfiltratorTR:
		return "ps2.InfiltratorTR"
	case LightAssaultTR:
		return "ps2.LightAssaultTR"
	case MedicTR:
		return "ps2.MedicTR"
	case EngineerTR:
		return "ps2.EngineerTR"
	case HeavyAssaultTR:
		return "ps2.HeavyAssaultTR"
	case MaxTR:
		return "ps2.MaxTR"
	case InfiltratorVS:
		return "ps2.InfiltratorVS"
	case LightAssaultVS:
		return "ps2.LightAssaultVS"
	case MedicVS:
		return "ps2.MedicVS"
	case EngineerVS:
		return "ps2.EngineerVS"
	case HeavyAssaultVS:
		return "ps2.HeavyAssaultVS"
	case MaxVS:
		return "ps2.MaxVS"
	case InfiltratorNSO:
		return "ps2.InfiltratorNSO"
	case LightAssaultNSO:
		return "ps2.LightAssaultNSO"
	case MedicNSO:
		return "ps2.MedicNSO"
	case EngineerNSO:
		return "ps2.EngineerNSO"
	case HeavyAssaultNSO:
		return "ps2.HeavyAssaultNSO"
	case MaxNSO:
		return "ps2.MaxNSO"
	default:
		return fmt.Sprintf("ps2.LoadoutID(%d)", int(l))
	}
}

type ProfileID int
type ProfileTypeID int
