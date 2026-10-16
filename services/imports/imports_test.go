package imports

import (
	"slices"
	"strings"
	"testing"

	"github.com/kaellybot/kaelly-gamedata/models/constants"
	"github.com/kaellybot/kaelly-gamedata/models/entities"
	"github.com/kaellybot/kaelly-gamedata/models/mappers"
)

func TestComputeChanges(t *testing.T) {
	current := map[int32]string{1: "a", 2: "b", 3: "c"}
	next := map[int32]string{1: "a", 2: "B", 4: "d"}

	diff := computeChanges(current, next)
	slices.Sort(diff.created)
	if !slices.Equal(diff.created, []int32{4}) || !slices.Equal(diff.updated, []int32{2}) ||
		!slices.Equal(diff.deleted, []int32{3}) {
		t.Errorf("unexpected changes: %+v", diff)
	}

	if diff.report() != (tableReport{Total: 3, Created: 1, Updated: 1, Deleted: 1}) || diff.isEmpty() {
		t.Errorf("unexpected report: %+v", diff.report())
	}

	type record struct {
		id   int32
		name string
	}
	records := []record{{1, "same"}, {2, "updated"}, {4, "created"}}
	kept := upserts(records, func(r record) int32 { return r.id }, diff)
	if len(kept) != 2 || kept[0].id != 2 || kept[1].id != 4 {
		t.Errorf("upserts must keep created and updated records only: %+v", kept)
	}

	if !computeChanges(next, next).isEmpty() {
		t.Error("identical tables must have no change")
	}
}

func validData() *clientData {
	items := []entities.Item{{ID: constants.ReferenceItemID}}
	calendars := []entities.AlmanaxCalendar{{ID: constants.ReferenceAlmanaxCalendarID}}
	return &clientData{
		itemTypes: []entities.ItemType{{ID: 1}},
		items:     items,
		calendars: calendars,
		stats: map[string]mappers.LanguageStats{
			constants.ClassItems: {Total: 100, Identical: map[string]int{"fr": 3}, Missing: map[string]int{}},
		},
	}
}

func validChanges() clientChanges {
	table := changes[int32]{currentTotal: 100, nextTotal: 100}
	return clientChanges{itemTypes: table, items: table, calendars: table}
}

func TestCheckClientGate(t *testing.T) {
	service := &Impl{languages: []string{"fr", "en", "de"}, maxDeletedRatio: 0.05}

	if reason := service.checkClientGate(validData(), validChanges()); reason != "" {
		t.Fatalf("valid data rejected: %s", reason)
	}

	tests := map[string]struct {
		data     func() *clientData
		changes  func() clientChanges
		expected string
	}{
		"empty table": {validData, func() clientChanges {
			diff := validChanges()
			diff.calendars.nextTotal = 0
			return diff
		}, "table almanax_calendars is empty"},
		"too many deletions": {validData, func() clientChanges {
			diff := validChanges()
			diff.items.deleted = make([]int32, 6)
			return diff
		}, "table items loses 6 of its 100 rows"},
		"missing reference item": {func() *clientData {
			data := validData()
			data.items = []entities.Item{{ID: 1}}
			return data
		}, validChanges, "reference item 7905 is missing"},
		"language answered in English": {func() *clientData {
			data := validData()
			data.stats[constants.ClassItems].Identical["de"] = 100
			return data
		}, validChanges, "Items: 100% of de names are missing or identical to en"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			reason := service.checkClientGate(test.data(), test.changes())
			if !strings.HasPrefix(reason, test.expected) {
				t.Errorf("got %q, expected %q", reason, test.expected)
			}
		})
	}
}

func TestFirstImportIsNotADeletion(t *testing.T) {
	service := &Impl{languages: []string{"en"}, maxDeletedRatio: 0.05}
	diff := validChanges()
	diff.items.currentTotal = 0
	if reason := service.checkClientGate(validData(), diff); reason != "" {
		t.Errorf("first import rejected: %s", reason)
	}
}

func TestSameFiles(t *testing.T) {
	file := func(class, hash string) entities.GameDataImportFile {
		return entities.GameDataImportFile{Class: class, Locale: 1, Sha256: hash}
	}

	previous := []entities.GameDataImportFile{file("Items", "a"), file("ItemTypes", "b")}
	if !sameFiles(previous, []entities.GameDataImportFile{file("ItemTypes", "b"), file("Items", "a")}) {
		t.Error("same files in another order must be equal")
	}

	if sameFiles(previous, []entities.GameDataImportFile{file("Items", "a"), file("ItemTypes", "c")}) {
		t.Error("a changed hash must differ")
	}

	if sameFiles(previous, previous[:1]) {
		t.Error("a missing file must differ")
	}
}

func TestParseLanguages(t *testing.T) {
	if languages := parseLanguages(" fr, en,,es ,pt"); !slices.Equal(languages, []string{"fr", "en", "es", "pt"}) {
		t.Errorf("unexpected languages: %v", languages)
	}
}
