package imports

import (
	"context"
	"encoding/json"
	"fmt"
	"path"

	"github.com/kaellybot/kaelly-gamedata/models/constants"
	"github.com/kaellybot/kaelly-gamedata/models/dtos"
	"github.com/kaellybot/kaelly-gamedata/models/entities"
	"github.com/kaellybot/kaelly-gamedata/models/mappers"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// importClient imports the client tables (spec §5.1 to §5.6). It returns the mapped
// data when it is in the database (or would be, in dry run), nil otherwise.
func (service *Impl) importClient(ctx context.Context) (*clientData, error) {
	record := service.newImport(entities.ImportSourceClient)
	config, err := service.touchService.GetConfig(ctx)
	if err != nil {
		return nil, service.stop(&record, entities.ImportStatusFailed, "client config: "+err.Error())
	}

	buildVersion, err := service.touchService.GetBuildVersion(ctx, config.DataURL)
	if err != nil {
		return nil, service.stop(&record, entities.ImportStatusFailed, "build version: "+err.Error())
	}

	assetsVersion := path.Base(config.AssetsURL)
	record.BuildVersion = &buildVersion
	record.AssetsVersion = &assetsVersion
	if errSave := service.save(&record); errSave != nil {
		return nil, errSave
	}

	downloads, err := service.downloadClient(ctx, config.DataURL)
	if err != nil {
		return nil, service.stop(&record, entities.ImportStatusFailed, "download: "+err.Error())
	}

	if errStore := service.storeSnapshots(&record, downloads); errStore != nil {
		return nil, errStore
	}

	data, err := service.mapClient(downloads)
	if err != nil {
		return nil, service.stop(&record, entities.ImportStatusRejected, "parse: "+err.Error())
	}

	return service.applyClient(&record, data)
}

func (service *Impl) downloadClient(ctx context.Context, dataURL string) ([]download, error) {
	downloads := make([]download, 0)
	for _, class := range constants.GetClientClasses() {
		for _, language := range service.languages {
			body, err := service.touchService.GetTable(ctx, dataURL, class, language)
			if err != nil {
				return nil, fmt.Errorf("%s (%s): %w", class, language, err)
			}

			log.Debug().Str(constants.LogClass, class).Str(constants.LogLanguage, language).
				Int("bytes", len(body)).Msgf("Table downloaded")
			downloads = append(downloads, download{class: class, language: language, body: body})
		}
	}

	return downloads, nil
}

func (service *Impl) mapClient(downloads []download) (*clientData, error) {
	tables := dtos.TouchTables{
		ItemTypes:        make(map[string]map[string]dtos.TouchItemType),
		Items:            make(map[string]map[string]dtos.TouchItem),
		AlmanaxCalendars: make(map[string]map[string]dtos.TouchAlmanaxCalendar),
	}

	for _, file := range downloads {
		var err error
		switch file.class {
		case constants.ClassItemTypes:
			tables.ItemTypes[file.language], err = decode[dtos.TouchItemType](file.body)
		case constants.ClassItems:
			tables.Items[file.language], err = decode[dtos.TouchItem](file.body)
		case constants.ClassAlmanaxCalendars:
			tables.AlmanaxCalendars[file.language], err = decode[dtos.TouchAlmanaxCalendar](file.body)
		}

		if err != nil {
			return nil, fmt.Errorf("%s (%s): %w", file.class, file.language, err)
		}
	}

	itemTypes, itemTypeStats := mappers.MapItemTypes(tables.ItemTypes, service.languages)
	items, itemStats := mappers.MapItems(tables.Items, service.languages)
	calendars, calendarStats := mappers.MapAlmanaxCalendars(tables.AlmanaxCalendars, service.languages)
	return &clientData{
		itemTypes: itemTypes,
		items:     items,
		calendars: calendars,
		stats: map[string]mappers.LanguageStats{
			constants.ClassItemTypes:        itemTypeStats,
			constants.ClassItems:            itemStats,
			constants.ClassAlmanaxCalendars: calendarStats,
		},
	}, nil
}

// applyClient diffs, checks and applies the client data in one transaction.
func (service *Impl) applyClient(record *entities.GameDataImport, data *clientData) (*clientData, error) {
	unchanged, err := service.isUnchanged(*record)
	if err != nil {
		return nil, err
	}

	if unchanged {
		return data, service.markUnchanged(record)
	}

	diff, err := service.diffClient(data)
	if err != nil {
		return nil, err
	}

	if reason := service.checkClientGate(data, diff); reason != "" {
		return nil, service.stop(record, entities.ImportStatusRejected, reason)
	}

	result := report{Tables: map[string]tableReport{
		tableItemTypes: diff.itemTypes.report(),
		tableItems:     diff.items.report(),
		tableCalendars: diff.calendars.report(),
	}}

	if service.dryRun {
		service.finish(record, entities.ImportStatusApplied, result)
		return data, nil
	}

	err = service.db.GetDB().Transaction(func(tx *gorm.DB) error {
		if errWrite := service.writeClient(tx, data, diff); errWrite != nil {
			return errWrite
		}

		gameVersion := entities.GameVersion{ID: constants.Game, Version: *record.BuildVersion}
		if errVersion := service.gameVersionRepo.Save(tx, gameVersion); errVersion != nil {
			return errVersion
		}

		service.finish(record, entities.ImportStatusApplied, result)
		return service.importRepo.Save(tx, record)
	})
	if err != nil {
		return nil, service.stop(record, entities.ImportStatusFailed, "apply: "+err.Error())
	}

	service.cleanSnapshots()
	return data, nil
}

// writeClient writes in dependency order: types before items, items deleted before types.
func (service *Impl) writeClient(tx *gorm.DB, data *clientData, diff clientChanges) error {
	itemTypeID := func(itemType entities.ItemType) int32 { return itemType.ID }
	itemID := func(item entities.Item) int32 { return item.ID }
	calendarID := func(calendar entities.AlmanaxCalendar) int32 { return calendar.ID }
	writes := []func() error{
		func() error {
			return service.itemRepo.SaveItemTypes(tx, constants.Game, upserts(data.itemTypes, itemTypeID, diff.itemTypes))
		},
		func() error {
			return service.itemRepo.SaveItems(tx, constants.Game, upserts(data.items, itemID, diff.items))
		},
		func() error { return service.itemRepo.DeleteItems(tx, constants.Game, diff.items.deleted) },
		func() error { return service.itemRepo.DeleteItemTypes(tx, constants.Game, diff.itemTypes.deleted) },
		func() error {
			return service.almanaxRepo.SaveCalendars(tx, constants.Game, upserts(data.calendars, calendarID, diff.calendars))
		},
		func() error { return service.almanaxRepo.DeleteCalendars(tx, constants.Game, diff.calendars.deleted) },
	}

	for _, write := range writes {
		if err := write(); err != nil {
			return err
		}
	}

	return nil
}

func (service *Impl) diffClient(data *clientData) (clientChanges, error) {
	currentItemTypes, err := service.itemRepo.GetItemTypeHashes(constants.Game)
	if err != nil {
		return clientChanges{}, err
	}

	currentItems, err := service.itemRepo.GetItemHashes(constants.Game)
	if err != nil {
		return clientChanges{}, err
	}

	currentCalendars, err := service.almanaxRepo.GetCalendarHashes(constants.Game)
	if err != nil {
		return clientChanges{}, err
	}

	return clientChanges{
		itemTypes: computeChanges(currentItemTypes, hashes(data.itemTypes,
			func(v entities.ItemType) int32 { return v.ID }, func(v entities.ItemType) string { return v.ContentHash })),
		items: computeChanges(currentItems, hashes(data.items,
			func(v entities.Item) int32 { return v.ID }, func(v entities.Item) string { return v.ContentHash })),
		calendars: computeChanges(currentCalendars, hashes(data.calendars,
			func(v entities.AlmanaxCalendar) int32 { return v.ID },
			func(v entities.AlmanaxCalendar) string { return v.ContentHash })),
	}, nil
}

// isUnchanged compares the downloaded files to those of the last applied client import.
func (service *Impl) isUnchanged(record entities.GameDataImport) (bool, error) {
	last, found, err := service.importRepo.GetLast(constants.Game, entities.ImportSourceClient,
		[]entities.ImportStatus{entities.ImportStatusApplied, entities.ImportStatusUnchanged})
	if err != nil || !found {
		return false, err
	}

	return sameFiles(last.Files, record.Files), nil
}

// markUnchanged ends an import whose data did not change. A new build with the same
// data still moves the version, so the encyclopedia announces it.
func (service *Impl) markUnchanged(record *entities.GameDataImport) error {
	service.finish(record, entities.ImportStatusUnchanged, report{})
	if service.dryRun {
		return nil
	}

	return service.db.GetDB().Transaction(func(tx *gorm.DB) error {
		current, found, err := service.gameVersionRepo.Get(constants.Game)
		if err != nil {
			return err
		}

		if !found || current.Version != *record.BuildVersion {
			errSave := service.gameVersionRepo.Save(tx, entities.GameVersion{ID: constants.Game, Version: *record.BuildVersion})
			if errSave != nil {
				return errSave
			}
		}

		return service.importRepo.Save(tx, record)
	})
}

func sameFiles(previous, next []entities.GameDataImportFile) bool {
	if len(previous) != len(next) {
		return false
	}

	hashesByFile := make(map[string]string, len(previous))
	for _, file := range previous {
		hashesByFile[fileKey(file)] = file.Sha256
	}

	for _, file := range next {
		if hashesByFile[fileKey(file)] != file.Sha256 {
			return false
		}
	}

	return true
}

func fileKey(file entities.GameDataImportFile) string {
	return fmt.Sprintf("%s/%d", file.Class, file.Locale)
}

func decode[T any](body []byte) (map[string]T, error) {
	var table map[string]T
	if err := json.Unmarshal(body, &table); err != nil {
		return nil, err
	}

	return table, nil
}
