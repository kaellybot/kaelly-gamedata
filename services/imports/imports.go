package imports

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	amqp "github.com/kaellybot/kaelly-amqp"
	"github.com/kaellybot/kaelly-gamedata/models/constants"
	"github.com/kaellybot/kaelly-gamedata/models/entities"
	"github.com/kaellybot/kaelly-gamedata/repositories/almanaxes"
	"github.com/kaellybot/kaelly-gamedata/repositories/gamedataimports"
	"github.com/kaellybot/kaelly-gamedata/repositories/gameversions"
	"github.com/kaellybot/kaelly-gamedata/repositories/items"
	"github.com/kaellybot/kaelly-gamedata/services/krosmoz"
	"github.com/kaellybot/kaelly-gamedata/services/storages"
	"github.com/kaellybot/kaelly-gamedata/services/touch"
	"github.com/kaellybot/kaelly-gamedata/utils/databases"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

func New(touchService touch.Service, krosmozService krosmoz.Service, storageService storages.Service,
	importRepo gamedataimports.Repository, itemRepo items.Repository, almanaxRepo almanaxes.Repository,
	gameVersionRepo gameversions.Repository, db databases.MySQLConnection, broker amqp.MessageBroker,
) (*Impl, error) {
	languages := parseLanguages(viper.GetString(constants.TouchLanguages))
	krosmozLanguage := viper.GetString(constants.KrosmozLanguage)
	for _, required := range []string{constants.ReferenceLanguage, krosmozLanguage} {
		if !slices.Contains(languages, required) {
			return nil, fmt.Errorf("%w: %s (%v)", ErrLanguageNotDownloaded, required, languages)
		}
	}

	if daysPerRun := viper.GetInt(constants.KrosmozDaysPerRun); daysPerRun <= 0 {
		return nil, fmt.Errorf("%w: %d", ErrInvalidDaysPerRun, daysPerRun)
	}

	return &Impl{
		languages:              languages,
		krosmozLanguage:        krosmozLanguage,
		almanaxRefreshInterval: viper.GetDuration(constants.AlmanaxRefreshInterval),
		krosmozDaysPerRun:      viper.GetInt(constants.KrosmozDaysPerRun),
		maxDeletedRatio:        viper.GetFloat64(constants.MaxDeletedRatio),
		snapshotRetention:      viper.GetInt(constants.SnapshotRetention),
		dryRun:                 viper.GetBool(constants.DryRun),
		touchService:           touchService,
		krosmozService:         krosmozService,
		storageService:         storageService,
		importRepo:             importRepo,
		itemRepo:               itemRepo,
		almanaxRepo:            almanaxRepo,
		gameVersionRepo:        gameVersionRepo,
		db:                     db,
		broker:                 broker,
	}, nil
}

// Import runs one pass, as described in docs/specs/dofus-touch-data.md (§5): the client
// data, then the almanax. An import that cannot be applied is recorded and alerted, and
// does not fail the run; only an unusable database or storage does.
func (service *Impl) Import() error {
	ctx := context.Background()
	log.Info().
		Str(constants.LogGame, constants.Game.String()).
		Bool(constants.LogDryRun, service.dryRun).
		Msgf("Importing game data...")

	data, err := service.importClient(ctx)
	if err != nil {
		return err
	}

	return service.importAlmanax(ctx, data)
}

func (service *Impl) newImport(source entities.ImportSource) entities.GameDataImport {
	return entities.GameDataImport{
		Game:      constants.Game,
		Source:    source,
		Trigger:   entities.ImportTriggerScheduled,
		Status:    entities.ImportStatusRunning,
		StartedAt: time.Now().UTC(),
	}
}

// save persists an import outside any transaction. Nothing is written in dry run.
func (service *Impl) save(record *entities.GameDataImport) error {
	if service.dryRun {
		return nil
	}

	return service.importRepo.Save(service.db.GetDB(), record)
}

// finish sets the outcome of an import and logs it. The caller persists it.
func (service *Impl) finish(record *entities.GameDataImport, status entities.ImportStatus, result report) {
	now := time.Now().UTC()
	record.Status = status
	record.FinishedAt = &now
	if body, err := json.Marshal(result); err == nil {
		value := string(body)
		record.Report = &value
	}

	level := zerolog.InfoLevel
	switch status {
	case entities.ImportStatusRejected, entities.ImportStatusFailed:
		level = zerolog.ErrorLevel
	case entities.ImportStatusPartial:
		level = zerolog.WarnLevel
	case entities.ImportStatusRunning, entities.ImportStatusApplied, entities.ImportStatusUnchanged:
	}

	event := log.WithLevel(level).
		Int64(constants.LogImportID, record.ID).
		Str(constants.LogGame, record.Game.String()).
		Str(constants.LogSource, string(record.Source)).
		Str(constants.LogStatus, string(status)).
		Bool(constants.LogDryRun, service.dryRun)
	if record.BuildVersion != nil {
		event = event.Str(constants.LogBuildVersion, *record.BuildVersion)
	}
	if record.Report != nil {
		event = event.RawJSON(constants.LogReport, []byte(*record.Report))
	}
	event.Msgf("Import finished")
}

// stop ends an import that cannot be applied, records it and alerts.
func (service *Impl) stop(record *entities.GameDataImport, status entities.ImportStatus, reason string) error {
	service.finish(record, status, report{Reason: reason})
	if err := service.save(record); err != nil {
		return err
	}

	service.alert(*record, reason)
	return nil
}

func parseLanguages(value string) []string {
	languages := make([]string, 0)
	for language := range strings.SplitSeq(value, ",") {
		if trimmed := strings.TrimSpace(language); trimmed != "" {
			languages = append(languages, trimmed)
		}
	}

	return languages
}
