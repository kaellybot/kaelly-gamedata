package entities

import amqp "github.com/kaellybot/kaelly-amqp"

// GameVersion is the release currently applied for a game, read by the encyclopedia.
type GameVersion struct {
	ID      amqp.Game `gorm:"primaryKey;autoIncrement:false"`
	Version string
}
