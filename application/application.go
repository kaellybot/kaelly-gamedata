package application

import (
	amqp "github.com/kaellybot/kaelly-amqp"
	"github.com/kaellybot/kaelly-gamedata/models/constants"
	"github.com/kaellybot/kaelly-gamedata/repositories/almanaxes"
	"github.com/kaellybot/kaelly-gamedata/repositories/gamedataimports"
	"github.com/kaellybot/kaelly-gamedata/repositories/gameversions"
	"github.com/kaellybot/kaelly-gamedata/repositories/items"
	"github.com/kaellybot/kaelly-gamedata/services/imports"
	"github.com/kaellybot/kaelly-gamedata/services/krosmoz"
	"github.com/kaellybot/kaelly-gamedata/services/storages"
	"github.com/kaellybot/kaelly-gamedata/services/touch"
	"github.com/kaellybot/kaelly-gamedata/utils/databases"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

func New() (*Impl, error) {
	// misc
	broker := amqp.New(constants.RabbitMQClientID, viper.GetString(constants.RabbitMQAddress))
	db := databases.New()
	storageService, err := storages.New()
	if err != nil {
		return nil, err
	}

	// repositories
	importRepo := gamedataimports.New(db)
	itemRepo := items.New(db)
	almanaxRepo := almanaxes.New(db)
	gameVersionRepo := gameversions.New(db)

	// services
	importService, err := imports.New(touch.New(), krosmoz.New(), storageService,
		importRepo, itemRepo, almanaxRepo, gameVersionRepo, db, broker)
	if err != nil {
		return nil, err
	}

	return &Impl{
		importService:  importService,
		storageService: storageService,
		broker:         broker,
		db:             db,
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
	app.storageService.Shutdown()
	log.Info().Msgf("Application is no longer running")
}
