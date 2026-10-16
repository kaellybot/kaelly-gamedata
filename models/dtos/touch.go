package dtos

// TouchConfig is the client bootstrap file: where the data and the assets are served.
type TouchConfig struct {
	DataURL   string `json:"dataUrl"`
	AssetsURL string `json:"assetsUrl"`
}

type TouchItemType struct {
	ID          int32  `json:"id"`
	NameID      string `json:"nameId"`
	SuperTypeID int32  `json:"superTypeId"`
}

type TouchItem struct {
	Type                   string `json:"_type"`
	ID                     int32  `json:"id"`
	TypeID                 int32  `json:"typeId"`
	Level                  int32  `json:"level"`
	IconID                 int32  `json:"iconId"`
	NameID                 string `json:"nameId"`
	DescriptionID          string `json:"descriptionId"`
	RealWeight             int32  `json:"realWeight"`
	Criteria               string `json:"criteria"`
	ApCost                 int32  `json:"apCost"`
	MinRange               int32  `json:"minRange"`
	Range                  int32  `json:"range"`
	MaxCastPerTurn         int32  `json:"maxCastPerTurn"`
	CriticalHitProbability int32  `json:"criticalHitProbability"`
	CriticalHitBonus       int32  `json:"criticalHitBonus"`
	TwoHanded              bool   `json:"twoHanded"`
}

type TouchAlmanaxCalendar struct {
	ID     int32  `json:"id"`
	NameID string `json:"nameId"`
	DescID string `json:"descId"`
	NpcID  int32  `json:"npcId"`
}

// TouchTables holds the client tables of one run, per language.
type TouchTables struct {
	ItemTypes        map[string]map[string]TouchItemType
	Items            map[string]map[string]TouchItem
	AlmanaxCalendars map[string]map[string]TouchAlmanaxCalendar
}
