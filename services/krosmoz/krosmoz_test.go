package krosmoz

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
)

func readFixture(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile("testdata/almanax-2026-10-16.html")
	if err != nil {
		t.Fatalf("cannot read fixture: %v", err)
	}

	return string(body)
}

// The Dofus and Touch blocks share the same id: the Touch one must be found by its title.
func TestExtractTouchBlockSkipsDofusBlock(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(readFixture(t)))
	if err != nil {
		t.Fatal(err)
	}

	block, err := extractTouchBlock(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(block, "DOFUS Touch") || strings.Contains(block, "Corne du Boufcoul") {
		t.Errorf("wrong block extracted: %s", block)
	}
}

func TestExtractTouchBlockMissing(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader("<html><body><p>maintenance</p></body></html>"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err = extractTouchBlock(doc); !errors.Is(err, ErrTouchBlockNotFound) {
		t.Errorf("expected ErrTouchBlockNotFound, got %v", err)
	}
}

func TestParseAlmanaxBlock(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(readFixture(t)))
	if err != nil {
		t.Fatal(err)
	}

	block, err := extractTouchBlock(doc)
	if err != nil {
		t.Fatal(err)
	}

	almanax, err := ParseAlmanaxBlock(block, "fr", 10, 16)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedDescription := "Les chances d'obtenir du butin sont augmentées de 125 % " +
		"dans le donjon de la Caverne du Koulosse."
	switch {
	case almanax.Month != 10 || almanax.Day != 16:
		t.Errorf("wrong day: %02d-%02d", almanax.Month, almanax.Day)
	case almanax.BonusName != "Butin":
		t.Errorf("wrong bonus name: %q", almanax.BonusName)
	case almanax.BonusDescription != expectedDescription:
		t.Errorf("wrong bonus description: %q", almanax.BonusDescription)
	case almanax.OfferingName != "Laine du Boufcoul":
		t.Errorf("wrong offering name: %q", almanax.OfferingName)
	case almanax.OfferingQuantity != 9:
		t.Errorf("wrong offering quantity: %d", almanax.OfferingQuantity)
	case almanax.OfferingIconID != 57001:
		t.Errorf("wrong offering icon: %d", almanax.OfferingIconID)
	}
}

func TestParseOffering(t *testing.T) {
	tests := map[string]struct {
		language string
		text     string
		quantity int32
		name     string
	}{
		"fr": {"fr", "Récupérer 9 Laine du Boufcoul et rapporter l'offrande à Théodoran Ax", 9, "Laine du Boufcoul"},
		"fr with et": {"fr", "Récupérer 2 Poudre et Paillettes et rapporter l'offrande à Théodoran Ax",
			2, "Poudre et Paillettes"},
		"en": {"en", "Find 9 Gobkool Wool and take the offering to Antyklime Ax", 9, "Gobkool Wool"},
		"es": {"es", "Recolectar 9 Lana de jalatranki y llevárselo a Ontoral Zo", 9, "Lana de jalatranki"},
		"de": {"de", "Sich 9 Fresscool-Wolle beschaffen und als Opfergabe bei Theodoran X abgeben",
			9, "Fresscool-Wolle"},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			quantity, itemName, err := parseOffering(test.text, test.language)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if quantity != test.quantity || itemName != test.name {
				t.Errorf("got (%d, %q), expected (%d, %q)", quantity, itemName, test.quantity, test.name)
			}
		})
	}
}

func TestParseOfferingErrors(t *testing.T) {
	if _, _, err := parseOffering("Find 9 Gobkool Wool and take the offering to Antyklime Ax", "pt"); !errors.Is(
		err, ErrUnsupportedLanguage) {
		t.Errorf("expected ErrUnsupportedLanguage, got %v", err)
	}

	if _, _, err := parseOffering("Something else", "fr"); !errors.Is(err, ErrUnexpectedOffering) {
		t.Errorf("expected ErrUnexpectedOffering, got %v", err)
	}
}

func TestParseIconID(t *testing.T) {
	iconID, err := parseIconID("https://static.ankama.com/dofus-touch/www/game/items/200/89002.w75h75.png")
	if err != nil || iconID != 89002 {
		t.Errorf("got (%d, %v), expected 89002", iconID, err)
	}

	if _, err = parseIconID("https://static.ankama.com/krosmoz/www/img/almanax/picto_step2.png"); !errors.Is(
		err, ErrOfferingIconNotFound) {
		t.Errorf("expected ErrOfferingIconNotFound, got %v", err)
	}
}

func TestGetAlmanaxDates(t *testing.T) {
	today := time.Date(2026, time.October, 16, 15, 30, 0, 0, time.UTC)
	dates := GetAlmanaxDates(today)
	if len(dates) != 366 {
		t.Fatalf("expected 366 dates, got %d", len(dates))
	}

	start := time.Date(2026, time.October, 16, 0, 0, 0, 0, time.UTC)
	seen := make(map[string]struct{}, len(dates))
	for _, date := range dates {
		if date.Before(start) {
			t.Errorf("%s is in the past", date.Format(time.DateOnly))
		}
		seen[date.Format("01-02")] = struct{}{}
	}

	if len(seen) != 366 {
		t.Errorf("expected 366 distinct days, got %d", len(seen))
	}

	expected := map[string]string{
		"02-29": "2028-02-29", // next leap year
		"10-16": "2026-10-16", // today counts
		"10-15": "2027-10-15", // yesterday is next year
		"01-01": "2027-01-01",
	}
	for _, date := range dates {
		if want, found := expected[date.Format("01-02")]; found && date.Format(time.DateOnly) != want {
			t.Errorf("%s: got %s, expected %s", date.Format("01-02"), date.Format(time.DateOnly), want)
		}
	}
}

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, time.October, 16, 12, 0, 0, 0, time.UTC)
	tests := map[string]time.Duration{
		"":                              0,
		"120":                           2 * time.Minute,
		"Fri, 16 Oct 2026 14:00:00 GMT": 2 * time.Hour,
		"Fri, 16 Oct 2026 10:00:00 GMT": 0, // in the past
		"soon":                          0,
	}

	for value, expected := range tests {
		if got := parseRetryAfter(value, now); got != expected {
			t.Errorf("parseRetryAfter(%q) = %v, expected %v", value, got, expected)
		}
	}
}

func TestHTTPErrorIsBlocked(t *testing.T) {
	for status, expected := range map[int]bool{
		http.StatusForbidden: true, http.StatusTooManyRequests: true, http.StatusServiceUnavailable: true,
		http.StatusInternalServerError: false, http.StatusNotFound: false,
	} {
		if got := (&HTTPError{StatusCode: status}).IsBlocked(); got != expected {
			t.Errorf("status %d: IsBlocked = %v, expected %v", status, got, expected)
		}
	}
}

func TestGetAlmanaxBlock(t *testing.T) {
	page := readFixture(t)
	var userAgent, path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userAgent, path = r.UserAgent(), r.URL.Path
		if strings.HasSuffix(r.URL.Path, "2026-10-17") {
			w.Header().Set("Retry-After", "3600")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(page))
	}))
	defer server.Close()

	service := &Impl{
		almanaxURL: server.URL + "/%s/almanax/%s",
		language:   "fr",
		userAgent:  "KaellyBot-Gamedata/test",
		client:     server.Client(),
	}

	block, err := service.GetAlmanaxBlock(context.Background(), time.Date(2026, time.October, 16, 0, 0, 0, 0, time.UTC))
	if err != nil || !strings.Contains(block, "DOFUS Touch") {
		t.Fatalf("unexpected result: %v", err)
	}

	if userAgent != "KaellyBot-Gamedata/test" || path != "/fr/almanax/2026-10-16" {
		t.Errorf("unexpected request: user agent %q, path %q", userAgent, path)
	}

	_, err = service.GetAlmanaxBlock(context.Background(), time.Date(2026, time.October, 17, 0, 0, 0, 0, time.UTC))
	var httpErr *HTTPError
	if !errors.Is(err, ErrUnexpectedStatus) || !errors.As(err, &httpErr) {
		t.Fatalf("expected an HTTPError, got %v", err)
	}

	if !httpErr.IsBlocked() || httpErr.RetryAfter != time.Hour {
		t.Errorf("a 429 with Retry-After 3600 must be a one hour block: %+v", httpErr)
	}
}
