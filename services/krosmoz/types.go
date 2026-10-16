package krosmoz

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

const (
	// Delay between two Krosmoz pages, to keep the crawl gentle.
	RequestDelay = 500 * time.Millisecond

	// Every language shows the Touch block under a title containing this.
	touchBlockTitle = "DOFUS Touch"

	// A leap year, to enumerate the 366 days of the almanax cycle.
	leapYear = 2028

	krosmozDateFormat = "2006-01-02"
)

// HTTPError is a page answered with another status than 200.
type HTTPError struct {
	StatusCode int
	RetryAfter time.Duration
	URL        string
}

func (err *HTTPError) Error() string {
	return fmt.Sprintf("%v: %d %s on %s", ErrUnexpectedStatus, err.StatusCode, http.StatusText(err.StatusCode), err.URL)
}

func (err *HTTPError) Unwrap() error {
	return ErrUnexpectedStatus
}

// IsBlocked reports a refusal to serve us (forbidden, rate limited, unavailable):
// the crawl must stop at once and back off, not retry.
func (err *HTTPError) IsBlocked() bool {
	switch err.StatusCode {
	case http.StatusForbidden, http.StatusTooManyRequests, http.StatusServiceUnavailable:
		return true
	default:
		return false
	}
}

var (
	ErrUnexpectedStatus     = errors.New("unexpected HTTP status")
	ErrTouchBlockNotFound   = errors.New("no DOFUS Touch block on the almanax page")
	ErrUnexpectedOffering   = errors.New("offering text does not match the expected sentence")
	ErrUnsupportedLanguage  = errors.New("no offering sentence known for this language")
	ErrOfferingIconNotFound = errors.New("no offering icon id in the image URL")
)

type Service interface {
	GetAlmanaxBlock(ctx context.Context, date time.Time) (string, error)
}

type Impl struct {
	almanaxURL string
	language   string
	userAgent  string
	client     *http.Client
}
