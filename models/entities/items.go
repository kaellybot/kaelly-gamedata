package entities

import amqp "github.com/kaellybot/kaelly-amqp"

type ItemType struct {
	ID            int32     `gorm:"primaryKey;autoIncrement:false"`
	Game          amqp.Game `gorm:"primaryKey;autoIncrement:false"`
	SuperTypeID   int32
	AmqpItemType  *amqp.ItemType      `gorm:"column:item_type"`
	EquipmentType *amqp.EquipmentType `gorm:"column:equipment_type"`
	ContentHash   string              `json:"-"`
	Labels        []ItemTypeLabel     `gorm:"-:all"`
}

type ItemTypeLabel struct {
	ItemTypeID int32         `gorm:"primaryKey;autoIncrement:false"`
	Game       amqp.Game     `gorm:"primaryKey;autoIncrement:false"`
	Locale     amqp.Language `gorm:"primaryKey;autoIncrement:false"`
	Label      string
}

type Item struct {
	ID             int32     `gorm:"primaryKey;autoIncrement:false"`
	Game           amqp.Game `gorm:"primaryKey;autoIncrement:false"`
	TypeID         int32
	Level          int32
	IconID         int32
	Pods           int32
	Criteria       *string
	Conditions     *string `gorm:"type:json"`
	ApCost         *int32
	MinRange       *int32
	MaxRange       *int32
	MaxCastPerTurn *int32
	CriticalRate   *int32
	CriticalBonus  *int32
	TwoHanded      *bool
	ContentHash    string      `json:"-"`
	Labels         []ItemLabel `gorm:"-:all"`
}

type ItemLabel struct {
	ItemID      int32         `gorm:"primaryKey;autoIncrement:false"`
	Game        amqp.Game     `gorm:"primaryKey;autoIncrement:false"`
	Locale      amqp.Language `gorm:"primaryKey;autoIncrement:false"`
	Name        string
	Description *string
}
