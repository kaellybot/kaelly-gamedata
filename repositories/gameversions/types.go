package gameversions

import (
	amqp "github.com/kaellybot/kaelly-amqp"
	"github.com/kaellybot/kaelly-gamedata/models/entities"
	"github.com/kaellybot/kaelly-gamedata/utils/databases"
	"gorm.io/gorm"
)

type Repository interface {
	Get(game amqp.Game) (entities.GameVersion, bool, error)
	Save(tx *gorm.DB, gameVersion entities.GameVersion) error
}

type Impl struct {
	db databases.MySQLConnection
}
