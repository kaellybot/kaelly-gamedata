package mappers

import (
	"strings"
	"testing"

	amqp "github.com/kaellybot/kaelly-amqp"
	"github.com/kaellybot/kaelly-gamedata/models/dtos"
	"github.com/kaellybot/kaelly-gamedata/models/entities"
)

func TestNormalizeText(t *testing.T) {
	got := NormalizeText("Les gains   d’expérience sont augmentés dans la zone des <b>Lacs Enchantés</b>.\n")
	if got != "Les gains d'expérience sont augmentés dans la zone des Lacs Enchantés." {
		t.Errorf("unexpected normalization: %q", got)
	}
}

func TestMapItems(t *testing.T) {
	tables := map[string]map[string]dtos.TouchItem{
		"en": {
			"10": {Type: "Item", ID: 10, TypeID: 48, IconID: 57001, NameID: "Gobkool Wool", Criteria: "null"},
			"2":  {Type: "Weapon", ID: 2, TypeID: 6, NameID: "Sword", Criteria: "CS>20", ApCost: 3, Range: 1},
		},
		"fr": {
			"10": {Type: "Item", ID: 10, NameID: "Laine du Boufcoul", DescriptionID: "Une laine."},
			"2":  {Type: "Weapon", ID: 2, NameID: "Sword"},
		},
	}

	items, stats := MapItems(tables, []string{"fr", "en", "pt"})
	if len(items) != 2 || items[0].ID != 2 || items[1].ID != 10 {
		t.Fatalf("expected items sorted by numeric id, got %+v", items)
	}

	weapon, resource := items[0], items[1]
	switch {
	case weapon.ApCost == nil || *weapon.ApCost != 3 || weapon.MaxRange == nil || *weapon.MaxRange != 1:
		t.Errorf("weapon fields not mapped: %+v", weapon)
	case resource.ApCost != nil:
		t.Errorf("weapon fields set on a resource: %+v", resource)
	case resource.Criteria != nil || weapon.Criteria == nil || *weapon.Criteria != "CS>20":
		t.Errorf("criteria not mapped: %v, %v", resource.Criteria, weapon.Criteria)
	case len(resource.Labels) != 2 || resource.Labels[0].Locale != amqp.Language_FR:
		t.Errorf("labels not sorted by locale: %+v", resource.Labels)
	case resource.Labels[1].Description != nil || *resource.Labels[0].Description != "Une laine.":
		t.Errorf("descriptions not mapped: %+v", resource.Labels)
	}

	if stats.Total != 2 || stats.Identical["fr"] != 1 || stats.Missing["pt"] != 2 {
		t.Errorf("unexpected language stats: %+v", stats)
	}
}

func TestContentHashFollowsLabels(t *testing.T) {
	table := func(name string) map[string]map[string]dtos.TouchItem {
		return map[string]map[string]dtos.TouchItem{"en": {"1": {ID: 1, NameID: name}}}
	}

	first, _ := MapItems(table("Wool"), []string{"en"})
	same, _ := MapItems(table("Wool"), []string{"en"})
	renamed, _ := MapItems(table("Wol"), []string{"en"})
	if first[0].ContentHash != same[0].ContentHash {
		t.Error("the same record must give the same hash")
	}

	if first[0].ContentHash == renamed[0].ContentHash {
		t.Error("a label change must change the hash")
	}
}

func TestMapItemTypes(t *testing.T) {
	tables := map[string]map[string]dtos.TouchItemType{
		"en": {"48": {ID: 48, NameID: "Wool", SuperTypeID: 9}, "151": {ID: 151, NameID: "Cube", SuperTypeID: 39}},
	}

	itemTypes, _ := MapItemTypes(tables, []string{"en"})
	resource, cube := itemTypes[0], itemTypes[1]
	if resource.AmqpItemType == nil || *resource.AmqpItemType != amqp.ItemType_RESOURCE_TYPE {
		t.Errorf("resource super type not mapped: %+v", resource)
	}

	if cube.AmqpItemType != nil {
		t.Errorf("cubes must not be served: %+v", cube)
	}
}

func newIndex() AlmanaxIndex {
	calendar := func(id int32, name, description string) entities.AlmanaxCalendar {
		return entities.AlmanaxCalendar{ID: id, Labels: []entities.AlmanaxCalendarLabel{
			{Locale: amqp.Language_FR, Name: name, Description: description},
			{Locale: amqp.Language_EN, Name: "Loot", Description: "English text"},
		}}
	}

	item := func(id, iconID int32, name string) entities.Item {
		return entities.Item{ID: id, IconID: iconID, Labels: []entities.ItemLabel{{Locale: amqp.Language_FR, Name: name}}}
	}

	calendars := []entities.AlmanaxCalendar{
		calendar(31, "Butin", "Les chances d'obtenir du butin sont augmentées de 125 % dans le donjon de la "+
			"<b>Caverne du Koulosse</b>."),
		calendar(331, "Points d'expérience", "Les gains d'expérience sont augmentés de 100 %."),
		calendar(75, "Points d'expérience", "Les gains d'expérience sont augmentés de 100 %."),
		calendar(5, "Ancien nom", "Description seule."),
	}

	items := []entities.Item{
		item(7905, 57001, "Laine du Boufcoul"),
		item(12132, 89002, "Cadeau de Nowel"),
		item(8337, 89003, "Cadeau de Nowel"),
	}

	return NewAlmanaxIndex(calendars, items, "fr")
}

func TestMapAlmanaxes(t *testing.T) {
	days := []dtos.KrosmozAlmanax{
		{Month: 10, Day: 16, BonusName: "Butin", BonusDescription: "Les chances d'obtenir du butin sont augmentées " +
			"de 125 % dans le donjon de la Caverne du Koulosse.", OfferingName: "Laine du Boufcoul",
			OfferingQuantity: 9, OfferingIconID: 57001},
		{Month: 12, Day: 25, BonusName: "Points d'expérience", BonusDescription: "Les gains d'expérience " +
			"sont augmentés de 100 %.", OfferingName: "Cadeau de Nowel", OfferingQuantity: 4, OfferingIconID: 89002},
		{Month: 1, Day: 1, BonusName: "Nouveau nom", BonusDescription: "Description seule.",
			OfferingName: "Laine du Boufcoul", OfferingQuantity: 1, OfferingIconID: 57001},
	}

	almanaxes, unmatched := MapAlmanaxes(days, newIndex())
	if len(unmatched) != 0 || len(almanaxes) != 3 {
		t.Fatalf("expected 3 matched days, got %d (unmatched %v)", len(almanaxes), unmatched)
	}

	tests := []struct {
		calendarID, itemID, quantity int32
	}{
		{31, 7905, 9},
		{75, 12132, 4}, // identical texts: lowest id; same name: the icon decides
		{5, 7905, 1},   // renamed bonus: matched on its description
	}

	for i, test := range tests {
		almanax := almanaxes[i]
		if *almanax.CalendarID != test.calendarID || *almanax.OfferingItemID != test.itemID ||
			*almanax.OfferingQuantity != test.quantity || almanax.ContentHash == nil {
			t.Errorf("day %02d-%02d: got (%d, %d, %d), expected %+v", almanax.Month, almanax.Day,
				*almanax.CalendarID, *almanax.OfferingItemID, *almanax.OfferingQuantity, test)
		}
	}
}

// A half that cannot be joined is NULL, the other half is kept (26/04: unknown bonus).
func TestMapAlmanaxesPartialDays(t *testing.T) {
	days := []dtos.KrosmozAlmanax{
		{Month: 4, Day: 26, BonusName: "Quêtes et kamas", BonusDescription: "Les gains de kamas sont augmentés.",
			OfferingName: "Laine du Boufcoul", OfferingQuantity: 2, OfferingIconID: 57001},
		{Month: 3, Day: 4, BonusName: "Butin", BonusDescription: "Les chances d'obtenir du butin sont augmentées " +
			"de 125 % dans le donjon de la Caverne du Koulosse.", OfferingName: "Laine du Boufcoul", OfferingIconID: 1},
		{Month: 3, Day: 5, BonusName: "Inconnu", BonusDescription: "Rien.", OfferingName: "Rien", OfferingIconID: 1},
	}

	almanaxes, unmatched := MapAlmanaxes(days, newIndex())
	if len(almanaxes) != 2 || len(unmatched) != 3 {
		t.Fatalf("expected 2 partial rows and 3 mismatches, got %d rows, %v", len(almanaxes), unmatched)
	}

	unknownBonus, unknownOffering := almanaxes[0], almanaxes[1]
	switch {
	case unknownBonus.CalendarID != nil || unknownBonus.OfferingItemID == nil || *unknownBonus.OfferingItemID != 7905 ||
		*unknownBonus.OfferingQuantity != 2:
		t.Errorf("unknown bonus: the offering must be kept, the bonus NULL: %+v", unknownBonus)
	case unknownOffering.CalendarID == nil || *unknownOffering.CalendarID != 31 ||
		unknownOffering.OfferingItemID != nil || unknownOffering.OfferingQuantity != nil:
		t.Errorf("unknown offering: the bonus must be kept, the offering NULL: %+v", unknownOffering)
	case unknownBonus.ContentHash == nil || unknownOffering.ContentHash == nil:
		t.Error("partial rows must have a content hash")
	}

	if !strings.HasPrefix(unmatched[0].String(), "04-26: no calendar entry") ||
		!strings.HasPrefix(unmatched[1].String(), "03-04: no item for offering") ||
		!strings.Contains(unmatched[2].String(), "no calendar entry") ||
		!strings.Contains(unmatched[2].String(), "; no item for offering") {
		t.Errorf("unexpected reasons: %v", unmatched)
	}
}
