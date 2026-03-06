package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/yufugumi/waxe-go/internal/reporter"
	"github.com/yufugumi/waxe-go/internal/scanner"
	"github.com/yufugumi/waxe-go/internal/sitemap"
)

var version = "dev"

var nowFn = time.Now

type scanTarget struct {
	TestName     string
	ExcludeRules []string
}

func main() {
	if err := NewRootCommand().Execute(); err != nil {
		if errors.Is(err, context.Canceled) {
			os.Exit(130)
		}
		os.Exit(1)
	}
}

func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:     "axel",
		Short:   "Run accessibility scans",
		Version: version,
	}

	rootCmd.AddCommand(newScanCommand())

	return rootCmd
}

func newScanCommand() *cobra.Command {
	var sitemapURL string
	var baseURL string
	var timeout time.Duration
	var workers int
	var maxRetries int
	var retryDelay time.Duration
	var chunkDelay time.Duration
	var maxChunkDelay time.Duration

	cmd := &cobra.Command{
		Use:   "scan [base-url]",
		Short: "Scan a site using a sitemap or base URL",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
			defer stop()

			var err error
			workers, err = resolveEnvInt(cmd, "workers", "WAXE_WORKERS", workers)
			if err != nil {
				return err
			}
			maxRetries, err = resolveEnvInt(cmd, "retries", "WAXE_RETRIES", maxRetries)
			if err != nil {
				return err
			}
			retryDelay, err = resolveEnvDuration(cmd, "retry-delay", "WAXE_RETRY_DELAY", retryDelay)
			if err != nil {
				return err
			}
			chunkDelay, err = resolveEnvDuration(cmd, "chunk-delay", "WAXE_CHUNK_DELAY", chunkDelay)
			if err != nil {
				return err
			}
			maxChunkDelay, err = resolveEnvDuration(cmd, "chunk-delay-max", "WAXE_CHUNK_DELAY_MAX", maxChunkDelay)
			if err != nil {
				return err
			}

			positionalBaseURL := ""
			if len(args) > 0 {
				positionalBaseURL = strings.TrimSpace(args[0])
			}
			if positionalBaseURL != "" {
				if strings.TrimSpace(sitemapURL) != "" {
					return fmt.Errorf("positional base URL cannot be used with --sitemap-url")
				}
				if cmd.Flags().Changed("base-url") {
					return fmt.Errorf("--base-url cannot be used with positional base URL")
				}
				return runScanFromBaseURL(ctx, positionalBaseURL, buildScanOptions(timeout, workers, maxRetries, retryDelay, chunkDelay, maxChunkDelay))
			}

			effectiveSitemapURL := resolveSitemapURL(sitemapURL)
			if effectiveSitemapURL == "" {
				return fmt.Errorf("base URL or sitemap URL is required")
			}

			return runScanFromSitemap(ctx, effectiveSitemapURL, baseURL, buildScanOptions(timeout, workers, maxRetries, retryDelay, chunkDelay, maxChunkDelay))
		},
	}

	cmd.Flags().StringVar(&sitemapURL, "sitemap-url", "", "Sitemap URL to scan")
	cmd.Flags().StringVar(&baseURL, "base-url", "", "Base URL for resolving relative sitemap entries")
	cmd.Flags().DurationVar(&timeout, "timeout", scanner.DefaultPerURLTimeout, "Per-URL timeout (e.g., 30s, 1m)")
	cmd.Flags().IntVar(&workers, "workers", 10, "Number of concurrent workers")
	cmd.Flags().IntVar(&maxRetries, "retries", 0, "Retries per URL (0 disables retries)")
	cmd.Flags().DurationVar(&retryDelay, "retry-delay", 2*time.Second, "Delay between retries")
	cmd.Flags().DurationVar(&chunkDelay, "chunk-delay", 250*time.Millisecond, "Base delay between chunks")
	cmd.Flags().DurationVar(&maxChunkDelay, "chunk-delay-max", 2*time.Second, "Maximum delay between chunks")

	return cmd
}

func runScanFromSitemap(ctx context.Context, sitemapURL string, baseURL string, options scanner.ScanOptions) error {
	resolvedSitemapURL, err := normalizeSitemapURL(sitemapURL)
	if err != nil {
		return err
	}

	target := scanTarget{
		TestName: deriveTestNameFromSitemap(resolvedSitemapURL),
	}

	resolvedBaseURL, err := resolveBaseURLOverride(baseURL, resolvedSitemapURL)
	if err != nil {
		return err
	}

	return runScanWithConfig(ctx, target, resolvedSitemapURL, resolvedBaseURL, options)
}

func runScanWithConfig(ctx context.Context, target scanTarget, sitemapURL string, baseURL string, options scanner.ScanOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	sitemapData, err := sitemap.Fetch(ctx, sitemapURL)
	if err != nil {
		return err
	}

	urls, err := sitemap.Parse(sitemapData, log.Printf)
	if err != nil {
		return err
	}

	urls, err = resolveSitemapURLs(urls, baseURL)
	if err != nil {
		if fallbackURLs, fallbackErr := fallbackURLsFromEnv(); fallbackErr != nil {
			return fallbackErr
		} else if len(fallbackURLs) > 0 {
			log.Printf("warning: sitemap resolve failed (%v); using fallback URLs", err)
			urls = fallbackURLs
		} else {
			return err
		}
	}

	finalURLs, err := finalizeURLs(urls, "sitemap")
	if err != nil {
		return err
	}

	return runScanWithURLs(ctx, target, finalURLs, options)
}

func runScanWithURLs(ctx context.Context, target scanTarget, urls []string, options scanner.ScanOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(urls) == 0 {
		return fmt.Errorf("no URLs available to scan")
	}

	progressReporter, stopProgress := newProgressPrinter(len(urls))
	defer stopProgress()

	options.ExcludeRules = target.ExcludeRules
	options.Reporter = progressReporter
	results, err := scanner.ScanURLsWithOptions(ctx, urls, options)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	var failedCount int
	for _, r := range results {
		if r != nil && r.Error != "" {
			failedCount++
			log.Printf("warning: %s: %s", r.URL, r.Error)
		}
	}
	if failedCount > 0 {
		log.Printf("warning: %d/%d URLs failed and were skipped", failedCount, len(results))
	}

	date := nowFn().Format("2006-01-02")
	report, err := reporter.Generate(results, target.TestName, date)
	if err != nil {
		return err
	}

	outputDir := getOutputDir()
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}

	reportName := safeFilename(target.TestName)
	reportPath := filepath.Join(outputDir, fmt.Sprintf("%s-%s.html", reportName, date))
	if err := os.WriteFile(reportPath, report, 0o644); err != nil {
		return err
	}

	return nil
}

func finalizeURLs(urls []string, source string) ([]string, error) {
	maxURLs, err := maxURLsFromEnv()
	if err != nil {
		return nil, err
	}
	urls = applyMaxURLs(urls, maxURLs)

	if len(urls) == 0 {
		fallbackURLs, err := fallbackURLsFromEnv()
		if err != nil {
			return nil, err
		}
		if len(fallbackURLs) > 0 {
			urls = fallbackURLs
		}
	}
	urls = applyMaxURLs(urls, maxURLs)

	if len(urls) == 0 {
		if source == "" {
			source = "source"
		}
		return nil, fmt.Errorf("no URLs found from %s or fallback list", source)
	}

	return urls, nil
}

func buildScanOptions(timeout time.Duration, workers int, maxRetries int, retryDelay time.Duration, chunkDelay time.Duration, maxChunkDelay time.Duration) scanner.ScanOptions {
	return scanner.ScanOptions{
		Workers:       workers,
		PerURLTimeout: timeout,
		MaxRetries:    maxRetries,
		RetryDelay:    retryDelay,
		ChunkDelay:    chunkDelay,
		MaxChunkDelay: maxChunkDelay,
		BlockMedia:    true,
	}
}

func resolveEnvInt(cmd *cobra.Command, flagName string, envKey string, current int) (int, error) {
	if cmd.Flags().Changed(flagName) {
		return current, nil
	}
	raw := strings.TrimSpace(os.Getenv(envKey))
	if raw == "" {
		return current, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s value %q: %w", envKey, raw, err)
	}
	return value, nil
}

func resolveEnvDuration(cmd *cobra.Command, flagName string, envKey string, current time.Duration) (time.Duration, error) {
	if cmd.Flags().Changed(flagName) {
		return current, nil
	}
	raw := strings.TrimSpace(os.Getenv(envKey))
	if raw == "" {
		return current, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s value %q: %w", envKey, raw, err)
	}
	return value, nil
}

func resolveSitemapURL(cliSitemapURL string) string {
	trimmed := sitemap.SanitizeLoc(cliSitemapURL)
	if trimmed != "" {
		return trimmed
	}
	return sitemap.SanitizeLoc(os.Getenv("WAXE_SITEMAP_URL"))
}

func normalizeSitemapURL(raw string) (string, error) {
	trimmed := sitemap.SanitizeLoc(raw)
	if trimmed == "" {
		return "", fmt.Errorf("sitemap URL is required")
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid sitemap URL %q: %w", trimmed, err)
	}
	if parsed.IsAbs() && parsed.Host != "" {
		return parsed.String(), nil
	}

	withScheme, err := url.Parse("https://" + trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid sitemap URL %q: %w", trimmed, err)
	}
	if !withScheme.IsAbs() || withScheme.Host == "" {
		return "", fmt.Errorf("sitemap URL must be absolute: %s", trimmed)
	}

	return withScheme.String(), nil
}

func resolveBaseURLOverride(cliBaseURL string, sitemapURL string) (string, error) {
	if override := strings.TrimSpace(cliBaseURL); override != "" {
		return override, nil
	}
	if override := strings.TrimSpace(os.Getenv("WAXE_BASE_URL")); override != "" {
		return override, nil
	}

	parsed, err := url.Parse(sitemapURL)
	if err != nil {
		return "", fmt.Errorf("invalid sitemap URL %q: %w", sitemapURL, err)
	}
	if !parsed.IsAbs() || parsed.Host == "" {
		return "", fmt.Errorf("sitemap URL must be absolute: %s", sitemapURL)
	}

	return fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host), nil
}

func deriveTestNameFromSitemap(sitemapURL string) string {
	parsed, err := url.Parse(sitemapURL)
	if err != nil {
		return "Sitemap Scan"
	}

	host := parsed.Hostname()
	if host == "" {
		return "Sitemap Scan"
	}

	return fmt.Sprintf("Sitemap %s", host)
}

func resolveSitemapURLs(urls []string, baseURL string) ([]string, error) {
	if len(urls) == 0 || baseURL == "" {
		return urls, nil
	}

	base, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}
	if !base.IsAbs() {
		return nil, fmt.Errorf("base URL must be absolute: %s", baseURL)
	}

	resolved := make([]string, 0, len(urls))
	for _, loc := range urls {
		if loc == "" {
			log.Printf("warning: skipping sitemap URL with empty <loc> after sanitization")
			continue
		}

		parsed, err := url.Parse(loc)
		if err != nil {
			log.Printf("warning: skipping invalid sitemap URL %q: %v", loc, err)
			continue
		}

		if parsed.IsAbs() {
			resolved = append(resolved, loc)
			continue
		}

		resolved = append(resolved, base.ResolveReference(parsed).String())
	}

	if len(resolved) == 0 {
		return nil, fmt.Errorf("no valid sitemap URLs found after resolving")
	}

	return resolved, nil
}

func safeFilename(name string) string {
	sanitized := strings.TrimSpace(name)
	sanitized = strings.ReplaceAll(sanitized, string(os.PathSeparator), "-")
	sanitized = strings.ReplaceAll(sanitized, "/", "-")
	sanitized = strings.ReplaceAll(sanitized, "\\", "-")
	sanitized = strings.ReplaceAll(sanitized, "..", "")
	if sanitized == "" {
		return "report"
	}
	return sanitized
}

func getOutputDir() string {
	if outputDir := os.Getenv("WAXE_OUTPUT_DIR"); outputDir != "" {
		return outputDir
	}

	return "."
}

func fallbackURLsFromEnv() ([]string, error) {
	if path := strings.TrimSpace(os.Getenv("WAXE_FALLBACK_URLS_FILE")); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read fallback URLs file: %w", err)
		}
		return parseURLList(string(data)), nil
	}

	raw := strings.TrimSpace(os.Getenv("WAXE_FALLBACK_URLS"))
	if raw == "" {
		return nil, nil
	}

	return parseURLList(raw), nil
}

func parseURLList(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r' || r == '\t'
	})

	urls := make([]string, 0, len(fields))
	for _, field := range fields {
		trimmed := strings.TrimSpace(field)
		if trimmed == "" {
			continue
		}
		urls = append(urls, trimmed)
	}

	return urls
}

func maxURLsFromEnv() (int, error) {
	raw := strings.TrimSpace(os.Getenv("WAXE_MAX_URLS"))
	if raw == "" {
		return 0, nil
	}

	max, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid WAXE_MAX_URLS value %q: %w", raw, err)
	}
	if max < 0 {
		return 0, fmt.Errorf("WAXE_MAX_URLS must be positive: %d", max)
	}

	return max, nil
}

func applyMaxURLs(urls []string, max int) []string {
	if max <= 0 || len(urls) <= max {
		return urls
	}

	return urls[:max]
}

type progressPrinter struct {
	lastLen int
}

func newProgressPrinter(total int) (scanner.ProgressReporter, func()) {
	if total <= 0 {
		return nil, func() {}
	}

	printer := &progressPrinter{}
	reporter := func(update scanner.ProgressUpdate) {
		line := formatProgressLine(update)
		if line == "" {
			return
		}
		if printer.lastLen > len(line) {
			line = line + strings.Repeat(" ", printer.lastLen-len(line))
		}
		fmt.Fprintf(os.Stdout, "\r%s", line)
		printer.lastLen = len(line)
	}

	stop := func() {
		if printer.lastLen == 0 {
			return
		}
		fmt.Fprintln(os.Stdout)
		printer.lastLen = 0
	}

	return reporter, stop
}

func formatProgressLine(update scanner.ProgressUpdate) string {
	if update.Total <= 0 {
		return ""
	}

	url := update.URL
	if url == "" {
		url = "-"
	}

	return fmt.Sprintf("Progress: %d/%d (%.1f%%) %s", update.Processed, update.Total, update.Percent, url)
}
