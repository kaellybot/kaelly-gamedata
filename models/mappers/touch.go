package mappers

import (
	"cmp"
	"slices"
	"strconv"

	amqp "github.com/kaellybot/kaelly-amqp"
	"github.com/kaellybot/kaelly-gamedata/models/constants"
	"github.com/kaellybot/kaelly-gamedata/models/dtos"
	"github.com/kaellybot/kaelly-gamedata/models/entities"
)

const weaponType = "Weapon"

// LanguageStats counts, per language, the records whose name is missing or
// identical to the reference language. Used by the validation gate.
type LanguageStats struct {
	Total     int
	Identical map[string]int
	Missing   map[string]int
}

func newLanguageStats(total int) LanguageStats {
	return LanguageStats{Total: total, Identical: make(map[string]int), Missing: make(map[string]int)}
}

func (stats *LanguageStats) count(language, name, referenceName string, found bool) {
	if language == constants.ReferenceLanguage {
		return
	}

	switch {
	case !found:
		stats.Missing[language]++
	case name == referenceName:
		stats.Identical[language]++
	}
}

func MapItemTypes(tables map[string]map[string]dtos.TouchItemType, languages []string,
) ([]entities.ItemType, LanguageStats) {
	reference := tables[constants.ReferenceLanguage]
	stats := newLanguageStats(len(reference))
	itemTypesBySuperType := constants.GetItemTypesBySuperType()
	result := make([]entities.ItemType, 0, len(reference))
	for _, key := range sortedKeys(reference) {
		raw := reference[key]
		itemType := entities.ItemType{ID: raw.ID, Game: constants.Game, SuperTypeID: raw.SuperTypeID}
		if mapped, found := itemTypesBySuperType[raw.SuperTypeID]; found {
			itemType.AmqpItemType = &mapped
		}

		for _, language := range languages {
			translated, found := tables[language][key]
			stats.count(language, translated.NameID, raw.NameID, found)
			if found {
				itemType.Labels = append(itemType.Labels, entities.ItemTypeLabel{
					ItemTypeID: raw.ID, Game: constants.Game,
					Locale: toLocale(language), Label: translated.NameID,
				})
			}
		}

		sortByLocale(itemType.Labels, func(label entities.ItemTypeLabel) amqp.Language { return label.Locale })
		itemType.ContentHash = ContentHash(itemType)
		result = append(result, itemType)
	}

	return result, stats
}

func MapItems(tables map[string]map[string]dtos.TouchItem, languages []string,
) ([]entities.Item, LanguageStats) {
	reference := tables[constants.ReferenceLanguage]
	stats := newLanguageStats(len(reference))
	result := make([]entities.Item, 0, len(reference))
	for _, key := range sortedKeys(reference) {
		raw := reference[key]
		item := mapItem(raw)
		for _, language := range languages {
			translated, found := tables[language][key]
			stats.count(language, translated.NameID, raw.NameID, found)
			if found {
				item.Labels = append(item.Labels, entities.ItemLabel{
					ItemID: raw.ID, Game: constants.Game, Locale: toLocale(language),
					Name: translated.NameID, Description: optionalString(translated.DescriptionID),
				})
			}
		}

		sortByLocale(item.Labels, func(label entities.ItemLabel) amqp.Language { return label.Locale })
		item.ContentHash = ContentHash(item)
		result = append(result, item)
	}

	return result, stats
}

func mapItem(raw dtos.TouchItem) entities.Item {
	item := entities.Item{
		ID:       raw.ID,
		Game:     constants.Game,
		TypeID:   raw.TypeID,
		Level:    raw.Level,
		IconID:   raw.IconID,
		Pods:     raw.RealWeight,
		Criteria: optionalString(raw.Criteria),
	}

	if raw.Type == weaponType {
		item.ApCost = &raw.ApCost
		item.MinRange = &raw.MinRange
		item.MaxRange = &raw.Range
		item.MaxCastPerTurn = &raw.MaxCastPerTurn
		item.CriticalRate = &raw.CriticalHitProbability
		item.CriticalBonus = &raw.CriticalHitBonus
		item.TwoHanded = &raw.TwoHanded
	}

	return item
}

func MapAlmanaxCalendars(tables map[string]map[string]dtos.TouchAlmanaxCalendar, languages []string,
) ([]entities.AlmanaxCalendar, LanguageStats) {
	reference := tables[constants.ReferenceLanguage]
	stats := newLanguageStats(len(reference))
	result := make([]entities.AlmanaxCalendar, 0, len(reference))
	for _, key := range sortedKeys(reference) {
		raw := reference[key]
		calendar := entities.AlmanaxCalendar{ID: raw.ID, Game: constants.Game, NpcID: raw.NpcID}
		for _, language := range languages {
			translated, found := tables[language][key]
			stats.count(language, translated.NameID, raw.NameID, found)
			if found {
				calendar.Labels = append(calendar.Labels, entities.AlmanaxCalendarLabel{
					CalendarID: raw.ID, Game: constants.Game, Locale: toLocale(language),
					Name: translated.NameID, Description: translated.DescID,
				})
			}
		}

		sortByLocale(calendar.Labels, func(label entities.AlmanaxCalendarLabel) amqp.Language { return label.Locale })
		calendar.ContentHash = ContentHash(calendar)
		result = append(result, calendar)
	}

	return result, stats
}

func toLocale(language string) amqp.Language {
	return constants.GetLanguages()[language]
}

// optionalString maps the client's empty values ("" and "null") to NULL.
func optionalString(value string) *string {
	if value == "" || value == "null" {
		return nil
	}

	return &value
}

// sortedKeys returns the table keys (string ids) in numeric order, for deterministic output.
func sortedKeys[V any](table map[string]V) []string {
	keys := make([]string, 0, len(table))
	for key := range table {
		keys = append(keys, key)
	}

	slices.SortFunc(keys, func(a, b string) int {
		idA, errA := strconv.Atoi(a)
		idB, errB := strconv.Atoi(b)
		if errA != nil || errB != nil {
			return cmp.Compare(a, b)
		}
		return cmp.Compare(idA, idB)
	})

	return keys
}

func sortByLocale[T any](labels []T, locale func(T) amqp.Language) {
	slices.SortFunc(labels, func(a, b T) int { return cmp.Compare(locale(a), locale(b)) })
}
