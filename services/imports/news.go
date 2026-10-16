package imports

import (
	amqp "github.com/kaellybot/kaelly-amqp"
	"github.com/kaellybot/kaelly-gamedata/models/constants"
	"github.com/kaellybot/kaelly-gamedata/models/entities"
	"github.com/rs/zerolog/log"
)

// alert publishes NEWS_IMPORT for an import that is not fully applied (spec §5.7).
func (service *Impl) alert(record entities.GameDataImport, reason string) {
	if service.dryRun {
		return
	}

	version := ""
	if record.BuildVersion != nil {
		version = *record.BuildVersion
	}

	message := amqp.RabbitMQMessage{
		Type:     amqp.RabbitMQMessage_NEWS_IMPORT,
		Language: amqp.Language_ANY,
		Game:     record.Game,
		NewsImportMessage: &amqp.NewsImportMessage{
			ImportId: record.ID,
			Source:   mapSource(record.Source),
			Version:  version,
			Status:   mapStatus(record.Status),
			Reason:   reason,
		},
	}

	err := service.broker.Emit(&message, amqp.ExchangeNews, constants.NewsImportRoutingKey, amqp.GenerateUUID())
	if err != nil {
		log.Error().Err(err).
			Int64(constants.LogImportID, record.ID).
			Str(constants.LogGame, record.Game.String()).
			Msgf("Import alert failed to be published")
	}
}

func mapSource(source entities.ImportSource) amqp.NewsImportMessage_Source {
	switch source {
	case entities.ImportSourceClient:
		return amqp.NewsImportMessage_CLIENT
	case entities.ImportSourceKrosmoz:
		return amqp.NewsImportMessage_KROSMOZ
	default:
		return amqp.NewsImportMessage_UNKNOWN_SOURCE
	}
}

func mapStatus(status entities.ImportStatus) amqp.NewsImportMessage_Status {
	switch status {
	case entities.ImportStatusRejected:
		return amqp.NewsImportMessage_REJECTED
	case entities.ImportStatusFailed:
		return amqp.NewsImportMessage_FAILED
	case entities.ImportStatusPartial:
		return amqp.NewsImportMessage_PARTIAL
	case entities.ImportStatusRunning, entities.ImportStatusApplied, entities.ImportStatusUnchanged:
		return amqp.NewsImportMessage_UNKNOWN_STATUS
	default:
		return amqp.NewsImportMessage_UNKNOWN_STATUS
	}
}
