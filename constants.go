package ps2

const (
	FactionUnknown FactionID = iota
	VS
	NC
	TR
	NSO
)
const (
	MetagameEventStarted      MetagameEventStateID = 135
	MetagameEventRestarted    MetagameEventStateID = 136
	MetagameEventCancelled    MetagameEventStateID = 137
	MetagameEventEnded        MetagameEventStateID = 138
	MetagameEventBonusChanged MetagameEventStateID = 139
)

const (
	UnknownEventCategory MetagameEventCategory = iota
	Meltdown
	UnstableMeltdown
	KoltyrMeltdown
	MaximumPressure
	SuddenDeath
	AerialAnomalies
	OutfitwarsPreMatch
	OutfitwarsMatch
	HauntedBastion
)

const (
	Indar      ZoneID = 2
	Hossin     ZoneID = 4
	Amerish    ZoneID = 6
	Esamir     ZoneID = 8
	Nexus      ZoneID = 10
	Koltyr     ZoneID = 14
	Oshur      ZoneID = 344
	Desolation ZoneID = 361
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
	EnvPC Environment = iota
	EnvPS4US
	EnvPS4EU
)
