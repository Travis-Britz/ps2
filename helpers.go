package ps2

// InfantryProfileTypeID is a convenience function for checking a player's infantry class.
func InfantryType(l LoadoutID) ProfileTypeID {
	switch l {
	case LightAssaultNC, LightAssaultTR, LightAssaultVS, LightAssaultNSO:
		return LightAssault
	case InfiltratorNC, InfiltratorTR, InfiltratorVS, InfiltratorNSO:
		return Infiltrator
	case MedicNC, MedicTR, MedicVS, MedicNSO:
		return Medic
	case HeavyAssaultNC, HeavyAssaultTR, HeavyAssaultVS, HeavyAssaultNSO:
		return HeavyAssault
	case EngineerNC, EngineerTR, EngineerVS, EngineerNSO:
		return Engineer
	case MaxNC, MaxTR, MaxVS, MaxNSO:
		return Max
	default:
		return 0
	}
}

// IsPermanentZone returns true for zones that are shown on the world map at all times.
func IsPermanentZone(z ContinentID) bool {
	switch z {
	case Amerish, Indar, Esamir, Hossin, Oshur:
		return true
	default:
		return false
	}
}

// IsPlayableZone returns true for zones that are playable, including special zones like those for outfit wars.
// It does not include non-combat zones like sanctuary or VR training.
func IsPlayableZone(z ContinentID) bool {
	switch z {
	case Amerish, Indar, Esamir, Hossin, Oshur, Koltyr, Desolation, Nexus:
		return true
	default:
		return false
	}
}

// IsHiddenWorld returns true for worlds that should be hidden from selection menus.
// These worlds may be permanently locked, unavailable, or otherwise inaccessible for live gameplay.
func IsHiddenWorld(w WorldID) bool {
	switch w {
	// hidden PC worlds
	case Jaeger, Apex:
		return true

		// hidden PS4 worlds
	case Palos, Crux, Searhus, Xelas, Lithcorp, Rashnu:
		return true
	default:
		return false
	}
}

func GetEnvironment(w WorldID) Environment {
	switch w {
	case Ceres, Lithcorp, Rashnu:
		return PS4EU
	case Genudine, Palos, Crux, Searhus, Xelas:
		return PS4US
	default:
		return PC
	}
}

// StartingFaction returns the faction that triggered an alert to start.
func StartingFaction(id MetagameEventID) FactionID {
	return eventDataSupplementTable[id].faction
}

// IsContinentLock returns true for metagame events that will lock a continent.
func IsContinentLock(id MetagameEventID) bool {
	return eventDataSupplementTable[id].locksContinent
}

func IsTerritoryAlert(id MetagameEventID) bool {
	if result, found := eventDataSupplementTable[id]; found {
		return result.isTerritory
	}

	// I would like to check the event type here as a fallback for unknown IDs,
	// (and return true for type 6)
	// but I don't have direct access to that data from this function.
	// todo: change function signature to take a type,
	// or return an error if an exact answer is not known?
	return false
}

// eventDataSupplementTable supplements metagame event data with fields that the official census api doesn't include.
var eventDataSupplementTable = map[MetagameEventID]struct {
	faction        FactionID
	locksContinent bool
	isTerritory    bool
}{
	IndarSuperiority:          {TR, true, true},
	IndarEnlightenment:        {VS, true, true},
	IndarLiberation:           {NC, true, true},
	EsamirSuperiority:         {TR, true, true},
	EsamirEnlightenment:       {VS, true, true},
	EsamirLiberation:          {NC, true, true},
	HossinSuperiority:         {TR, true, true},
	HossinEnlightenment:       {VS, true, true},
	HossinLiberation:          {NC, true, true},
	AmerishSuperiority:        {TR, true, true},
	AmerishEnlightenment:      {VS, true, true},
	AmerishLiberation:         {NC, true, true},
	159:                       {None, false, false}, // "Amerish Warpgates Stabilizing" - not implemented?
	160:                       {None, false, false}, // "Esamir Warpgates Stabilizing"
	161:                       {None, false, false}, // "Indar Warpgates Stabilizing"
	162:                       {None, false, false}, // "Hossin Warpgates Stabilizing"
	EsamirUnstableMeltdownNC:  {NC, true, true},
	HossinUnstableMeltdownNC:  {NC, true, true},
	AmerishUnstableMeltdownNC: {NC, true, true},
	IndarUnstableMeltdownNC:   {NC, true, true},
	EsamirUnstableMeltdownVS:  {VS, true, true},
	HossinUnstableMeltdownVS:  {VS, true, true},
	AmerishUnstableMeltdownVS: {VS, true, true},
	IndarUnstableMeltdownVS:   {VS, true, true},
	EsamirUnstableMeltdownTR:  {TR, true, true},
	HossinUnstableMeltdownTR:  {TR, true, true},
	AmerishUnstableMeltdownTR: {TR, true, true},
	IndarUnstableMeltdownTR:   {TR, true, true},
	204:                       {None, false, false}, // "OUTFIT WARS", "Capture Active Vanu Relics"
	205:                       {None, false, false}, // "OUTFIT WARS (pre-match)", "Prepare for the Outfit War!"
	206:                       {None, false, false}, // "OUTFIT WARS", "Active Relics Changing"
	207:                       {None, false, false}, // "OUTFIT WARS", "Earn 750 points or have the most when time expires."
	KoltyrLiberation:          {NC, true, true},
	KoltyrSuperiority:         {TR, true, true},
	KoltyrEnlightenment:       {VS, true, true},
	AmerishConquest:           {None, true, true},
	EsamirConquest:            {None, true, true},
	HossinConquest:            {None, true, true},
	IndarConquest:             {None, true, true},
	KoltyrConquest:            {None, true, true},
	OshurLiberation:           {NC, true, true},
	OshurSuperiority:          {TR, true, true},
	OshurEnlightenment:        {VS, true, true},
	OshurConquest:             {None, true, true},
	IndarAerialAnomalies:      {None, false, false},
	HossinAerialAnomalies:     {None, false, false},
	AmerishAerialAnomalies:    {None, false, false},
	EsamirAerialAnomalies:     {None, false, false},
	OshurAerialAnomalies:      {None, false, false},
	IndarSuddenDeath:          {None, true, false},
	HossinSuddenDeath:         {None, true, false},
	AmerishSuddenDeath:        {None, true, false},
	EsamirSuddenDeath:         {None, true, false},
	OshurSuddenDeath:          {None, true, false},
	IndarForgottenCarrier:     {None, false, false},
	HossinForgottenCarrier:    {None, false, false},
	AmerishForgottenCarrier:   {None, false, false},
	EsamirForgottenCarrier:    {None, false, false},
	OshurForgottenCarrier:     {None, false, false},
	OshurUnstableMeltdownNC:   {NC, true, true},
	OshurUnstableMeltdownVS:   {VS, true, true},
	OshurUnstableMeltdownTR:   {TR, true, true},
}
