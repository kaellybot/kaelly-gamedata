package application

import (
	amqp "github.com/kaellybot/kaelly-amqp"
	"github.com/kaellybot/kaelly-gamedata/services/imports"
	"github.com/kaellybot/kaelly-gamedata/utils/databases"
)

type Application interface {
	Run() error
	Shutdown()
}

type Impl struct {
	importService imports.Service
	broker        amqp.MessageBroker
	db            databases.MySQLConnection
}
