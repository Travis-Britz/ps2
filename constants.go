package ps2

const (
	None FactionID = 0
	VS   FactionID = 1
	NC   FactionID = 2
	TR   FactionID = 3
	NSO  FactionID = 4
)
const (
	Started      MetagameEventStateID = 135
	Restarted    MetagameEventStateID = 136
	Cancelled    MetagameEventStateID = 137
	Ended        MetagameEventStateID = 138
	BonusChanged MetagameEventStateID = 139
)

// https://census.daybreakgames.com/get/ps2:v2/metagame_event?c:limit=5000&c:distinct=type&c:lang=en&c:show=type&c:join=metagame_event^list:1^on:type^to:type^inject_at:metagame_events
const (
	conquest1       MetagameEventType = 1  // (deprecated?) Capture a continent
	facilityCapture MetagameEventType = 2  // (obsolete) Capture Tech Plants, Large Outposts, Biolabs, etc.
	survival        MetagameEventType = 4  // forgotten fleet carrier, nanite storm (?), outfit wars pre-match
	unknown5        MetagameEventType = 5  // (unused?) warpgates stabilizing, meteor shower (the bending)
	KillEnemies     MetagameEventType = 6  // maximum pressure, sudden death
	conquest8       MetagameEventType = 8  // (obsolete) hossin liberation, esamir enlightenment, etc. - all 45 minutes
	Meltdown        MetagameEventType = 9  // standard hossin liberation, esamir enlightenment. some 45 minutes
	EarnPoints      MetagameEventType = 10 // anomalies, refine and refuel, outfit wars
)

const (
	Indar       ContinentID = 2
	Hossin      ContinentID = 4
	Amerish     ContinentID = 6
	Esamir      ContinentID = 8
	Nexus       ContinentID = 10
	Extinction  ContinentID = 11
	Desolation2 ContinentID = 12
	Ascension   ContinentID = 13
	Koltyr      ContinentID = 14
	Oshur       ContinentID = 344
	Desolation  ContinentID = 361
	Sanctuary   ContinentID = 362
	Tutorial    ContinentID = 364
)

const (
	Connery  WorldID = 1
	Miller   WorldID = 10
	Cobalt   WorldID = 13
	Emerald  WorldID = 17
	Jaeger   WorldID = 19
	Apex     WorldID = 24
	Briggs   WorldID = 25
	SolTech  WorldID = 40
	Genudine WorldID = 1000
	Palos    WorldID = 1001
	Crux     WorldID = 1002
	Searhus  WorldID = 1003
	Xelas    WorldID = 1004
	Ceres    WorldID = 2000
	Lithcorp WorldID = 2001
	Rashnu   WorldID = 2002
)

const (
	PC    Environment = 0
	PS4US Environment = 1
	PS4EU Environment = 2
)

const (
	En Locale = "en"
	De Locale = "de"
	Es Locale = "es"
	Fr Locale = "fr"
	It Locale = "it"
	Tr Locale = "tr"
)

const (
	UnrestrictedHex      MapHexType = 0 // unrestricted access
	FullyRestrictedHex   MapHexType = 1 // no access
	FactionRestrictedHex MapHexType = 2 // Warpgates
)

const (
	type1          ImageTypeID = 1  // labeled "not yet used"
	type2          ImageTypeID = 2  // labeled "not yet used"
	ExtremelySmall ImageTypeID = 3  // 8px height
	VerySmall      ImageTypeID = 4  // 16px height
	Small          ImageTypeID = 5  // 32px height
	Medium         ImageTypeID = 6  // 64px height
	Large          ImageTypeID = 7  // 128px height
	VeryLarge      ImageTypeID = 8  // 256px height
	type9          ImageTypeID = 9  // labeled "not yet used" but has results
	Massive        ImageTypeID = 10 // no set height
)

const (
	DefaultFacility        FacilityTypeID = 1
	AmpStation             FacilityTypeID = 2
	Biolab                 FacilityTypeID = 3
	Techplant              FacilityTypeID = 4
	LargeOutpost           FacilityTypeID = 5
	SmallOutpost           FacilityTypeID = 6
	Warpgate               FacilityTypeID = 7
	Interlink              FacilityTypeID = 8
	ConstructionOutpost    FacilityTypeID = 9
	RelicOutpost           FacilityTypeID = 10 // Desolation
	ContainmentSite        FacilityTypeID = 11
	Trident                FacilityTypeID = 12
	Seapost                FacilityTypeID = 13
	LargeOutpostCTF        FacilityTypeID = 14
	SmallOutpostCTF        FacilityTypeID = 15
	AmpStationCTF          FacilityTypeID = 16
	ConstructionOutpostCTF FacilityTypeID = 17
)

const (
	InfiltratorNC  LoadoutID = 1
	LightAssaultNC LoadoutID = 3
	MedicNC        LoadoutID = 4
	EngineerNC     LoadoutID = 5
	HeavyAssaultNC LoadoutID = 6
	MaxNC          LoadoutID = 7

	InfiltratorTR  LoadoutID = 8
	LightAssaultTR LoadoutID = 10
	MedicTR        LoadoutID = 11
	EngineerTR     LoadoutID = 12
	HeavyAssaultTR LoadoutID = 13
	MaxTR          LoadoutID = 14

	InfiltratorVS  LoadoutID = 15
	LightAssaultVS LoadoutID = 17
	MedicVS        LoadoutID = 18
	EngineerVS     LoadoutID = 19
	HeavyAssaultVS LoadoutID = 20
	MaxVS          LoadoutID = 21

	InfiltratorNSO  LoadoutID = 28
	LightAssaultNSO LoadoutID = 29
	MedicNSO        LoadoutID = 30
	EngineerNSO     LoadoutID = 31
	HeavyAssaultNSO LoadoutID = 32
	MaxNSO          LoadoutID = 45
)

const (
	Infiltrator  ProfileTypeID = 1
	LightAssault ProfileTypeID = 3
	Medic        ProfileTypeID = 4
	Engineer     ProfileTypeID = 5
	HeavyAssault ProfileTypeID = 6
	Max          ProfileTypeID = 7
)

// Common vehicles
const (
	Flash     VehicleID = 1
	Sunderer  VehicleID = 2
	Lightning VehicleID = 3
	Magrider  VehicleID = 4
	Vanguard  VehicleID = 5
	Prowler   VehicleID = 6
	Scythe    VehicleID = 7
	Reaver    VehicleID = 8
	Mosquito  VehicleID = 9
	Liberator VehicleID = 10
	Galaxy    VehicleID = 11
	Harasser  VehicleID = 12
	Valkyrie  VehicleID = 14
	ANT       VehicleID = 15
	Colossus  VehicleID = 2007
	Bastion   VehicleID = 2019
	Javelin   VehicleID = 2033
	Dervish   VehicleID = 2136
	Chimera   VehicleID = 2137
	Corsair   VehicleID = 2142
)

// generate:
// jq -r '.[] | "\(.name) ExperienceAwardTypeID = \(.experience_award_type_id)"' experience_types.json
const (
	Kill                                    ExperienceAwardTypeID = 1
	KillAssist                              ExperienceAwardTypeID = 2
	SpawnKilllAssist                        ExperienceAwardTypeID = 3
	Heal                                    ExperienceAwardTypeID = 4
	HealAssist                              ExperienceAwardTypeID = 5
	Repair                                  ExperienceAwardTypeID = 6
	Revive                                  ExperienceAwardTypeID = 9
	KillStreak                              ExperienceAwardTypeID = 10
	DominationKill                          ExperienceAwardTypeID = 12
	RevengeKill                             ExperienceAwardTypeID = 13
	Achievement                             ExperienceAwardTypeID = 15
	DefendControlPoint                      ExperienceAwardTypeID = 18
	AttackControlPoint                      ExperienceAwardTypeID = 19
	ControlPointConverted                   ExperienceAwardTypeID = 21
	FacilityCaptured                        ExperienceAwardTypeID = 23
	FacilityDestroySecondaryObjective       ExperienceAwardTypeID = 25
	FacilityDestroySecondaryObjectiveAssist ExperienceAwardTypeID = 26
	MultipleKill                            ExperienceAwardTypeID = 28
	NemesisKill                             ExperienceAwardTypeID = 29
	Headshot                                ExperienceAwardTypeID = 30
	SpotKill                                ExperienceAwardTypeID = 31
	StopKillStreak                          ExperienceAwardTypeID = 32
	PlayerClassKill                         ExperienceAwardTypeID = 33
	PlayerSpawnAtVehicle                    ExperienceAwardTypeID = 34
	GunnerKill                              ExperienceAwardTypeID = 35
	DeployKill                              ExperienceAwardTypeID = 36
	RoadKill                                ExperienceAwardTypeID = 37
	Resupply                                ExperienceAwardTypeID = 38
	SquadHeal                               ExperienceAwardTypeID = 39
	SquadRepair                             ExperienceAwardTypeID = 40
	SquadRevive                             ExperienceAwardTypeID = 41
	SquadResupply                           ExperienceAwardTypeID = 42
	SquadSpotKill                           ExperienceAwardTypeID = 43
	SquadSpawn                              ExperienceAwardTypeID = 44
	FacilityPlacedBomb                      ExperienceAwardTypeID = 53
	FacilityDefusedBomb                     ExperienceAwardTypeID = 54
	XpHackedTerminal                        ExperienceAwardTypeID = 55
	XpHackedTurret                          ExperienceAwardTypeID = 56
	VehicleResupply                         ExperienceAwardTypeID = 57
	SquadVehicleResupply                    ExperienceAwardTypeID = 58
	SpawnKill                               ExperienceAwardTypeID = 59
	PriorityKill                            ExperienceAwardTypeID = 60
	HighPriorityKill                        ExperienceAwardTypeID = 61
	VehicleDamage                           ExperienceAwardTypeID = 62
	MetaGameEvent                           ExperienceAwardTypeID = 63
	MotionDetect                            ExperienceAwardTypeID = 64
	SquadMotionDetect                       ExperienceAwardTypeID = 65
	XpDespawn                               ExperienceAwardTypeID = 66
	SaviorKill                              ExperienceAwardTypeID = 67
	VehicleRadarKill                        ExperienceAwardTypeID = 69
	SquadVehicleRadarKill                   ExperienceAwardTypeID = 70
	PriorityKillAssist                      ExperienceAwardTypeID = 71
	HighPriorityKillAssist                  ExperienceAwardTypeID = 72
	VehicleShare                            ExperienceAwardTypeID = 73
	ResourceHeal                            ExperienceAwardTypeID = 74
	SquadResourceHeal                       ExperienceAwardTypeID = 75
	ExplosiveShare                          ExperienceAwardTypeID = 76
	SpecialGrenadeAssist                    ExperienceAwardTypeID = 77
	SpecialGrenadeSquadAssist               ExperienceAwardTypeID = 78
	ObjectivePulse                          ExperienceAwardTypeID = 79
	SquadKill                               ExperienceAwardTypeID = 80
	BountyKill                              ExperienceAwardTypeID = 81
	Membership                              ExperienceAwardTypeID = 82
	ResourceTransfer                        ExperienceAwardTypeID = 85
	DrawFire                                ExperienceAwardTypeID = 90
	GenericNpcSpawn                         ExperienceAwardTypeID = 92
	EQ20                                    ExperienceAwardTypeID = 97
	XpConstructionModuleInstallDefence      ExperienceAwardTypeID = 98
	XpConstructionModuleOverload            ExperienceAwardTypeID = 99
	XpConstructionModuleCounterOverload     ExperienceAwardTypeID = 100
	XpConstructionModuleDisarm              ExperienceAwardTypeID = 101
	XpConstructionModuleInstallAttack       ExperienceAwardTypeID = 102
	LuaScript                               ExperienceAwardTypeID = 108
	ConvoyEvent                             ExperienceAwardTypeID = 109
	Mission                                 ExperienceAwardTypeID = 110
	CtfDefendPodium                         ExperienceAwardTypeID = 111
	CtfFlagCaptured                         ExperienceAwardTypeID = 112
	CtfFlagReturned                         ExperienceAwardTypeID = 113
	XpHackedFlagRepo                        ExperienceAwardTypeID = 114
	CtfDefendRepo                           ExperienceAwardTypeID = 115
	XpHackOverload                          ExperienceAwardTypeID = 116
	XpCounterHackOverload                   ExperienceAwardTypeID = 117
)

const (
	// TR
	IndarSuperiority          MetagameEventID = 147
	EsamirSuperiority         MetagameEventID = 150
	HossinSuperiority         MetagameEventID = 153
	AmerishSuperiority        MetagameEventID = 156
	KoltyrSuperiority         MetagameEventID = 209
	OshurSuperiority          MetagameEventID = 223
	EsamirUnstableMeltdownTR  MetagameEventID = 190
	HossinUnstableMeltdownTR  MetagameEventID = 191
	AmerishUnstableMeltdownTR MetagameEventID = 192
	IndarUnstableMeltdownTR   MetagameEventID = 193
	OshurUnstableMeltdownTR   MetagameEventID = 250

	// VS
	IndarEnlightenment        MetagameEventID = 148
	EsamirEnlightenment       MetagameEventID = 151
	HossinEnlightenment       MetagameEventID = 154
	AmerishEnlightenment      MetagameEventID = 157
	KoltyrEnlightenment       MetagameEventID = 210
	OshurEnlightenment        MetagameEventID = 224
	EsamirUnstableMeltdownVS  MetagameEventID = 186
	HossinUnstableMeltdownVS  MetagameEventID = 187
	AmerishUnstableMeltdownVS MetagameEventID = 188
	IndarUnstableMeltdownVS   MetagameEventID = 189
	OshurUnstableMeltdownVS   MetagameEventID = 249

	// NC
	IndarLiberation           MetagameEventID = 149
	EsamirLiberation          MetagameEventID = 152
	HossinLiberation          MetagameEventID = 155
	AmerishLiberation         MetagameEventID = 158
	KoltyrLiberation          MetagameEventID = 208
	OshurLiberation           MetagameEventID = 222
	EsamirUnstableMeltdownNC  MetagameEventID = 176
	HossinUnstableMeltdownNC  MetagameEventID = 177
	AmerishUnstableMeltdownNC MetagameEventID = 178
	IndarUnstableMeltdownNC   MetagameEventID = 179
	OshurUnstableMeltdownNC   MetagameEventID = 248

	// Conquest (started by continent max population)
	AmerishConquest MetagameEventID = 211
	EsamirConquest  MetagameEventID = 212
	HossinConquest  MetagameEventID = 213
	IndarConquest   MetagameEventID = 214
	KoltyrConquest  MetagameEventID = 215
	OshurConquest   MetagameEventID = 226

	IndarAerialAnomalies   MetagameEventID = 228
	HossinAerialAnomalies  MetagameEventID = 229
	AmerishAerialAnomalies MetagameEventID = 230
	EsamirAerialAnomalies  MetagameEventID = 231
	OshurAerialAnomalies   MetagameEventID = 232

	IndarSuddenDeath   MetagameEventID = 236
	HossinSuddenDeath  MetagameEventID = 237
	AmerishSuddenDeath MetagameEventID = 238
	EsamirSuddenDeath  MetagameEventID = 239
	OshurSuddenDeath   MetagameEventID = 240

	IndarForgottenCarrier   MetagameEventID = 242
	HossinForgottenCarrier  MetagameEventID = 243
	AmerishForgottenCarrier MetagameEventID = 244
	EsamirForgottenCarrier  MetagameEventID = 245
	OshurForgottenCarrier   MetagameEventID = 246

	IndarNaniteStorm   MetagameEventID = 251
	HossinNaniteStorm  MetagameEventID = 252
	AmerishNaniteStorm MetagameEventID = 253
	EsamirNaniteStorm  MetagameEventID = 254
	OshurNaniteStorm   MetagameEventID = 255
)
