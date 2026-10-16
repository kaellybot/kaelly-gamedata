package almanaxes

import (
	amqp "github.com/kaellybot/kaelly-amqp"
	"github.com/kaellybot/kaelly-gamedata/models/entities"
	"github.com/kaellybot/kaelly-gamedata/utils/databases"
	"gorm.io/gorm"
)

type Repository interface {
	GetCalendarHashes(game amqp.Game) (map[int32]string, error)
	GetAlmanaxHashes(game amqp.Game) (map[entities.AlmanaxKey]string, error)
	SaveCalendars(tx *gorm.DB, game amqp.Game, calendars []entities.AlmanaxCalendar) error
	DeleteCalendars(tx *gorm.DB, game amqp.Game, ids []int32) error
	SaveAlmanaxes(tx *gorm.DB, almanaxes []entities.Almanax) error
	GetCrawls(game amqp.Game) (map[entities.AlmanaxKey]entities.AlmanaxCrawl, error)
	SaveCrawls(tx *gorm.DB, crawls []entities.AlmanaxCrawl) error
}

type Impl struct {
	db databases.MySQLConnection
}
