package imports

import (
	"errors"
	"time"

	amqp "github.com/kaellybot/kaelly-amqp"
	"github.com/kaellybot/kaelly-gamedata/models/dtos"
	"github.com/kaellybot/kaelly-gamedata/models/entities"
	"github.com/kaellybot/kaelly-gamedata/models/mappers"
	"github.com/kaellybot/kaelly-gamedata/repositories/almanaxes"
	"github.com/kaellybot/kaelly-gamedata/repositories/gamedataimports"
	"github.com/kaellybot/kaelly-gamedata/repositories/gameversions"
	"github.com/kaellybot/kaelly-gamedata/repositories/items"
	"github.com/kaellybot/kaelly-gamedata/services/krosmoz"
	"github.com/kaellybot/kaelly-gamedata/services/storages"
	"github.com/kaellybot/kaelly-gamedata/services/touch"
	"github.com/kaellybot/kaelly-gamedata/utils/databases"
)

const (
	snapshotPrefix    = "raw/touch"
	snapshotKeyFormat = snapshotPrefix + "/%s.json.gz"

	// Unmatched almanax days quoted in an alert; the import report lists them all.
	maxQuotedUnmatched = 5

	// Delay before retrying a Krosmoz page once.
	krosmozRetryDelay = 5 * time.Second

	// After a block, the next crawl waits 12h, then 24h, 48h... up to 7 days.
	krosmozBackoffBase = 12 * time.Hour
	krosmozBackoffMax  = 7 * 24 * time.Hour

	tableItemTypes = "item_types"
	tableItems     = "items"
	tableCalendars = "almanax_calendars"
	tableAlmanaxes = "almanaxes"
)

var (
	ErrLanguageNotDownloaded = errors.New("language is not among the downloaded Touch languages")
	ErrInvalidDaysPerRun     = errors.New("KROSMOZ_DAYS_PER_RUN must be positive")
)

type Service interface {
	Import() error
}

type Impl struct {
	languages              []string
	krosmozLanguage        string
	almanaxRefreshInterval time.Duration
	krosmozDaysPerRun      int
	maxDeletedRatio        float64
	snapshotRetention      int
	dryRun                 bool
	touchService           touch.Service
	krosmozService         krosmoz.Service
	storageService         storages.Service
	importRepo             gamedataimports.Repository
	itemRepo               items.Repository
	almanaxRepo            almanaxes.Repository
	gameVersionRepo        gameversions.Repository
	db                     databases.MySQLConnection
	broker                 amqp.MessageBroker
}

// clientData is the client data of one run, mapped. The almanax join relies on it.
type clientData struct {
	itemTypes []entities.ItemType
	items     []entities.Item
	calendars []entities.AlmanaxCalendar
	stats     map[string]mappers.LanguageStats
}

type download struct {
	class    string
	language string
	body     []byte
}

type report struct {
	Tables        map[string]tableReport `json:"tables,omitempty"`
	Unmatched     []string               `json:"unmatched,omitempty"`
	Reason        string                 `json:"reason,omitempty"`
	Crawled       int                    `json:"crawled,omitempty"`
	BlockedUntil  *time.Time             `json:"blockedUntil,omitempty"`
	BlockedStreak int                    `json:"blockedStreak,omitempty"`
}

// almanaxRun is the outcome of one Krosmoz crawl, before it is applied.
type almanaxRun struct {
	almanaxes  []entities.Almanax
	mismatches []dtos.AlmanaxMismatch
	crawled    []entities.AlmanaxKey
	planned    int
	previous   map[entities.AlmanaxKey]entities.AlmanaxCrawl
	stopErr    error
	lastStreak int
	now        time.Time
}

type tableReport struct {
	Total   int `json:"total"`
	Created int `json:"created"`
	Updated int `json:"updated"`
	Deleted int `json:"deleted"`
}
