package almanaxes

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

func (repo *Impl) GetCalendarHashes(game amqp.Game) (map[int32]string, error) {
	var rows []struct {
		ID          int32
		ContentHash string
	}

	err := repo.db.GetDB().Model(&entities.AlmanaxCalendar{}).
		Select("id, content_hash").Where("game = ?", game).Find(&rows).Error
	if err != nil {
		return nil, err
	}

	hashes := make(map[int32]string, len(rows))
	for _, row := range rows {
		hashes[row.ID] = row.ContentHash
	}

	return hashes, nil
}

func (repo *Impl) GetAlmanaxHashes(game amqp.Game) (map[entities.AlmanaxKey]string, error) {
	var almanaxes []entities.Almanax
	if err := repo.db.GetDB().Where("game = ?", game).Find(&almanaxes).Error; err != nil {
		return nil, err
	}

	hashes := make(map[entities.AlmanaxKey]string, len(almanaxes))
	for _, almanax := range almanaxes {
		hash := ""
		if almanax.ContentHash != nil {
			hash = *almanax.ContentHash
		}
		hashes[entities.AlmanaxKey{Month: almanax.Month, Day: almanax.Day}] = hash
	}

	return hashes, nil
}

// SaveCalendars upserts the calendar entries and replaces their labels.
func (repo *Impl) SaveCalendars(tx *gorm.DB, game amqp.Game, calendars []entities.AlmanaxCalendar) error {
	if len(calendars) == 0 {
		return nil
	}

	if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).
		CreateInBatches(&calendars, databases.BatchSize).Error; err != nil {
		return err
	}

	ids := make([]int32, 0, len(calendars))
	labels := make([]entities.AlmanaxCalendarLabel, 0)
	for _, calendar := range calendars {
		ids = append(ids, calendar.ID)
		labels = append(labels, calendar.Labels...)
	}

	if err := databases.InChunks(ids, func(chunk []int32) error {
		return tx.Where("game = ? AND calendar_id IN ?", game, chunk).Delete(&entities.AlmanaxCalendarLabel{}).Error
	}); err != nil {
		return err
	}

	return tx.CreateInBatches(&labels, databases.BatchSize).Error
}

// DeleteCalendars deletes calendar entries; their labels follow by cascade.
func (repo *Impl) DeleteCalendars(tx *gorm.DB, game amqp.Game, ids []int32) error {
	return databases.InChunks(ids, func(chunk []int32) error {
		return tx.Where("game = ? AND id IN ?", game, chunk).Delete(&entities.AlmanaxCalendar{}).Error
	})
}

func (repo *Impl) GetCrawls(game amqp.Game) (map[entities.AlmanaxKey]entities.AlmanaxCrawl, error) {
	var crawls []entities.AlmanaxCrawl
	if err := repo.db.GetDB().Where("game = ?", game).Find(&crawls).Error; err != nil {
		return nil, err
	}

	result := make(map[entities.AlmanaxKey]entities.AlmanaxCrawl, len(crawls))
	for _, crawl := range crawls {
		result[entities.AlmanaxKey{Month: crawl.Month, Day: crawl.Day}] = crawl
	}

	return result, nil
}

func (repo *Impl) SaveCrawls(tx *gorm.DB, crawls []entities.AlmanaxCrawl) error {
	if len(crawls) == 0 {
		return nil
	}

	return tx.Clauses(clause.OnConflict{UpdateAll: true}).CreateInBatches(&crawls, databases.BatchSize).Error
}

func (repo *Impl) SaveAlmanaxes(tx *gorm.DB, almanaxes []entities.Almanax) error {
	if len(almanaxes) == 0 {
		return nil
	}

	return tx.Clauses(clause.OnConflict{UpdateAll: true}).CreateInBatches(&almanaxes, databases.BatchSize).Error
}
