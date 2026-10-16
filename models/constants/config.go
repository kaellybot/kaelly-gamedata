package constants

import (
	"time"

	"github.com/rs/zerolog"
)

const (
	ConfigFileName = ".env"

	// MySQL URL with the following format: HOST:PORT.
	MySQLURL = "MYSQL_URL"

	// MySQL user.
	MySQLUser = "MYSQL_USER"

	// MySQL password.
	MySQLPassword = "MYSQL_PASSWORD"

	// MySQL database name.
	MySQLDatabase = "MYSQL_DATABASE"

	// RabbitMQ address.
	RabbitMQAddress = "RABBITMQ_ADDRESS"

	// Dofus Touch client config, giving the data and assets URLs.
	TouchConfigURL = "TOUCH_CONFIG_URL"

	// Comma-separated languages downloaded from the Dofus Touch client data API.
	TouchLanguages = "TOUCH_LANGUAGES"

	// User Agent to call the Dofus Touch client data API and the static icons.
	TouchUserAgent = "TOUCH_USER_AGENT"

	// Krosmoz almanax URL, formatted with the language and the date (YYYY-MM-DD).
	KrosmozAlmanaxURL = "KROSMOZ_ALMANAX_URL"

	// Only language crawled on Krosmoz; every label comes from the client data.
	KrosmozLanguage = "KROSMOZ_LANGUAGE"

	// User Agent to call Krosmoz. The KaellyBot IP is whitelisted, a browser one is the fallback.
	KrosmozUserAgent = "KROSMOZ_USER_AGENT"

	// Minimum duration between two Krosmoz crawls. Duration type.
	AlmanaxRefreshInterval = "ALMANAX_REFRESH_INTERVAL"

	// Ratio of deleted rows in a table above which an import is rejected.
	MaxDeletedRatio = "MAX_DELETED_RATIO"

	// Number of applied imports whose raw snapshots are kept.
	SnapshotRetention = "SNAPSHOT_RETENTION"

	// Volume path: public/ is served by the CDN, raw/ is private.
	StoragePath = "STORAGE_PATH"

	// CDN base URL, prefixing the stored image keys.
	CDNBaseURL = "CDN_BASE_URL"

	// HTTP timeout for every external call. Duration type.
	HTTPTimeout = "HTTP_TIMEOUT"

	// Boolean; download, diff and report, but write nothing.
	DryRun = "DRY_RUN"

	// Metric port.
	MetricPort = "METRIC_PORT"

	// Zerolog values from [trace, debug, info, warn, error, fatal, panic].
	LogLevel = "LOG_LEVEL"

	// Boolean; true when running in production.
	Production = "PRODUCTION"

	defaultMySQLURL               = "localhost:3306"
	defaultMySQLUser              = ""
	defaultMySQLPassword          = ""
	defaultMySQLDatabase          = "kaellybot"
	defaultRabbitMQAddress        = "amqp://localhost:5672"
	defaultTouchConfigURL         = "https://dt-proxy-production-login.ankama-games.com/config.json"
	defaultTouchLanguages         = "fr,en,es,pt"
	defaultUserAgent              = "KaellyBot-Gamedata/1.0 (+https://github.com/kaellybot/kaelly-gamedata)"
	defaultKrosmozAlmanaxURL      = "https://www.krosmoz.com/%s/almanax/%s"
	defaultKrosmozLanguage        = "fr"
	defaultAlmanaxRefreshInterval = 30 * 24 * time.Hour
	defaultMaxDeletedRatio        = 0.05
	defaultSnapshotRetention      = 10
	defaultStoragePath            = "/data"
	defaultCDNBaseURL             = "https://cdn.kaellybot.com/"
	defaultHTTPTimeout            = 60 * time.Second
	defaultDryRun                 = false
	defaultMetricPort             = 2112
	defaultLogLevel               = zerolog.InfoLevel
	defaultProduction             = false
)

func GetDefaultConfigValues() map[string]any {
	return map[string]any{
		MySQLURL:               defaultMySQLURL,
		MySQLUser:              defaultMySQLUser,
		MySQLPassword:          defaultMySQLPassword,
		MySQLDatabase:          defaultMySQLDatabase,
		RabbitMQAddress:        defaultRabbitMQAddress,
		TouchConfigURL:         defaultTouchConfigURL,
		TouchLanguages:         defaultTouchLanguages,
		TouchUserAgent:         defaultUserAgent,
		KrosmozAlmanaxURL:      defaultKrosmozAlmanaxURL,
		KrosmozLanguage:        defaultKrosmozLanguage,
		KrosmozUserAgent:       defaultUserAgent,
		AlmanaxRefreshInterval: defaultAlmanaxRefreshInterval,
		MaxDeletedRatio:        defaultMaxDeletedRatio,
		SnapshotRetention:      defaultSnapshotRetention,
		StoragePath:            defaultStoragePath,
		CDNBaseURL:             defaultCDNBaseURL,
		HTTPTimeout:            defaultHTTPTimeout,
		DryRun:                 defaultDryRun,
		MetricPort:             defaultMetricPort,
		LogLevel:               defaultLogLevel.String(),
		Production:             defaultProduction,
	}
}
