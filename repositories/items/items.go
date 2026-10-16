package items

import (
	amqp "github.com/kaellybot/kaelly-amqp"
	"github.com/kaellybot/kaelly-gamedata/models/entities"
	"github.com/kaellybot/kaelly-gamedata/utils/databases"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type idHash struct {
	ID          int32
	ContentHash string
}

func New(db databases.MySQLConnection) *Impl {
	return &Impl{db: db}
}

func (repo *Impl) GetItemTypeHashes(game amqp.Game) (map[int32]string, error) {
	return repo.getHashes(&entities.ItemType{}, game)
}

func (repo *Impl) GetItemHashes(game amqp.Game) (map[int32]string, error) {
	return repo.getHashes(&entities.Item{}, game)
}

func (repo *Impl) getHashes(model any, game amqp.Game) (map[int32]string, error) {
	var rows []idHash
	err := repo.db.GetDB().Model(model).Select("id, content_hash").Where("game = ?", game).Find(&rows).Error
	if err != nil {
		return nil, err
	}

	hashes := make(map[int32]string, len(rows))
	for _, row := range rows {
		hashes[row.ID] = row.ContentHash
	}

	return hashes, nil
}

// SaveItemTypes upserts the item types and replaces their labels.
func (repo *Impl) SaveItemTypes(tx *gorm.DB, game amqp.Game, itemTypes []entities.ItemType) error {
	if len(itemTypes) == 0 {
		return nil
	}

	if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).
		CreateInBatches(&itemTypes, databases.BatchSize).Error; err != nil {
		return err
	}

	ids := make([]int32, 0, len(itemTypes))
	labels := make([]entities.ItemTypeLabel, 0)
	for _, itemType := range itemTypes {
		ids = append(ids, itemType.ID)
		labels = append(labels, itemType.Labels...)
	}

	return replaceLabels(tx, game, "item_type_id", ids, &entities.ItemTypeLabel{}, labels)
}

// SaveItems upserts the items and replaces their labels.
func (repo *Impl) SaveItems(tx *gorm.DB, game amqp.Game, items []entities.Item) error {
	if len(items) == 0 {
		return nil
	}

	if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).
		CreateInBatches(&items, databases.BatchSize).Error; err != nil {
		return err
	}

	ids := make([]int32, 0, len(items))
	labels := make([]entities.ItemLabel, 0)
	for _, item := range items {
		ids = append(ids, item.ID)
		labels = append(labels, item.Labels...)
	}

	return replaceLabels(tx, game, "item_id", ids, &entities.ItemLabel{}, labels)
}

// DeleteItemTypes deletes item types; their labels follow by cascade.
func (repo *Impl) DeleteItemTypes(tx *gorm.DB, game amqp.Game, ids []int32) error {
	return databases.InChunks(ids, func(chunk []int32) error {
		return tx.Where("game = ? AND id IN ?", game, chunk).Delete(&entities.ItemType{}).Error
	})
}

// DeleteItems deletes items; their labels follow by cascade.
func (repo *Impl) DeleteItems(tx *gorm.DB, game amqp.Game, ids []int32) error {
	return databases.InChunks(ids, func(chunk []int32) error {
		return tx.Where("game = ? AND id IN ?", game, chunk).Delete(&entities.Item{}).Error
	})
}

func replaceLabels[T any](tx *gorm.DB, game amqp.Game, idColumn string, ids []int32, model *T, labels []T,
) error {
	if err := databases.InChunks(ids, func(chunk []int32) error {
		return tx.Where("game = ? AND "+idColumn+" IN ?", game, chunk).Delete(model).Error
	}); err != nil {
		return err
	}

	if len(labels) == 0 {
		return nil
	}

	return tx.CreateInBatches(&labels, databases.BatchSize).Error
}
