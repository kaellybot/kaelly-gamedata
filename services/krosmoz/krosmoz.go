package krosmoz

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/kaellybot/kaelly-gamedata/models/constants"
	"github.com/kaellybot/kaelly-gamedata/models/dtos"
	"github.com/kaellybot/kaelly-gamedata/models/mappers"
	"github.com/spf13/viper"
)

func New() *Impl {
	return &Impl{
		almanaxURL: viper.GetString(constants.KrosmozAlmanaxURL),
		language:   viper.GetString(constants.KrosmozLanguage),
		userAgent:  viper.GetString(constants.KrosmozUserAgent),
		client:     &http.Client{Timeout: viper.GetDuration(constants.HTTPTimeout)},
	}
}

// GetAlmanaxBlock returns the HTML of the DOFUS Touch block of one almanax page.
// The Dofus and Touch blocks share the same id, so the block is found by its title.
func (service *Impl) GetAlmanaxBlock(ctx context.Context, date time.Time) (string, error) {
	pageURL := fmt.Sprintf(service.almanaxURL, service.language, date.Format(krosmozDateFormat))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", service.userAgent)
	resp, err := service.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", &HTTPError{
			StatusCode: resp.StatusCode,
			RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"), time.Now()),
			URL:        pageURL,
		}
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", err
	}

	return extractTouchBlock(doc)
}

func extractTouchBlock(doc *goquery.Document) (string, error) {
	var block *goquery.Selection
	doc.Find("div.achievement").EachWithBreak(func(_ int, selection *goquery.Selection) bool {
		if strings.Contains(selection.Find("h4").Text(), touchBlockTitle) {
			block = selection
			return false
		}
		return true
	})

	if block == nil {
		return "", ErrTouchBlockNotFound
	}

	return goquery.OuterHtml(block)
}

// ParseAlmanaxBlock reads a DOFUS Touch block, as returned by GetAlmanaxBlock.
func ParseAlmanaxBlock(block, language string, month, day int) (dtos.KrosmozAlmanax, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(block))
	if err != nil {
		return dtos.KrosmozAlmanax{}, err
	}

	mid := doc.Find("div.mid").First()
	more := mid.Find("div.more").First()
	infos := more.Find("div.more-infos").First()
	if more.Length() == 0 || infos.Length() == 0 {
		return dtos.KrosmozAlmanax{}, ErrTouchBlockNotFound
	}

	// "Bonus : Butin": the text of .mid without the description.
	nameText := mid.Clone()
	nameText.Find("div.more").Remove()
	_, bonusName, _ := strings.Cut(mappers.NormalizeText(nameText.Text()), ":")

	description := more.Clone()
	description.Find("div.more-infos").Remove()

	quantity, offeringName, err := parseOffering(mappers.NormalizeText(infos.Find("p.fleft").Text()), language)
	if err != nil {
		return dtos.KrosmozAlmanax{}, err
	}

	iconSrc, _ := infos.Find("div.more-infos-content img").Attr("src")
	iconID, err := parseIconID(iconSrc)
	if err != nil {
		return dtos.KrosmozAlmanax{}, err
	}

	return dtos.KrosmozAlmanax{
		Month:            month,
		Day:              day,
		BonusName:        strings.TrimSpace(bonusName),
		BonusDescription: mappers.NormalizeText(description.Text()),
		OfferingName:     offeringName,
		OfferingQuantity: quantity,
		OfferingIconID:   iconID,
	}, nil
}

// getOfferingSentences returns, per language, the offering sentence with the quantity
// and the item name as groups. Portuguese pages are served in English.
func getOfferingSentences() map[string]string {
	return map[string]string{
		"fr": `^Récupérer (\d+) (.+) et rapporter l'offrande à .+$`,
		"en": `^Find (\d+) (.+) and take the offering to .+$`,
		"es": `^Recolectar (\d+) (.+) y llevárselo a .+$`,
		"de": `^Sich (\d+) (.+) beschaffen und als Opfergabe bei .+ abgeben$`,
	}
}

func parseOffering(text, language string) (int32, string, error) {
	sentence, found := getOfferingSentences()[language]
	if !found {
		return 0, "", fmt.Errorf("%w: %s", ErrUnsupportedLanguage, language)
	}

	match := regexp.MustCompile(sentence).FindStringSubmatch(text)
	if match == nil {
		return 0, "", fmt.Errorf("%w: %q", ErrUnexpectedOffering, text)
	}

	quantity, err := strconv.ParseInt(match[1], 10, 32)
	if err != nil {
		return 0, "", err
	}

	return int32(quantity), match[2], nil
}

// parseIconID reads the icon id from .../game/items/200/<iconId>.w75h75.png.
func parseIconID(src string) (int32, error) {
	match := regexp.MustCompile(`/items/\d+/(\d+)\.`).FindStringSubmatch(src)
	if match == nil {
		return 0, fmt.Errorf("%w: %q", ErrOfferingIconNotFound, src)
	}

	iconID, err := strconv.ParseInt(match[1], 10, 32)
	if err != nil {
		return 0, err
	}

	return int32(iconID), nil
}

// parseRetryAfter reads a Retry-After header, in seconds or as an HTTP date. 0 if absent.
func parseRetryAfter(value string, now time.Time) time.Duration {
	if value == "" {
		return 0
	}

	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}

	if date, err := http.ParseTime(value); err == nil && date.After(now) {
		return date.Sub(now)
	}

	return 0
}

// GetAlmanaxDates returns the 366 days of the almanax cycle, each at its next
// occurrence from today. 29/02 is taken from the next leap year.
func GetAlmanaxDates(today time.Time) []time.Time {
	start := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	dates := make([]time.Time, 0)
	day := time.Date(leapYear, time.January, 1, 0, 0, 0, 0, time.UTC)
	for ; day.Year() == leapYear; day = day.AddDate(0, 0, 1) {
		dates = append(dates, nextOccurrence(start, day.Month(), day.Day()))
	}

	return dates
}

func nextOccurrence(start time.Time, month time.Month, day int) time.Time {
	for year := start.Year(); ; year++ {
		candidate := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
		// time.Date turns 29/02 of a common year into 01/03: skip those years.
		if candidate.Month() == month && candidate.Day() == day && !candidate.Before(start) {
			return candidate
		}
	}
}
