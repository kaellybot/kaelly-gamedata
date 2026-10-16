package gamedataimports

import (
	amqp "github.com/kaellybot/kaelly-amqp"
	"github.com/kaellybot/kaelly-gamedata/models/entities"
	"github.com/kaellybot/kaelly-gamedata/utils/databases"
	"gorm.io/gorm"
)

type Repository interface {
	GetLast(game amqp.Game, source entities.ImportSource, statuses []entities.ImportStatus,
	) (entities.GameDataImport, bool, error)
	GetObjectKeys(game amqp.Game, source entities.ImportSource, statuses []entities.ImportStatus,
		limit int) ([]string, error)
	Save(tx *gorm.DB, gameDataImport *entities.GameDataImport) error
}

type Impl struct {
	db databases.MySQLConnection
}
