package mappers

import (
	"fmt"
	"slices"
	"strings"

	"github.com/kaellybot/kaelly-gamedata/models/constants"
	"github.com/kaellybot/kaelly-gamedata/models/dtos"
	"github.com/kaellybot/kaelly-gamedata/models/entities"
)

type offeringKey struct {
	name   string
	iconID int32
}

// AlmanaxIndex finds the client records a Krosmoz almanax day refers to, by text.
type AlmanaxIndex struct {
	calendarsByText        map[string][]int32
	calendarsByDescription map[string][]int32
	itemsByOffering        map[offeringKey][]int32
}

// NewAlmanaxIndex indexes the labels of the language Krosmoz is crawled in.
func NewAlmanaxIndex(calendars []entities.AlmanaxCalendar, items []entities.Item, language string,
) AlmanaxIndex {
	locale := toLocale(language)
	index := AlmanaxIndex{
		calendarsByText:        make(map[string][]int32),
		calendarsByDescription: make(map[string][]int32),
		itemsByOffering:        make(map[offeringKey][]int32),
	}

	for _, calendar := range calendars {
		for _, label := range calendar.Labels {
			if label.Locale != locale {
				continue
			}

			description := NormalizeText(label.Description)
			text := calendarText(NormalizeText(label.Name), description)
			index.calendarsByText[text] = append(index.calendarsByText[text], calendar.ID)
			index.calendarsByDescription[description] = append(index.calendarsByDescription[description], calendar.ID)
		}
	}

	for _, item := range items {
		for _, label := range item.Labels {
			if label.Locale == locale {
				key := offeringKey{name: NormalizeText(label.Name), iconID: item.IconID}
				index.itemsByOffering[key] = append(index.itemsByOffering[key], item.ID)
			}
		}
	}

	return index
}

// MapAlmanaxes joins the Krosmoz days to the client data. A half that cannot be joined
// (bonus or offering) is left NULL and reported as a mismatch: the other half is still
// worth showing. A day with neither half gets no row.
func MapAlmanaxes(days []dtos.KrosmozAlmanax, index AlmanaxIndex,
) ([]entities.Almanax, []dtos.AlmanaxMismatch) {
	almanaxes := make([]entities.Almanax, 0, len(days))
	mismatches := make([]dtos.AlmanaxMismatch, 0)
	for _, day := range days {
		almanax := entities.Almanax{Day: day.Day, Month: day.Month, Game: constants.Game}
		reasons := make([]string, 0)

		if calendarID, found := index.findCalendar(day); found {
			almanax.CalendarID = &calendarID
		} else {
			reasons = append(reasons, fmt.Sprintf("no calendar entry for bonus %q", day.BonusDescription))
		}

		if itemID, found := index.findOffering(day); found {
			quantity := day.OfferingQuantity
			almanax.OfferingItemID = &itemID
			almanax.OfferingQuantity = &quantity
		} else {
			reasons = append(reasons, fmt.Sprintf("no item for offering %q (icon %d)",
				day.OfferingName, day.OfferingIconID))
		}

		if len(reasons) > 0 {
			mismatches = append(mismatches, dtos.AlmanaxMismatch{Month: day.Month, Day: day.Day,
				Reason: strings.Join(reasons, "; ")})
		}

		if almanax.CalendarID == nil && almanax.OfferingItemID == nil {
			continue
		}

		hash := ContentHash(almanax)
		almanax.ContentHash = &hash
		almanaxes = append(almanaxes, almanax)
	}

	return almanaxes, mismatches
}

// findCalendar matches on name and description, then on description alone. Entries
// sharing the same texts only differ in wording in other languages: the lowest id wins.
func (index AlmanaxIndex) findCalendar(day dtos.KrosmozAlmanax) (int32, bool) {
	description := NormalizeText(day.BonusDescription)
	if ids, found := index.calendarsByText[calendarText(NormalizeText(day.BonusName), description)]; found {
		return slices.Min(ids), true
	}

	if ids, found := index.calendarsByDescription[description]; found {
		return slices.Min(ids), true
	}

	return 0, false
}

// findOffering matches on name and icon: names alone are not unique.
func (index AlmanaxIndex) findOffering(day dtos.KrosmozAlmanax) (int32, bool) {
	ids, found := index.itemsByOffering[offeringKey{name: NormalizeText(day.OfferingName), iconID: day.OfferingIconID}]
	if !found {
		return 0, false
	}

	return slices.Min(ids), true
}

func calendarText(name, description string) string {
	return name + "\x00" + description
}
