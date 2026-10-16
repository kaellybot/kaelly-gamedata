package items

import (
	amqp "github.com/kaellybot/kaelly-amqp"
	"github.com/kaellybot/kaelly-gamedata/models/entities"
	"github.com/kaellybot/kaelly-gamedata/utils/databases"
	"gorm.io/gorm"
)

type Repository interface {
	GetItemTypeHashes(game amqp.Game) (map[int32]string, error)
	GetItemHashes(game amqp.Game) (map[int32]string, error)
	SaveItemTypes(tx *gorm.DB, game amqp.Game, itemTypes []entities.ItemType) error
	SaveItems(tx *gorm.DB, game amqp.Game, items []entities.Item) error
	DeleteItemTypes(tx *gorm.DB, game amqp.Game, ids []int32) error
	DeleteItems(tx *gorm.DB, game amqp.Game, ids []int32) error
}

type Impl struct {
	db databases.MySQLConnection
}
