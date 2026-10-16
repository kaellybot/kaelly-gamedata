package gamedataimports

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

// GetLast returns the most recent import of a source with one of the statuses, with its files.
func (repo *Impl) GetLast(game amqp.Game, source entities.ImportSource, statuses []entities.ImportStatus,
) (entities.GameDataImport, bool, error) {
	var gameDataImports []entities.GameDataImport
	err := repo.db.GetDB().
		Preload("Files").
		Where("game = ? AND source = ? AND status IN ?", game, source, statuses).
		Order("started_at DESC, id DESC").
		Limit(1).
		Find(&gameDataImports).Error
	if err != nil || len(gameDataImports) == 0 {
		return entities.GameDataImport{}, false, err
	}

	return gameDataImports[0], true, nil
}

// GetObjectKeys returns the raw snapshot keys referenced by the last imports of a source.
func (repo *Impl) GetObjectKeys(game amqp.Game, source entities.ImportSource,
	statuses []entities.ImportStatus, limit int) ([]string, error) {
	var importIDs []int64
	err := repo.db.GetDB().Model(&entities.GameDataImport{}).
		Where("game = ? AND source = ? AND status IN ?", game, source, statuses).
		Order("started_at DESC, id DESC").
		Limit(limit).
		Pluck("id", &importIDs).Error
	if err != nil || len(importIDs) == 0 {
		return nil, err
	}

	var keys []string
	err = repo.db.GetDB().Model(&entities.GameDataImportFile{}).
		Where("import_id IN ?", importIDs).
		Distinct().
		Pluck("object_key", &keys).Error
	return keys, err
}

// Save creates or updates the import, then its files.
func (repo *Impl) Save(tx *gorm.DB, gameDataImport *entities.GameDataImport) error {
	if err := tx.Omit(clause.Associations).Save(gameDataImport).Error; err != nil {
		return err
	}

	if len(gameDataImport.Files) == 0 {
		return nil
	}

	for i := range gameDataImport.Files {
		gameDataImport.Files[i].ImportID = gameDataImport.ID
	}

	return tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&gameDataImport.Files).Error
}
