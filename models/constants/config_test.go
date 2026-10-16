package constants

import (
	"slices"
	"strings"
	"testing"
)

// Dofus Touch has no German: the client data API silently answers in English.
func TestDefaultTouchLanguagesExcludeGerman(t *testing.T) {
	languages := strings.Split(defaultTouchLanguages, ",")
	if slices.Contains(languages, "de") {
		t.Errorf("default Touch languages %v must not contain de", languages)
	}

	for _, expected := range []string{"fr", "en", "es", "pt"} {
		if !slices.Contains(languages, expected) {
			t.Errorf("default Touch languages %v miss %s", languages, expected)
		}
	}
}

// The Krosmoz crawl is matched against the client labels of the same language.
func TestDefaultKrosmozLanguageIsDownloaded(t *testing.T) {
	languages := strings.Split(defaultTouchLanguages, ",")
	if !slices.Contains(languages, defaultKrosmozLanguage) {
		t.Errorf("Krosmoz language %s is not among the Touch languages %v",
			defaultKrosmozLanguage, languages)
	}
}

func TestDefaultGateValues(t *testing.T) {
	if defaultMaxDeletedRatio <= 0 || defaultMaxDeletedRatio >= 1 {
		t.Errorf("max deleted ratio %v must be in ]0, 1[", defaultMaxDeletedRatio)
	}

	if defaultSnapshotRetention < 1 {
		t.Errorf("snapshot retention %v must keep at least the last applied import",
			defaultSnapshotRetention)
	}
}

func TestEveryDefaultIsRegistered(t *testing.T) {
	keys := []string{
		MySQLURL, MySQLUser, MySQLPassword, MySQLDatabase, RabbitMQAddress,
		TouchConfigURL, TouchLanguages, TouchUserAgent,
		KrosmozAlmanaxURL, KrosmozLanguage, KrosmozUserAgent, AlmanaxRefreshInterval,
		MaxDeletedRatio, SnapshotRetention, StoragePath, CDNBaseURL, HTTPTimeout, DryRun,
		MetricPort, LogLevel, Production,
	}

	defaults := GetDefaultConfigValues()
	for _, key := range keys {
		if _, found := defaults[key]; !found {
			t.Errorf("config key %s has no default value", key)
		}
	}
}
