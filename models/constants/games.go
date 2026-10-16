package constants

import amqp "github.com/kaellybot/kaelly-amqp"

const (
	// Game whose data is imported.
	Game = amqp.Game_DOFUS_TOUCH

	// Client data API tables.
	ClassItemTypes        = "ItemTypes"
	ClassItems            = "Items"
	ClassAlmanaxCalendars = "AlmanaxCalendars"

	// Krosmoz almanax crawl, stored as one raw file.
	ClassKrosmozAlmanax = "KrosmozAlmanax"

	// Language every other one is compared to by the validation gate.
	ReferenceLanguage = "en"

	// Records that must exist in every applied import (validation gate).
	ReferenceItemID            int32 = 7905 // Laine du Boufcoul
	ReferenceAlmanaxCalendarID int32 = 31   // Butin, Caverne du Koulosse

	// Above this ratio of names identical to the reference language, a language is
	// considered missing: Ankama silently answers in English for unknown languages.
	MaxIdenticalNameRatio = 0.5

	// AMQP routing key of import alerts.
	NewsImportRoutingKey = "news.import"
)

// GetClientClasses returns the client tables downloaded on every run, in apply order.
func GetClientClasses() []string {
	return []string{ClassItemTypes, ClassItems, ClassAlmanaxCalendars}
}
