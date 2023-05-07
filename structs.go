package ps2

import (
	"encoding/json"
	"fmt"
	"strconv"
)

type RewardID int
type AchievementID int
type LoadoutID int
type ExperienceID int
type SkillID int
type VehicleID uint16
type FacilityID int
type MetagameEventStateID int
type CharacterID int64
type OutfitID int64

// ZoneInstanceID represents a (possibly) instanced Zone ID
// https://github.com/cooltrain7/Planetside-2-API-Tracker/wiki/Tutorial:-Zone-IDs
type ZoneInstanceID uint32

func (id ZoneInstanceID) String() string { return strconv.FormatInt(int64(id), 10) }

func (id ZoneInstanceID) ZoneID() ZoneID { return ZoneID(id & 0x0000FFFF) }

// Instance is an incrementing counter to differentiate zones with multiple instanced copies running.
//
// Instance is unique per server and resets when the server restarts.
func (id ZoneInstanceID) Instance() uint16  { return uint16(uint(id&0xFFFF0000) >> 16) }
func (id ZoneInstanceID) IsInstanced() bool { return id.Instance() != 0 }

type ZoneID uint16

func (z ZoneID) String() string { return strconv.Itoa(int(z)) }

// IsPermanentZone returns true for zones that are shown on the world map at all times.
func IsPermanentZone(z ZoneID) bool {
	switch z {
	case Amerish, Indar, Esamir, Hossin, Oshur:
		return true
	default:
		return false
	}
}

// IsPlayableZone returns true for zones that are playable, including special zones like those for outfit wars.
// It does not include non-combat zones like sanctuary or VR training.
func IsPlayableZone(z ZoneID) bool {
	switch z {
	case Amerish, Indar, Esamir, Hossin, Oshur, Koltyr, Desolation, Nexus:
		return true
	default:
		return false
	}
}

// WorldID is the ID for a server like Emerald, Cobalt, etc.
type WorldID int

func (w WorldID) String() string { return strconv.Itoa(int(w)) }

type ItemID int
type FactionID uint8

func (f FactionID) String() string {
	switch f {
	case FactionUnknown:
		return "Unknown"
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
type ItemType struct {
	ItemTypeID ItemTypeID `json:"item_type_id,string"`
	Name       string     `json:"name"`
	Code       string     `json:"code"`
}
type ItemCategoryID int

// InstanceID represents the metagame instance counter for a world.
type InstanceID int

// MetagameEventInstanceID represents a unique metagame event.
type MetagameEventInstanceID struct {
	WorldID
	InstanceID
}
type MetagameEventID int

var eventData = map[MetagameEventID]struct {
	name        string
	description string
	category    MetagameEventCategory
	faction     FactionID
	zone        ZoneID
}{
	147: {"Indar Superiority", "Control territory to lock Indar", Meltdown, TR, Indar},
	148: {"Indar Enlightenment", "Control territory to lock Indar", Meltdown, VS, Indar},
	149: {"Indar Liberation", "Control territory to lock Indar", Meltdown, NC, Indar},
	150: {"Esamir Superiority", "Control territory to lock Esamir", Meltdown, TR, Esamir},
	151: {"Esamir Enlightenment", "Control territory to lock Esamir", Meltdown, VS, Esamir},
	152: {"Esamir Liberation", "Control territory to lock Esamir", Meltdown, NC, Esamir},
	153: {"Hossin Superiority", "Control territory to lock Hossin", Meltdown, TR, Hossin},
	154: {"Hossin Enlightenment", "Control territory to lock Hossin", Meltdown, VS, Hossin},
	155: {"Hossin Liberation", "Control territory to lock Hossin", Meltdown, NC, Hossin},
	156: {"Amerish Superiority", "Control territory to lock Amerish", Meltdown, TR, Amerish},
	157: {"Amerish Enlightenment", "Control territory to lock Amerish", Meltdown, VS, 6},
	158: {"Amerish Liberation", "Control territory to lock Amerish", Meltdown, NC, 6},
	159: {"Amerish Warpgates Stabilizing", "Additional territories are coming back online.", 0, 0, Amerish},
	160: {"Esamir Warpgates Stabilizing", "Additional territories are coming back online.", 0, 0, Esamir},
	161: {"Indar Warpgates Stabilizing", "Additional territories are coming back online.", 0, 0, Indar},
	162: {"Hossin Warpgates Stabilizing", "Additional territories are coming back online.", 0, 0, Hossin},
	176: {"Esamir Unstable Meltdown", "Control territory to lock Esamir", UnstableMeltdown, NC, Esamir},
	177: {"Hossin Unstable Meltdown", "Control territory to lock Hossin", UnstableMeltdown, NC, Hossin},
	178: {"Amerish Unstable Meltdown", "Control territory to lock Amerish", UnstableMeltdown, NC, Amerish},
	179: {"Indar Unstable Meltdown", "Control territory to lock Indar", UnstableMeltdown, NC, Indar},
	186: {"Esamir Unstable Meltdown", "Control territory to lock Esamir", UnstableMeltdown, VS, Esamir},
	187: {"Hossin Unstable Meltdown", "Control territory to lock Hossin", UnstableMeltdown, VS, Hossin},
	188: {"Amerish Unstable Meltdown", "Control territory to lock Amerish", UnstableMeltdown, VS, Amerish},
	189: {"Indar Unstable Meltdown", "Control territory to lock Indar", UnstableMeltdown, VS, Indar},
	190: {"Esamir Unstable Meltdown", "Control territory to lock Esamir", UnstableMeltdown, TR, Esamir},
	191: {"Hossin Unstable Meltdown", "Control territory to lock Hossin", UnstableMeltdown, TR, Hossin},
	192: {"Amerish Unstable Meltdown", "Control territory to lock Amerish", UnstableMeltdown, TR, Amerish},
	193: {"Indar Unstable Meltdown", "Control territory to lock Indar", UnstableMeltdown, TR, Indar},
	204: {"OUTFIT WARS", "Capture Active Vanu Relics", 0, 0, 0},
	205: {"OUTFIT WARS (pre-match)", "Prepare for the Outfit War!", OutfitwarsPreMatch, 0, Nexus},
	206: {"OUTFIT WARS", "Active Relics Changing", 0, 0, 0},
	207: {"OUTFIT WARS", "Earn 750 points or have the most when time expires.", OutfitwarsMatch, 0, Nexus},
	208: {"Koltyr Liberation", "Control territory to lock Koltyr", KoltyrMeltdown, NC, Koltyr},
	209: {"Koltyr Superiority", "Control territory to lock Koltyr", KoltyrMeltdown, TR, Koltyr},
	210: {"Koltyr Enlightenment", "Control territory to lock Koltyr", KoltyrMeltdown, VS, Koltyr},
	211: {"Amerish Conquest", "Control territory to lock Amerish", Meltdown, 0, Amerish},
	212: {"Esamir Conquest", "Control territory to lock Esamir", Meltdown, 0, Esamir},
	213: {"Hossin Conquest", "Control territory to lock Hossin", Meltdown, 0, Hossin},
	214: {"Indar Conquest", "Control territory to lock Indar", Meltdown, 0, Indar},
	215: {"Koltyr Conquest", "Control territory to lock Koltyr", KoltyrMeltdown, 0, Koltyr},
	222: {"Oshur Liberation", "Control territory to lock Oshur", Meltdown, NC, Oshur},
	223: {"Oshur Superiority", "Control territory to lock Oshur", Meltdown, TR, Oshur},
	224: {"Oshur Enlightenment", "Control territory to lock Oshur", Meltdown, VS, Oshur},
	226: {"Oshur Conquest", "Control territory to lock Oshur", Meltdown, 0, Oshur},
	228: {"Indar Airial Anomalies", "Gather Tempest", AerialAnomalies, 0, Indar},
	229: {"Hossin Airial Anomalies", "Gather Tempest", AerialAnomalies, 0, Hossin},
	230: {"Amerish Airial Anomalies", "Gather Tempest", AerialAnomalies, 0, Amerish},
	231: {"Esamir Airial Anomalies", "Gather Tempest", AerialAnomalies, 0, Esamir},
	232: {"Oshur Airial Anomalies", "Gather Tempest", AerialAnomalies, 0, Oshur},
	236: {"Indar Sudden Death", "Kill as many enemies as possible", SuddenDeath, 0, Indar},
	237: {"Hossin Sudden Death", "Kill as many enemies as possible", SuddenDeath, 0, Hossin},
	238: {"Amerish Sudden Death", "Kill as many enemies as possible", SuddenDeath, 0, Amerish},
	239: {"Esamir Sudden Death", "Kill as many enemies as possible", SuddenDeath, 0, Esamir},
	240: {"Oshur Sudden Death", "Kill as many enemies as possible", SuddenDeath, 0, Oshur},
	242: {"Indar Forgotten Fleet Carrier", "Destroy the ghost ship", HauntedBastion, 0, Indar},
	243: {"Hossin Forgotten Fleet Carrier", "Destroy the ghost ship", HauntedBastion, 0, Hossin},
	244: {"Amerish Forgotten Fleet Carrier", "Destroy the ghost ship", HauntedBastion, 0, Amerish},
	245: {"Esamir Forgotten Fleet Carrier", "Destroy the ghost ship", HauntedBastion, 0, Esamir},
}

func (e MetagameEventID) Category() MetagameEventCategory {
	switch e {

	default:
		return UnknownEventCategory
	}
}

type MapRegionID int
type MapHexType uint8
type RewardCurrencyID int
type FacilityTypeID int
type FacilityType struct {
	FacilityTypeID FacilityTypeID `json:"facility_type_id,string"`
	Description    string         `json:"description"`
}
type MapRegion struct {
	Facility
	MapRegionID      RegionID         `json:"map_region_id,string"`
	ZoneID           ZoneID           `json:"zone_id,string"`
	LocationX        float64          `json:"location_x,string"`
	LocationY        float64          `json:"location_y,string"`
	LocationZ        float64          `json:"location_z,string"`
	RewardAmount     int              `json:"reward_amount"`
	RewardCurrencyID RewardCurrencyID `json:"reward_currency_id,string"`
}
type Facility struct {
	FacilityID     FacilityID     `json:"facility_id,string"`
	FacilityName   string         `json:"facility_name"`
	FacilityTypeID FacilityTypeID `json:"facility_type_id,string"`
	FacilityType   string         `json:"facility_type"`
}
type FacilityLink struct {
	ZoneID      ZoneID     `json:"zone_id"`
	FacilityIDA FacilityID `json:"facility_id_a"`
	FacilityIDB FacilityID `json:"facility_id_b"`
}
type RegionID int
type Region struct {
	RegionID         RegionID     `json:"region_id,string"`
	ZoneID           ZoneID       `json:"zone_id,string"`
	InitialFactionID FactionID    `json:"initial_faction_id,string"`
	Name             Localization `json:"name"`
}
type CurrencyID int
type Currency struct {
	CurrencyID   CurrencyID   `json:"currency_id,string"`
	Name         Localization `json:"name"`
	IconID       string       `json:"icon_id"`
	InventoryCap string       `json:"inventory_cap"`
}
type FireModeID int
