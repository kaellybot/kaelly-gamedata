package gameversions

import (
	amqp "github.com/kaellybot/kaelly-amqp"
	"github.com/kaellybot/kaelly-gamedata/models/entities"
	"github.com/kaellybot/kaelly-gamedata/utils/databases"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func New(db databases.MySQLConnection) *Impl {
	return &Impl{db: db}
}

func (repo *Impl) Get(game amqp.Game) (entities.GameVersion, bool, error) {
	var gameVersions []entities.GameVersion
	err := repo.db.GetDB().Where("id = ?", game).Limit(1).Find(&gameVersions).Error
	if err != nil || len(gameVersions) == 0 {
		return entities.GameVersion{}, false, err
	}

	return gameVersions[0], true, nil
}

func (repo *Impl) Save(tx *gorm.DB, gameVersion entities.GameVersion) error {
	return tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&gameVersion).Error
}
