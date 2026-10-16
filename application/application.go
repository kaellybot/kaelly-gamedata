package application

import (
	amqp "github.com/kaellybot/kaelly-amqp"
	"github.com/kaellybot/kaelly-gamedata/models/constants"
	"github.com/kaellybot/kaelly-gamedata/services/imports"
	"github.com/kaellybot/kaelly-gamedata/utils/databases"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

func New() (*Impl, error) {
	// misc
	broker := amqp.New(constants.RabbitMQClientID, viper.GetString(constants.RabbitMQAddress))
	db := databases.New()

	// services
	importService, err := imports.New()
	if err != nil {
		return nil, err
	}

	return &Impl{
		importService: importService,
		broker:        broker,
		db:            db,
	}, nil
}

func (app *Impl) Run() error {
	if err := app.db.Run(); err != nil {
		return err
	}

	if err := app.broker.Run(); err != nil {
		return err
	}

	return app.importService.Import()
}

func (app *Impl) Shutdown() {
	app.broker.Shutdown()
	app.db.Shutdown()
	log.Info().Msgf("Application is no longer running")
}
