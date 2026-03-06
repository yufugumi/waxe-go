package scanner

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/yufugumi/axel/internal/browser"
)

const (
	scanChunkSize        = 50
	DefaultPerURLTimeout = 30 * time.Second
	defaultWorkers       = 1
	defaultChunkDelay    = 250 * time.Millisecond
	defaultMaxChunkDelay = 2 * time.Second
	defaultMaxRetries    = 0
	defaultRetryDelay    = 2 * time.Second
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
	return ScanURLsWithOptions(ctx, urls, ScanOptions{
		Workers:       workers,
		ExcludeRules:  excludeRules,
		PerURLTimeout: perURLTimeout,
		Reporter:      reporter,
		MaxRetries:    defaultMaxRetries,
		RetryDelay:    defaultRetryDelay,
		ChunkDelay:    defaultChunkDelay,
		MaxChunkDelay: defaultMaxChunkDelay,
		BlockMedia:    true,
	})
}

// ScanURLsWithOptions scans URLs using the provided options.
func ScanURLsWithOptions(ctx context.Context, urls []string, options ScanOptions) ([]*ScanResult, error) {
	if ctx == nil {
		return nil, fmt.Errorf("ScanURLs: context is nil")
	}
	if options.PerURLTimeout <= 0 {
		return nil, fmt.Errorf("ScanURLs: timeout must be positive: %s", options.PerURLTimeout)
	}
	if options.ChunkDelay < 0 {
		return nil, fmt.Errorf("ScanURLs: chunk delay must be non-negative: %s", options.ChunkDelay)
	}
	if options.MaxChunkDelay < 0 {
		return nil, fmt.Errorf("ScanURLs: max chunk delay must be non-negative: %s", options.MaxChunkDelay)
	}
	if options.MaxChunkDelay > 0 && options.ChunkDelay > options.MaxChunkDelay {
		return nil, fmt.Errorf("ScanURLs: chunk delay must be <= max chunk delay")
	}
	if options.Workers <= 0 {
		options.Workers = defaultWorkers
	}
	if options.MaxRetries < 0 {
		return nil, fmt.Errorf("ScanURLs: max retries must be non-negative: %d", options.MaxRetries)
	}
	if options.RetryDelay < 0 {
		return nil, fmt.Errorf("ScanURLs: retry delay must be non-negative: %s", options.RetryDelay)
	}

	results := make([]*ScanResult, len(urls))
	if len(urls) == 0 {
		return results, nil
	}

	progress := newProgressTracker(len(urls), options.Reporter)
	progress.emitStart()

	allocCtx, allocCancel := browser.NewAllocator(ctx)
	defer allocCancel()

	browserCtx, browserCancel, err := browser.NewBrowserContext(allocCtx)
	if err != nil {
		return nil, fmt.Errorf("ScanURLs: %w", err)
	}
	defer browserCancel()

	for start := 0; start < len(urls); start += scanChunkSize {
		end := min(start+scanChunkSize, len(urls))

		if err := scanChunk(ctx, browserCtx, urls, start, end, options, results, progress); err != nil {
			return nil, err
		}

		if end < len(urls) {
			delay := calculateChunkDelay(options.ChunkDelay, options.MaxChunkDelay, start/scanChunkSize)
			if err := sleepWithContext(ctx, delay); err != nil {
				return nil, err
			}
		}
	}

	progress.emitDone()
	return results, nil
}

func scanChunk(
	ctx context.Context,
	allocCtx context.Context,
	urls []string,
	start int,
	end int,
	options ScanOptions,
	results []*ScanResult,
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
	workerCount := min(options.Workers, jobCount)

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
	for range workerCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				if err := chunkCtx.Err(); err != nil {
					return
				}
				result, err := scanURL(chunkCtx, allocCtx, job.url, options)
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

func scanURL(ctx context.Context, allocCtx context.Context, url string, options ScanOptions) (*ScanResult, error) {
	if url == "" {
		return nil, fmt.Errorf("ScanURLs: url is empty")
	}
	if allocCtx == nil {
		return nil, fmt.Errorf("ScanURLs: browser allocator context is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var lastErr error
	maxAttempts := options.MaxRetries + 1
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		result, err := scanURLAttempt(ctx, allocCtx, url, options)
		if err == nil {
			return result, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		lastErr = err
		if attempt < maxAttempts-1 {
			if err := sleepWithContext(ctx, options.RetryDelay); err != nil {
				return nil, err
			}
			continue
		}
		return &ScanResult{URL: url, Timestamp: time.Now(), Error: lastErr.Error()}, nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("retries exhausted")
	}
	return &ScanResult{URL: url, Timestamp: time.Now(), Error: lastErr.Error()}, nil
}

func scanURLAttempt(ctx context.Context, allocCtx context.Context, url string, options ScanOptions) (*ScanResult, error) {
	tabParentCtx, tabParentCancel := context.WithCancel(allocCtx)
	go func() {
		select {
		case <-ctx.Done():
			tabParentCancel()
		case <-tabParentCtx.Done():
		}
	}()

	browserCtx, cancel := browser.NewTab(tabParentCtx)
	defer cancel()
	defer tabParentCancel()

	if err := browser.BlockRequests(browserCtx, options.BlockMedia); err != nil {
		return nil, fmt.Errorf("block requests: %w", err)
	}

	attemptCtx, attemptCancel := context.WithTimeout(browserCtx, options.PerURLTimeout)
	defer attemptCancel()

	if err := browser.Navigate(attemptCtx, url); err != nil {
		if attemptCtx.Err() != nil {
			return nil, fmt.Errorf("navigate: %w", attemptCtx.Err())
		}
		return nil, fmt.Errorf("navigate: %w", err)
	}

	if err := InjectAxeCore(attemptCtx); err != nil {
		if attemptCtx.Err() != nil {
			return nil, fmt.Errorf("inject axe: %w", attemptCtx.Err())
		}
		return nil, fmt.Errorf("inject axe: %w", err)
	}

	violations, err := ExecuteAxe(attemptCtx, options.ExcludeRules)
	if err != nil {
		if attemptCtx.Err() != nil {
			return nil, fmt.Errorf("execute axe: %w", attemptCtx.Err())
		}
		return nil, fmt.Errorf("execute axe: %w", err)
	}

	result := &ScanResult{
		URL:        url,
		Violations: violations,
		Timestamp:  time.Now(),
	}
	return result, nil
}

func calculateChunkDelay(base time.Duration, max time.Duration, chunkIndex int) time.Duration {
	if base <= 0 {
		return 0
	}
	if chunkIndex <= 0 {
		return clampDelay(base, max)
	}

	shift := chunkIndex
	if shift > 30 {
		shift = 30
	}

	delay := base * time.Duration(1<<shift)
	return clampDelay(delay, max)
}

func clampDelay(delay time.Duration, max time.Duration) time.Duration {
	if delay < 0 {
		return 0
	}
	if max > 0 && delay > max {
		return max
	}
	return delay
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
