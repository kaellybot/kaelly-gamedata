package imports

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/kaellybot/kaelly-gamedata/models/dtos"
	"github.com/kaellybot/kaelly-gamedata/models/entities"
	"github.com/kaellybot/kaelly-gamedata/services/krosmoz"
)

func january(dayOfMonth int) time.Time {
	return time.Date(2027, time.January, dayOfMonth, 0, 0, 0, 0, time.UTC)
}

func crawled(month, dayOfMonth int, at time.Time) (entities.AlmanaxKey, entities.AlmanaxCrawl) {
	return entities.AlmanaxKey{Month: month, Day: dayOfMonth},
		entities.AlmanaxCrawl{Month: month, Day: dayOfMonth, CrawledAt: at, Status: entities.AlmanaxCrawlMatched}
}

func TestSelectAlmanaxDates(t *testing.T) {
	now := time.Date(2026, time.October, 16, 12, 0, 0, 0, time.UTC)
	dates := []time.Time{january(1), january(2), january(3), january(4), january(5)}
	previous := make(map[entities.AlmanaxKey]entities.AlmanaxCrawl)
	for _, entry := range []struct {
		month, day int
		age        time.Duration
	}{
		{1, 1, 10 * 24 * time.Hour}, // due, crawled 10 days ago
		{1, 2, 1 * time.Hour},       // fresh
		{1, 4, 20 * 24 * time.Hour}, // due, oldest crawl
	} {
		key, crawl := crawled(entry.month, entry.day, now.Add(-entry.age))
		previous[key] = crawl
	}

	selected := selectAlmanaxDates(dates, previous, now, 7*24*time.Hour, 3)
	got := make([]string, 0, len(selected))
	for _, date := range selected {
		got = append(got, date.Format("01-02"))
	}

	// Never crawled first (01-03, 01-05), then the oldest crawl (01-04). 01-01 waits, 01-02 is fresh.
	if strings.Join(got, ",") != "01-03,01-05,01-04" {
		t.Errorf("unexpected selection: %v", got)
	}

	if all := selectAlmanaxDates(dates, previous, now, 7*24*time.Hour, 100); len(all) != 4 {
		t.Errorf("expected every due day (4), got %d", len(all))
	}
}

func TestBackoff(t *testing.T) {
	tests := []struct {
		streak     int
		retryAfter time.Duration
		expected   time.Duration
	}{
		{1, 0, 12 * time.Hour},
		{2, 0, 24 * time.Hour},
		{3, 0, 48 * time.Hour},
		{10, 0, 7 * 24 * time.Hour},         // capped
		{1, 30 * time.Hour, 30 * time.Hour}, // Krosmoz asks for longer
		{3, 1 * time.Hour, 48 * time.Hour},  // never shorter than our own backoff
		{50, 0, 7 * 24 * time.Hour},         // no overflow
		{2, 9 * 24 * time.Hour, 9 * 24 * time.Hour},
	}

	for _, test := range tests {
		if got := backoff(test.streak, test.retryAfter); got != test.expected {
			t.Errorf("backoff(%d, %v) = %v, expected %v", test.streak, test.retryAfter, got, test.expected)
		}
	}
}

func TestBlockedRetryAfter(t *testing.T) {
	blocked := fmt.Errorf("2026-10-17: %w", &krosmoz.HTTPError{StatusCode: http.StatusTooManyRequests,
		RetryAfter: time.Hour})
	if retryAfter, isBlocked := blockedRetryAfter(blocked); !isBlocked || retryAfter != time.Hour {
		t.Errorf("a wrapped 429 must be a block: %v, %v", retryAfter, isBlocked)
	}

	for _, err := range []error{
		&krosmoz.HTTPError{StatusCode: http.StatusInternalServerError},
		errors.New("connection reset"),
	} {
		if _, isBlocked := blockedRetryAfter(err); isBlocked {
			t.Errorf("%v must not be a block", err)
		}
	}
}

func TestSetStopReasonBacksOff(t *testing.T) {
	now := time.Date(2026, time.October, 16, 12, 0, 0, 0, time.UTC)
	service := &Impl{}
	run := almanaxRun{
		crawled:    []entities.AlmanaxKey{{Month: 1, Day: 1}},
		planned:    31,
		stopErr:    fmt.Errorf("2027-01-02: %w", &krosmoz.HTTPError{StatusCode: http.StatusForbidden}),
		lastStreak: 1,
		now:        now,
	}

	var result report
	service.setStopReason(&result, run)
	if result.BlockedStreak != 2 || result.BlockedUntil == nil || !result.BlockedUntil.Equal(now.Add(24*time.Hour)) {
		t.Errorf("second block in a row must back off 24h: %+v", result)
	}

	if !strings.HasPrefix(result.Reason, "crawl stopped after 1 of 31 days") {
		t.Errorf("unexpected reason: %s", result.Reason)
	}

	var unblocked report
	run.stopErr = errors.New("timeout")
	service.setStopReason(&unblocked, run)
	if unblocked.BlockedUntil != nil || unblocked.Reason == "" {
		t.Errorf("a non-block failure explains the stop without backing off: %+v", unblocked)
	}
}

func TestAlmanaxStatus(t *testing.T) {
	someChange := changes[entities.AlmanaxKey]{created: []entities.AlmanaxKey{{Month: 1, Day: 1}}}
	noChange := changes[entities.AlmanaxKey]{}
	stopped := errors.New("blocked")
	keys := []entities.AlmanaxKey{{Month: 1, Day: 1}}
	mismatch := []dtos.AlmanaxMismatch{{Month: 1, Day: 2}}

	tests := map[string]struct {
		run      almanaxRun
		diff     changes[entities.AlmanaxKey]
		expected entities.ImportStatus
	}{
		"nothing crawled":    {almanaxRun{stopErr: stopped}, noChange, entities.ImportStatusFailed},
		"stopped midway":     {almanaxRun{stopErr: stopped, crawled: keys}, someChange, entities.ImportStatusPartial},
		"unmatched day":      {almanaxRun{crawled: keys, mismatches: mismatch}, someChange, entities.ImportStatusPartial},
		"nothing changed":    {almanaxRun{crawled: keys}, noChange, entities.ImportStatusUnchanged},
		"some days imported": {almanaxRun{crawled: keys}, someChange, entities.ImportStatusApplied},
	}

	for name, test := range tests {
		if got := almanaxStatus(test.run, test.diff); got != test.expected {
			t.Errorf("%s: got %s, expected %s", name, got, test.expected)
		}
	}
}

func TestCrawlStatesAndNewlyUnmatched(t *testing.T) {
	now := time.Date(2026, time.October, 16, 12, 0, 0, 0, time.UTC)
	known := "no calendar entry for bonus \"kamas\""
	previous := map[entities.AlmanaxKey]entities.AlmanaxCrawl{
		{Month: 4, Day: 26}: {Month: 4, Day: 26, Status: entities.AlmanaxCrawlUnmatched, Reason: &known},
		{Month: 4, Day: 27}: {Month: 4, Day: 27, Status: entities.AlmanaxCrawlMatched},
	}

	mismatches := []dtos.AlmanaxMismatch{
		{Month: 4, Day: 26, Reason: known},                  // already known: not alerted again
		{Month: 4, Day: 27, Reason: "no DOFUS Touch block"}, // was matched: alerted
	}

	fresh := newlyUnmatched(mismatches, previous)
	if len(fresh) != 1 || fresh[0].Day != 27 {
		t.Errorf("only 04-27 is newly unmatched, got %v", fresh)
	}

	run := almanaxRun{
		crawled:    []entities.AlmanaxKey{{Month: 4, Day: 25}, {Month: 4, Day: 26}, {Month: 4, Day: 27}},
		mismatches: mismatches,
		now:        now,
	}
	states := crawlStates(run)
	if len(states) != 3 || states[0].Status != entities.AlmanaxCrawlMatched ||
		states[1].Status != entities.AlmanaxCrawlUnmatched || *states[2].Reason != "no DOFUS Touch block" ||
		!states[0].CrawledAt.Equal(now) {
		t.Errorf("unexpected crawl states: %+v", states)
	}

	if reason := alertReason(report{}, nil); reason != "" {
		t.Errorf("nothing new must not alert: %q", reason)
	}

	if reason := alertReason(report{Reason: "crawl stopped"}, fresh); !strings.HasPrefix(reason,
		"crawl stopped | 1 almanax day(s) unmatched: 04-27") {
		t.Errorf("unexpected alert: %q", reason)
	}
}

func TestSummarizeMismatches(t *testing.T) {
	mismatches := make([]dtos.AlmanaxMismatch, 0)
	for i := 1; i <= 6; i++ {
		mismatches = append(mismatches, dtos.AlmanaxMismatch{Month: 1, Day: i, Reason: "x"})
	}

	summary := summarizeMismatches(mismatches)
	if !strings.HasPrefix(summary, "6 almanax day(s) unmatched: 01-01: x") || strings.Contains(summary, "01-06") {
		t.Errorf("unexpected summary: %s", summary)
	}
}

func TestParseReport(t *testing.T) {
	until := "2026-10-17T12:00:00Z"
	value := `{"reason":"blocked","blockedUntil":"` + until + `","blockedStreak":2}`
	result := parseReport(&value)
	if result.BlockedStreak != 2 || result.BlockedUntil == nil || result.BlockedUntil.Format(time.RFC3339) != until {
		t.Errorf("unexpected report: %+v", result)
	}

	if empty := parseReport(nil); empty.BlockedUntil != nil {
		t.Errorf("a missing report has no backoff: %+v", empty)
	}
}
