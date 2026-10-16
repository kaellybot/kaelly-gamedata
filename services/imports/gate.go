package imports

import (
	"fmt"
	"slices"

	"github.com/kaellybot/kaelly-gamedata/models/constants"
	"github.com/kaellybot/kaelly-gamedata/models/entities"
)

// clientChanges holds the diff of every client table.
type clientChanges struct {
	itemTypes changes[int32]
	items     changes[int32]
	calendars changes[int32]
}

// checkClientGate returns why the client data must not be applied, or "" (spec §5.4).
func (service *Impl) checkClientGate(data *clientData, diff clientChanges) string {
	tables := []struct {
		name    string
		changes changes[int32]
	}{
		{tableItemTypes, diff.itemTypes},
		{tableItems, diff.items},
		{tableCalendars, diff.calendars},
	}

	for _, table := range tables {
		if table.changes.nextTotal == 0 {
			return fmt.Sprintf("table %s is empty", table.name)
		}

		if table.changes.currentTotal > 0 {
			ratio := float64(len(table.changes.deleted)) / float64(table.changes.currentTotal)
			if ratio > service.maxDeletedRatio {
				return fmt.Sprintf("table %s loses %d of its %d rows (%.1f%%)", table.name,
					len(table.changes.deleted), table.changes.currentTotal, ratio*percent)
			}
		}
	}

	if !slices.ContainsFunc(data.items, func(item entities.Item) bool {
		return item.ID == constants.ReferenceItemID
	}) {
		return fmt.Sprintf("reference item %d is missing", constants.ReferenceItemID)
	}

	if !slices.ContainsFunc(data.calendars, func(calendar entities.AlmanaxCalendar) bool {
		return calendar.ID == constants.ReferenceAlmanaxCalendarID
	}) {
		return fmt.Sprintf("reference almanax calendar %d is missing", constants.ReferenceAlmanaxCalendarID)
	}

	return service.checkLanguages(data)
}

// checkLanguages detects a language Ankama silently answered in English.
func (service *Impl) checkLanguages(data *clientData) string {
	for _, class := range constants.GetClientClasses() {
		stats := data.stats[class]
		if stats.Total == 0 {
			continue
		}

		for _, language := range service.languages {
			if language == constants.ReferenceLanguage {
				continue
			}

			ratio := float64(stats.Identical[language]+stats.Missing[language]) / float64(stats.Total)
			if ratio > constants.MaxIdenticalNameRatio {
				return fmt.Sprintf("%s: %.0f%% of %s names are missing or identical to %s",
					class, ratio*percent, language, constants.ReferenceLanguage)
			}
		}
	}

	return ""
}

const percent = 100
