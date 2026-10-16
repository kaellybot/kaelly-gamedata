package entities

import (
	"time"

	amqp "github.com/kaellybot/kaelly-amqp"
)

// AlmanaxCalendar is one almanax bonus, from the client data. It has no date.
type AlmanaxCalendar struct {
	ID          int32     `gorm:"primaryKey;autoIncrement:false"`
	Game        amqp.Game `gorm:"primaryKey;autoIncrement:false"`
	NpcID       int32
	ContentHash string                 `json:"-"`
	Labels      []AlmanaxCalendarLabel `gorm:"-:all"`
}

type AlmanaxCalendarLabel struct {
	CalendarID  int32         `gorm:"primaryKey;autoIncrement:false"`
	Game        amqp.Game     `gorm:"primaryKey;autoIncrement:false"`
	Locale      amqp.Language `gorm:"primaryKey;autoIncrement:false"`
	Name        string
	Description string
}

// Almanax is one day of the yearly cycle. Dofus rows use DofusDudeEffectID,
// Dofus Touch rows use the calendar entry and the offering.
type Almanax struct {
	Day               int       `gorm:"primaryKey;autoIncrement:false"`
	Month             int       `gorm:"primaryKey;autoIncrement:false"`
	Game              amqp.Game `gorm:"primaryKey;autoIncrement:false"`
	DofusDudeEffectID *string
	CalendarID        *int32
	OfferingItemID    *int32
	OfferingQuantity  *int32
	KamasReward       *int32
	ContentHash       *string `json:"-"`
}

// AlmanaxKey identifies a day of the yearly cycle.
type AlmanaxKey struct {
	Month int
	Day   int
}

type AlmanaxCrawlStatus string

const (
	AlmanaxCrawlMatched   AlmanaxCrawlStatus = "MATCHED"
	AlmanaxCrawlUnmatched AlmanaxCrawlStatus = "UNMATCHED"
)

// AlmanaxCrawl is the last Krosmoz crawl of one day of the cycle.
type AlmanaxCrawl struct {
	Month     int       `gorm:"primaryKey;autoIncrement:false"`
	Day       int       `gorm:"primaryKey;autoIncrement:false"`
	Game      amqp.Game `gorm:"primaryKey;autoIncrement:false"`
	CrawledAt time.Time
	Status    AlmanaxCrawlStatus
	Reason    *string
}
