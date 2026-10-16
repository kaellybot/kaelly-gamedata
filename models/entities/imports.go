package entities

import (
	"time"

	amqp "github.com/kaellybot/kaelly-amqp"
)

type ImportSource string

type ImportTrigger string

type ImportStatus string

const (
	ImportSourceClient  ImportSource = "CLIENT"
	ImportSourceKrosmoz ImportSource = "KROSMOZ"

	ImportTriggerScheduled ImportTrigger = "SCHEDULED"
	ImportTriggerManual    ImportTrigger = "MANUAL"

	ImportStatusRunning   ImportStatus = "RUNNING"
	ImportStatusApplied   ImportStatus = "APPLIED"
	ImportStatusPartial   ImportStatus = "PARTIAL"
	ImportStatusUnchanged ImportStatus = "UNCHANGED"
	ImportStatusRejected  ImportStatus = "REJECTED"
	ImportStatusFailed    ImportStatus = "FAILED"
)

// GameDataImport is one download: the client tables on every run, or a Krosmoz crawl.
type GameDataImport struct {
	ID            int64 `gorm:"primaryKey;autoIncrement"`
	Game          amqp.Game
	Source        ImportSource
	BuildVersion  *string
	AssetsVersion *string
	Trigger       ImportTrigger
	Status        ImportStatus
	StartedAt     time.Time
	FinishedAt    *time.Time
	Report        *string              `gorm:"type:json"`
	Files         []GameDataImportFile `gorm:"foreignKey:ImportID"`
}

// GameDataImportFile is one downloaded file, stored content-addressed in the raw snapshots.
type GameDataImportFile struct {
	ImportID  int64         `gorm:"primaryKey;autoIncrement:false"`
	Class     string        `gorm:"primaryKey"`
	Locale    amqp.Language `gorm:"primaryKey;autoIncrement:false"`
	Sha256    string        `gorm:"column:sha256"`
	SizeBytes int
	ObjectKey string
}
