package imports

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/kaellybot/kaelly-gamedata/models/constants"
	"github.com/kaellybot/kaelly-gamedata/models/dtos"
	"github.com/kaellybot/kaelly-gamedata/models/entities"
	"github.com/kaellybot/kaelly-gamedata/models/mappers"
	"github.com/kaellybot/kaelly-gamedata/services/krosmoz"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// importAlmanax crawls a batch of Krosmoz days (rolling crawl) and joins it to the
// client data of this run (spec §5.5).
func (service *Impl) importAlmanax(ctx context.Context, data *clientData) error {
	if data == nil {
		log.Warn().Str(constants.LogGame, constants.Game.String()).
			Msgf("No valid client data in this run, almanax skipped")
		return nil
	}

	now := time.Now().UTC()
	lastStreak, backedOff, err := service.getKrosmozBackoff(now)
	if err != nil || backedOff {
		return err
	}

	previous, err := service.almanaxRepo.GetCrawls(constants.Game)
	if err != nil {
		return err
	}

	dates := selectAlmanaxDates(krosmoz.GetAlmanaxDates(now), previous, now,
		service.almanaxRefreshInterval, service.krosmozDaysPerRun)
	if len(dates) == 0 {
		log.Info().Str(constants.LogGame, constants.Game.String()).
			Msgf("Every almanax day was crawled recently, nothing to do")
		return nil
	}

	record := service.newImport(entities.ImportSourceKrosmoz)
	if errSave := service.save(&record); errSave != nil {
		return errSave
	}

	blocks, stopErr := service.crawlAlmanax(ctx, dates)
	if len(blocks) > 0 {
		body, errJSON := json.Marshal(blocks)
		if errJSON != nil {
			return service.stop(&record, entities.ImportStatusFailed, "snapshot: "+errJSON.Error())
		}

		snapshot := download{class: constants.ClassKrosmozAlmanax, language: service.krosmozLanguage, body: body}
		if errStore := service.storeSnapshots(&record, []download{snapshot}); errStore != nil {
			return errStore
		}
	}

	days, mismatches := service.parseAlmanax(blocks)
	index := mappers.NewAlmanaxIndex(data.calendars, data.items, service.krosmozLanguage)
	almanaxes, joinMismatches := mappers.MapAlmanaxes(days, index)
	return service.applyAlmanax(&record, almanaxRun{
		almanaxes:  almanaxes,
		mismatches: append(mismatches, joinMismatches...),
		crawled:    crawledKeys(blocks),
		planned:    len(dates),
		previous:   previous,
		stopErr:    stopErr,
		lastStreak: lastStreak,
		now:        now,
	})
}

// getKrosmozBackoff reads the last Krosmoz import: after a block, crawling waits
// until its blockedUntil. It returns the current block streak.
func (service *Impl) getKrosmozBackoff(now time.Time) (int, bool, error) {
	last, found, err := service.importRepo.GetLast(constants.Game, entities.ImportSourceKrosmoz,
		[]entities.ImportStatus{
			entities.ImportStatusApplied, entities.ImportStatusPartial, entities.ImportStatusUnchanged,
			entities.ImportStatusFailed, entities.ImportStatusRejected,
		})
	if err != nil || !found {
		return 0, false, err
	}

	result := parseReport(last.Report)
	if result.BlockedUntil != nil && now.Before(*result.BlockedUntil) {
		log.Warn().Str(constants.LogGame, constants.Game.String()).
			Time("blockedUntil", *result.BlockedUntil).
			Int("blockedStreak", result.BlockedStreak).
			Msgf("Krosmoz blocked the last crawl, backing off")
		return result.BlockedStreak, true, nil
	}

	return result.BlockedStreak, false, nil
}

// selectAlmanaxDates returns the days to crawl: never crawled first, then the oldest
// crawls, skipping the days crawled within the refresh interval, at most limit.
func selectAlmanaxDates(dates []time.Time, previous map[entities.AlmanaxKey]entities.AlmanaxCrawl,
	now time.Time, refreshInterval time.Duration, limit int) []time.Time {
	type candidate struct {
		date      time.Time
		crawledAt time.Time
	}

	candidates := make([]candidate, 0, len(dates))
	for _, date := range dates {
		crawl, found := previous[entities.AlmanaxKey{Month: int(date.Month()), Day: date.Day()}]
		if found && now.Sub(crawl.CrawledAt) < refreshInterval {
			continue
		}

		// A never-crawled day has a zero crawledAt: it sorts first.
		candidates = append(candidates, candidate{date: date, crawledAt: crawl.CrawledAt})
	}

	slices.SortStableFunc(candidates, func(a, b candidate) int {
		if byAge := a.crawledAt.Compare(b.crawledAt); byAge != 0 {
			return byAge
		}
		return a.date.Compare(b.date)
	})

	result := make([]time.Time, 0, min(limit, len(candidates)))
	for _, candidate := range candidates[:min(limit, len(candidates))] {
		result = append(result, candidate.date)
	}

	return result
}

// crawlAlmanax fetches the Touch block of each date, one page at a time. A page
// without a Touch block is kept empty. Any other failure stops the crawl: the pages
// fetched so far are returned with the error.
func (service *Impl) crawlAlmanax(ctx context.Context, dates []time.Time) (map[string]string, error) {
	log.Info().Int("days", len(dates)).Msgf("Crawling Krosmoz almanax...")
	blocks := make(map[string]string, len(dates))
	for i, date := range dates {
		if i > 0 {
			time.Sleep(krosmoz.RequestDelay)
		}

		block, err := service.getAlmanaxBlock(ctx, date)
		if err != nil && !errors.Is(err, krosmoz.ErrTouchBlockNotFound) {
			return blocks, fmt.Errorf("%s: %w", date.Format(time.DateOnly), err)
		}

		blocks[date.Format(time.DateOnly)] = block
	}

	return blocks, nil
}

// getAlmanaxBlock retries a failed page once, unless Krosmoz refused to serve us.
func (service *Impl) getAlmanaxBlock(ctx context.Context, date time.Time) (string, error) {
	block, err := service.krosmozService.GetAlmanaxBlock(ctx, date)
	if err == nil || errors.Is(err, krosmoz.ErrTouchBlockNotFound) {
		return block, err
	}

	if _, blocked := blockedRetryAfter(err); blocked {
		return "", err
	}

	log.Warn().Err(err).Str(constants.LogDate, date.Format(time.DateOnly)).Msgf("Krosmoz page failed, retrying once")
	time.Sleep(krosmozRetryDelay)
	return service.krosmozService.GetAlmanaxBlock(ctx, date)
}

func (service *Impl) parseAlmanax(blocks map[string]string) ([]dtos.KrosmozAlmanax, []dtos.AlmanaxMismatch) {
	dates := make([]string, 0, len(blocks))
	for date := range blocks {
		dates = append(dates, date)
	}
	slices.Sort(dates)

	days := make([]dtos.KrosmozAlmanax, 0, len(blocks))
	mismatches := make([]dtos.AlmanaxMismatch, 0)
	for _, value := range dates {
		date, err := time.Parse(time.DateOnly, value)
		if err != nil {
			continue
		}

		month, day := int(date.Month()), date.Day()
		if blocks[value] == "" {
			mismatches = append(mismatches, dtos.AlmanaxMismatch{Month: month, Day: day, Reason: "no DOFUS Touch block"})
			continue
		}

		almanax, err := krosmoz.ParseAlmanaxBlock(blocks[value], service.krosmozLanguage, month, day)
		if err != nil {
			mismatches = append(mismatches, dtos.AlmanaxMismatch{Month: month, Day: day, Reason: err.Error()})
			continue
		}

		days = append(days, almanax)
	}

	return days, mismatches
}

// applyAlmanax upserts the joined days (fully or by half) and the crawl state of every
// crawled day. A day with nothing joined, or whose page could not be read, keeps its
// previous row.
func (service *Impl) applyAlmanax(record *entities.GameDataImport, run almanaxRun) error {
	current, err := service.almanaxRepo.GetAlmanaxHashes(constants.Game)
	if err != nil {
		return err
	}

	key := func(almanax entities.Almanax) entities.AlmanaxKey {
		return entities.AlmanaxKey{Month: almanax.Month, Day: almanax.Day}
	}
	diff := computeChanges(current, hashes(run.almanaxes, key, func(almanax entities.Almanax) string {
		return *almanax.ContentHash
	}))
	diff.deleted = nil

	result := report{
		Tables:    map[string]tableReport{tableAlmanaxes: diff.report()},
		Unmatched: mismatchStrings(run.mismatches),
		Crawled:   len(run.crawled),
	}
	service.setStopReason(&result, run)
	status := almanaxStatus(run, diff)

	if service.dryRun {
		service.finish(record, status, result)
		return nil
	}

	err = service.db.GetDB().Transaction(func(tx *gorm.DB) error {
		if errSave := service.almanaxRepo.SaveAlmanaxes(tx, upserts(run.almanaxes, key, diff)); errSave != nil {
			return errSave
		}

		if errCrawls := service.almanaxRepo.SaveCrawls(tx, crawlStates(run)); errCrawls != nil {
			return errCrawls
		}

		service.finish(record, status, result)
		return service.importRepo.Save(tx, record)
	})
	if err != nil {
		return service.stop(record, entities.ImportStatusFailed, "apply: "+err.Error())
	}

	if reason := alertReason(result, newlyUnmatched(run.mismatches, run.previous)); reason != "" {
		service.alert(*record, reason)
	}

	service.cleanSnapshots()
	return nil
}

// setStopReason explains an interrupted crawl and, after a block, sets the backoff.
func (service *Impl) setStopReason(result *report, run almanaxRun) {
	if run.stopErr == nil {
		return
	}

	result.Reason = fmt.Sprintf("crawl stopped after %d of %d days: %v", len(run.crawled), run.planned, run.stopErr)
	if retryAfter, blocked := blockedRetryAfter(run.stopErr); blocked {
		streak := run.lastStreak + 1
		until := run.now.Add(backoff(streak, retryAfter))
		result.BlockedUntil = &until
		result.BlockedStreak = streak
		result.Reason += fmt.Sprintf("; blocked %d time(s) in a row, next crawl after %s",
			streak, until.Format(time.RFC3339))
	}
}

func almanaxStatus(run almanaxRun, diff changes[entities.AlmanaxKey]) entities.ImportStatus {
	switch {
	case run.stopErr != nil && len(run.crawled) == 0:
		return entities.ImportStatusFailed
	case run.stopErr != nil, len(run.mismatches) > 0:
		return entities.ImportStatusPartial
	case diff.isEmpty():
		return entities.ImportStatusUnchanged
	default:
		return entities.ImportStatusApplied
	}
}

// crawlStates records when each crawled day was crawled and whether it matched.
func crawlStates(run almanaxRun) []entities.AlmanaxCrawl {
	reasons := make(map[entities.AlmanaxKey]string, len(run.mismatches))
	for _, mismatch := range run.mismatches {
		reasons[entities.AlmanaxKey{Month: mismatch.Month, Day: mismatch.Day}] = mismatch.Reason
	}

	crawls := make([]entities.AlmanaxCrawl, 0, len(run.crawled))
	for _, key := range run.crawled {
		crawl := entities.AlmanaxCrawl{Month: key.Month, Day: key.Day, Game: constants.Game,
			CrawledAt: run.now, Status: entities.AlmanaxCrawlMatched}
		if reason, found := reasons[key]; found {
			crawl.Status = entities.AlmanaxCrawlUnmatched
			crawl.Reason = &reason
		}
		crawls = append(crawls, crawl)
	}

	return crawls
}

// newlyUnmatched keeps the mismatches not already known from the previous crawl, so
// a day that stays unmatched is alerted once, not on every refresh.
func newlyUnmatched(mismatches []dtos.AlmanaxMismatch, previous map[entities.AlmanaxKey]entities.AlmanaxCrawl,
) []dtos.AlmanaxMismatch {
	result := make([]dtos.AlmanaxMismatch, 0)
	for _, mismatch := range mismatches {
		crawl, found := previous[entities.AlmanaxKey{Month: mismatch.Month, Day: mismatch.Day}]
		if found && crawl.Status == entities.AlmanaxCrawlUnmatched && crawl.Reason != nil &&
			*crawl.Reason == mismatch.Reason {
			continue
		}
		result = append(result, mismatch)
	}

	return result
}

// alertReason returns what to alert about: an interrupted crawl, new mismatches, or "".
func alertReason(result report, newMismatches []dtos.AlmanaxMismatch) string {
	reasons := make([]string, 0)
	if result.Reason != "" {
		reasons = append(reasons, result.Reason)
	}

	if len(newMismatches) > 0 {
		reasons = append(reasons, summarizeMismatches(newMismatches))
	}

	return strings.Join(reasons, " | ")
}

// backoff doubles from krosmozBackoffBase with each block in a row, up to
// krosmozBackoffMax, and never waits less than Krosmoz asked for.
func backoff(streak int, retryAfter time.Duration) time.Duration {
	delay := krosmozBackoffBase
	for i := 1; i < streak && delay < krosmozBackoffMax; i++ {
		delay *= 2
	}

	return max(min(delay, krosmozBackoffMax), retryAfter)
}

func blockedRetryAfter(err error) (time.Duration, bool) {
	var httpErr *krosmoz.HTTPError
	if errors.As(err, &httpErr) && httpErr.IsBlocked() {
		return httpErr.RetryAfter, true
	}

	return 0, false
}

func crawledKeys(blocks map[string]string) []entities.AlmanaxKey {
	keys := make([]entities.AlmanaxKey, 0, len(blocks))
	for value := range blocks {
		if date, err := time.Parse(time.DateOnly, value); err == nil {
			keys = append(keys, entities.AlmanaxKey{Month: int(date.Month()), Day: date.Day()})
		}
	}

	slices.SortFunc(keys, func(a, b entities.AlmanaxKey) int {
		return cmp.Or(cmp.Compare(a.Month, b.Month), cmp.Compare(a.Day, b.Day))
	})
	return keys
}

func mismatchStrings(mismatches []dtos.AlmanaxMismatch) []string {
	result := make([]string, 0, len(mismatches))
	for _, mismatch := range mismatches {
		result = append(result, mismatch.String())
	}

	return result
}

func summarizeMismatches(mismatches []dtos.AlmanaxMismatch) string {
	quoted := mismatchStrings(mismatches[:min(len(mismatches), maxQuotedUnmatched)])
	return fmt.Sprintf("%d almanax day(s) unmatched: %s", len(mismatches), strings.Join(quoted, "; "))
}

func parseReport(value *string) report {
	var result report
	if value != nil {
		_ = json.Unmarshal([]byte(*value), &result)
	}

	return result
}
