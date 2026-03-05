package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/yufugumi/waxe-go/internal/config"
	"github.com/yufugumi/waxe-go/internal/reporter"
	"github.com/yufugumi/waxe-go/internal/scanner"
	"github.com/yufugumi/waxe-go/internal/sitemap"
)

var nowFn = time.Now

func main() {
	if err := NewRootCommand().Execute(); err != nil {
		os.Exit(1)
	}
}

// NewRootCommand constructs the root CLI command for the scanner.
func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "axed",
		Short: "Run WAXE accessibility scans",
	}

	rootCmd.AddCommand(newScanCommand())

	return rootCmd
}

func newScanCommand() *cobra.Command {
	var site string
	var sitemapURL string
	var baseURL string
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Scan a configured site",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateSitemapOverrides(site, sitemapURL); err != nil {
				return err
			}
			effectiveSitemapURL := resolveSitemapURL(site, sitemapURL)
			if site == "" && effectiveSitemapURL == "" {
				return fmt.Errorf("site or sitemap URL is required")
			}
			if site != "" && effectiveSitemapURL != "" {
				return fmt.Errorf("site and sitemap URL cannot be used together")
			}
			if effectiveSitemapURL == "" {
				return runScanFromSite(site, timeout)
			}

			return runScanFromSitemap(effectiveSitemapURL, baseURL, timeout)
		},
	}

	cmd.Flags().StringVar(&site, "site", "", "Site name to scan")
	cmd.Flags().StringVar(&sitemapURL, "sitemap-url", "", "Sitemap URL to scan")
	cmd.Flags().StringVar(&baseURL, "base-url", "", "Base URL for resolving relative sitemap entries")
	cmd.Flags().DurationVar(&timeout, "timeout", scanner.DefaultPerURLTimeout, "Per-URL timeout (e.g., 30s, 1m)")

	return cmd
}

func runScanFromSite(site string, perURLTimeout time.Duration) error {
	siteConfig, err := loadSiteConfig(site)
	if err != nil {
		return err
	}

	sitemapURL, baseURL := resolveSitemapOverrides(siteConfig, site)

	return runScanWithConfig(siteConfig, sitemapURL, baseURL, perURLTimeout)
}

func runScanFromSitemap(sitemapURL string, baseURL string, perURLTimeout time.Duration) error {
	resolvedSitemapURL, err := normalizeSitemapURL(sitemapURL)
	if err != nil {
		return err
	}

	siteConfig := config.SiteConfig{
		TestName: deriveTestNameFromSitemap(resolvedSitemapURL),
		URL:      resolvedSitemapURL,
	}

	resolvedBaseURL, err := resolveBaseURLOverride(baseURL, resolvedSitemapURL)
	if err != nil {
		return err
	}

	return runScanWithConfig(siteConfig, resolvedSitemapURL, resolvedBaseURL, perURLTimeout)
}

func runScanWithConfig(siteConfig config.SiteConfig, sitemapURL string, baseURL string, perURLTimeout time.Duration) error {
	ctx := context.Background()
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

	maxURLs, err := maxURLsFromEnv()
	if err != nil {
		return err
	}
	urls = applyMaxURLs(urls, maxURLs)

	if len(urls) == 0 {
		fallbackURLs, err := fallbackURLsFromEnv()
		if err != nil {
			return err
		}
		if len(fallbackURLs) > 0 {
			urls = fallbackURLs
		}
	}
	urls = applyMaxURLs(urls, maxURLs)

	if len(urls) == 0 {
		return fmt.Errorf("no URLs found from sitemap or fallback list")
	}

	progressReporter, stopProgress := newProgressPrinter(len(urls))
	defer stopProgress()

	results, err := scanner.ScanURLsWithProgress(ctx, urls, 10, siteConfig.ExcludeRules, perURLTimeout, progressReporter)
	if err != nil {
		return err
	}

	date := nowFn().Format("2006-01-02")
	report, err := reporter.Generate(results, siteConfig.TestName, date)
	if err != nil {
		return err
	}

	outputDir := getOutputDir()
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}

	reportName := safeFilename(siteConfig.TestName)
	reportPath := filepath.Join(outputDir, fmt.Sprintf("%s-%s.html", reportName, date))
	if err := os.WriteFile(reportPath, report, 0o644); err != nil {
		return err
	}

	return nil
}

func loadSiteConfig(site string) (config.SiteConfig, error) {
	siteConfig, ok := config.Sites[site]
	if !ok {
		return config.SiteConfig{}, fmt.Errorf("unknown site: %s", site)
	}

	return siteConfig, nil
}

func resolveSitemapOverrides(siteConfig config.SiteConfig, site string) (string, string) {
	sitemapURL := siteConfig.URL
	baseURL := ""

	if override := strings.TrimSpace(os.Getenv("WAXE_SITEMAP_URL")); override != "" {
		if allowSitemapOverrideFromEnv() {
			sitemapURL = override
		}
	}
	if override := strings.TrimSpace(os.Getenv("WAXE_BASE_URL")); override != "" {
		baseURL = override
	}

	if site != "test" {
		return sitemapURL, baseURL
	}

	if sitemapURL == siteConfig.URL {
		if override := strings.TrimSpace(os.Getenv("WAXE_TEST_SITEMAP_URL")); override != "" {
			sitemapURL = override
		}
	}

	if baseURL == "" {
		if override := strings.TrimSpace(os.Getenv("WAXE_TEST_SITE_URL")); override != "" {
			baseURL = override
		}
	}

	return sitemapURL, baseURL
}

func resolveSitemapURL(site string, cliSitemapURL string) string {
	trimmed := sitemap.SanitizeLoc(cliSitemapURL)
	if trimmed != "" {
		return trimmed
	}
	if site != "" {
		return ""
	}
	return sitemap.SanitizeLoc(os.Getenv("WAXE_SITEMAP_URL"))
}

func validateSitemapOverrides(site string, cliSitemapURL string) error {
	if strings.TrimSpace(site) == "" {
		return nil
	}
	if strings.TrimSpace(cliSitemapURL) != "" {
		return nil
	}
	if override := strings.TrimSpace(os.Getenv("WAXE_SITEMAP_URL")); override != "" {
		if allowSitemapOverrideFromEnv() {
			return nil
		}
		return fmt.Errorf("WAXE_SITEMAP_URL cannot be used with --site; use --sitemap-url instead")
	}
	return nil
}

func allowSitemapOverrideFromEnv() bool {
	value := strings.TrimSpace(os.Getenv("WAXE_ALLOW_SITEMAP_OVERRIDE"))
	if value == "" {
		return false
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false
	}
	return parsed
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
