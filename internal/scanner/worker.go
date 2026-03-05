package scanner

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/yufugumi/waxe-go/internal/browser"
)

const (
	scanChunkSize        = 50
	chunkDelay           = 2 * time.Second
	DefaultPerURLTimeout = 30 * time.Second
	defaultWorkers       = 1
	maxScanRetries       = 2
	retryDelay           = 2 * time.Second
)

type urlJob struct {
	index int
	url   string
}

type urlResult struct {
	index  int
	result *ScanResult
}

// ScanURLs scans all URLs using a worker pool with chunking and fail-fast behavior.
func ScanURLs(ctx context.Context, urls []string, workers int, excludeRules []string, perURLTimeout time.Duration) ([]*ScanResult, error) {
	return ScanURLsWithProgress(ctx, urls, workers, excludeRules, perURLTimeout, nil)
}

// ScanURLsWithProgress scans URLs and emits progress updates as each URL completes.
func ScanURLsWithProgress(
	ctx context.Context,
	urls []string,
	workers int,
	excludeRules []string,
	perURLTimeout time.Duration,
	reporter ProgressReporter,
) ([]*ScanResult, error) {
	if ctx == nil {
		return nil, fmt.Errorf("ScanURLs: context is nil")
	}
	if perURLTimeout <= 0 {
		return nil, fmt.Errorf("ScanURLs: timeout must be positive: %s", perURLTimeout)
	}

	if workers <= 0 {
		workers = defaultWorkers
	}

	results := make([]*ScanResult, len(urls))
	if len(urls) == 0 {
		return results, nil
	}

	progress := newProgressTracker(len(urls), reporter)
	progress.emitStart()

	for start := 0; start < len(urls); start += scanChunkSize {
		end := start + scanChunkSize
		if end > len(urls) {
			end = len(urls)
		}

		if err := scanChunk(ctx, urls, start, end, workers, excludeRules, results, perURLTimeout, progress); err != nil {
			return nil, err
		}

		if end < len(urls) {
			if err := sleepWithContext(ctx, chunkDelay); err != nil {
				return nil, err
			}
		}
	}

	progress.emitDone()
	return results, nil
}

func scanChunk(
	ctx context.Context,
	urls []string,
	start int,
	end int,
	workers int,
	excludeRules []string,
	results []*ScanResult,
	perURLTimeout time.Duration,
	progress *progressTracker,
) error {
	if start >= end {
		return nil
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	chunkCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobCount := end - start
	workerCount := workers
	if workerCount > jobCount {
		workerCount = jobCount
	}

	jobs := make(chan urlJob)
	resultCh := make(chan urlResult, jobCount)
	errCh := make(chan error, 1)

	var once sync.Once
	sendErr := func(err error) {
		if err == nil {
			return
		}
		once.Do(func() {
			errCh <- err
			cancel()
		})
	}

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				if err := chunkCtx.Err(); err != nil {
					return
				}
				result, err := scanURL(chunkCtx, job.url, excludeRules, perURLTimeout)
				if err != nil {
					sendErr(err)
					return
				}
				resultCh <- urlResult{index: job.index, result: result}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for i := start; i < end; i++ {
			select {
			case <-chunkCtx.Done():
				return
			case jobs <- urlJob{index: i, url: urls[i]}:
			}
		}
	}()

	completed := 0
	for completed < jobCount {
		select {
		case err := <-errCh:
			if err != nil {
				wg.Wait()
				return err
			}
		case <-ctx.Done():
			wg.Wait()
			return ctx.Err()
		case result := <-resultCh:
			results[result.index] = result.result
			completed++
			progress.increment(urls[result.index])
		}
	}

	wg.Wait()
	return nil
}

type progressTracker struct {
	total     int
	processed int
	reporter  ProgressReporter
}

func newProgressTracker(total int, reporter ProgressReporter) *progressTracker {
	if reporter == nil || total <= 0 {
		return &progressTracker{total: total}
	}

	return &progressTracker{total: total, reporter: reporter}
}

func (tracker *progressTracker) emitStart() {
	if tracker.reporter == nil || tracker.total <= 0 {
		return
	}
	tracker.reporter(ProgressUpdate{Processed: 0, Total: tracker.total, Percent: 0, URL: ""})
}

func (tracker *progressTracker) increment(url string) {
	if tracker.reporter == nil || tracker.total <= 0 {
		return
	}
	tracker.processed++
	percent := float64(tracker.processed) / float64(tracker.total) * 100
	tracker.reporter(ProgressUpdate{
		Processed: tracker.processed,
		Total:     tracker.total,
		Percent:   percent,
		URL:       url,
	})
}

func (tracker *progressTracker) emitDone() {
	if tracker.reporter == nil || tracker.total <= 0 {
		return
	}
	tracker.reporter(ProgressUpdate{Processed: tracker.total, Total: tracker.total, Percent: 100, URL: ""})
}

func scanURL(ctx context.Context, url string, excludeRules []string, perURLTimeout time.Duration) (*ScanResult, error) {
	if url == "" {
		return nil, fmt.Errorf("ScanURLs: url is empty")
	}

	browserCtx, cancel := browser.NewBrowser(ctx)
	defer cancel()

	if err := browser.BlockAnalytics(browserCtx); err != nil {
		return nil, fmt.Errorf("ScanURLs: block analytics: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= maxScanRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		attemptCtx, attemptCancel := context.WithTimeout(browserCtx, perURLTimeout)
		if err := browser.Navigate(attemptCtx, url); err != nil {
			attemptCancel()
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if attemptCtx.Err() != nil {
				lastErr = fmt.Errorf("ScanURLs: navigate: %w", attemptCtx.Err())
			} else {
				lastErr = fmt.Errorf("ScanURLs: navigate: %w", err)
			}
			if attempt < maxScanRetries {
				if err := sleepWithContext(ctx, retryDelay); err != nil {
					return nil, err
				}
				continue
			}
			return nil, lastErr
		}

		if err := InjectAxeCore(attemptCtx); err != nil {
			attemptCancel()
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if attemptCtx.Err() != nil {
				lastErr = fmt.Errorf("ScanURLs: inject axe: %w", attemptCtx.Err())
			} else {
				lastErr = fmt.Errorf("ScanURLs: inject axe: %w", err)
			}
			if attempt < maxScanRetries {
				if err := sleepWithContext(ctx, retryDelay); err != nil {
					return nil, err
				}
				continue
			}
			return nil, lastErr
		}

		violations, err := ExecuteAxe(attemptCtx, excludeRules)
		if err != nil {
			attemptCancel()
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if attemptCtx.Err() != nil {
				lastErr = fmt.Errorf("ScanURLs: execute axe: %w", attemptCtx.Err())
			} else {
				lastErr = fmt.Errorf("ScanURLs: execute axe: %w", err)
			}
			if attempt < maxScanRetries {
				if err := sleepWithContext(ctx, retryDelay); err != nil {
					return nil, err
				}
				continue
			}
			return nil, lastErr
		}

		result := &ScanResult{
			URL:        url,
			Violations: violations,
			Timestamp:  time.Now(),
		}
		attemptCancel()
		return result, nil
	}

	if lastErr == nil {
		return nil, fmt.Errorf("ScanURLs: retries exhausted")
	}
	return nil, lastErr
}

func sleepWithContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
