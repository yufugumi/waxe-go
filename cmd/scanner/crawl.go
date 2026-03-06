package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/yufugumi/waxe-go/internal/scanner"
	"github.com/yufugumi/waxe-go/internal/sitemap"
	"github.com/yufugumi/waxe-go/internal/useragent"
)

const (
	defaultCrawlDepth = 5
	defaultCrawlDelay = 300 * time.Millisecond
)

type crawlOptions struct {
	MaxDepth int
	Delay    time.Duration
	MaxURLs  int
}

type crawlItem struct {
	url   *url.URL
	depth int
}

type robotsRules struct {
	allow    []string
	disallow []string
}

type crawlProgress struct {
	lastLen int
}

func (p *crawlProgress) update(format string, args ...any) {
	line := fmt.Sprintf(format, args...)
	pad := ""
	if p.lastLen > len(line) {
		pad = strings.Repeat(" ", p.lastLen-len(line))
	}
	fmt.Fprintf(os.Stdout, "\r%s%s", line, pad)
	p.lastLen = len(line)
}

func (p *crawlProgress) done() {
	if p.lastLen > 0 {
		fmt.Fprintln(os.Stdout)
		p.lastLen = 0
	}
}

func runScanFromBaseURL(ctx context.Context, rawBaseURL string, options scanner.ScanOptions) error {
	base, err := normalizeBaseURL(rawBaseURL)
	if err != nil {
		return err
	}

	progress := &crawlProgress{}
	sitemapURLs, err := discoverSitemapURLs(ctx, base, progress)
	if err != nil {
		progress.done()
		return err
	}
	if len(sitemapURLs) > 0 {
		progress.done()
		finalURLs, err := finalizeURLs(sitemapURLs, "sitemap discovery")
		if err != nil {
			return err
		}
		return runScanWithURLs(ctx, scanTarget{TestName: fmt.Sprintf("Sitemap %s", base.Hostname())}, finalURLs, options)
	}

	maxURLs, err := maxURLsFromEnv()
	if err != nil {
		return err
	}

	crawled, err := crawlSite(ctx, base, crawlOptions{
		MaxDepth: defaultCrawlDepth,
		Delay:    defaultCrawlDelay,
		MaxURLs:  maxURLs,
	}, progress)
	progress.done()
	if err != nil {
		return err
	}

	finalURLs, err := finalizeURLs(crawled, "crawl")
	if err != nil {
		return err
	}

	return runScanWithURLs(ctx, scanTarget{TestName: fmt.Sprintf("Crawl %s", base.Hostname())}, finalURLs, options)
}

func normalizeBaseURL(raw string) (*url.URL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("base URL is required")
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL %q: %w", trimmed, err)
	}
	if parsed.IsAbs() && parsed.Host != "" {
		parsed.Fragment = ""
		return parsed, nil
	}

	withScheme, err := url.Parse("https://" + trimmed)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL %q: %w", trimmed, err)
	}
	if !withScheme.IsAbs() || withScheme.Host == "" {
		return nil, fmt.Errorf("base URL must be absolute: %s", trimmed)
	}

	withScheme.Fragment = ""
	return withScheme, nil
}

func discoverSitemapURLs(ctx context.Context, baseURL *url.URL, progress *crawlProgress) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if baseURL == nil {
		return nil, fmt.Errorf("base URL is required for sitemap discovery")
	}

	baseRoot := fmt.Sprintf("%s://%s", baseURL.Scheme, baseURL.Host)
	robotsURL := baseRoot + "/robots.txt"
	candidates := make([]string, 0, 4)

	robotsSitemaps, err := discoverRobotsSitemaps(ctx, robotsURL)
	if err != nil {
		if !strings.Contains(err.Error(), "HTTP 404") {
			log.Printf("warning: sitemap discovery robots.txt failed: %v", err)
		}
	} else if len(robotsSitemaps) == 0 {
		log.Printf("warning: no sitemap entries found in robots.txt")
	}

	seen := make(map[string]struct{})
	for _, entry := range robotsSitemaps {
		if entry == "" {
			continue
		}
		if _, ok := seen[entry]; ok {
			continue
		}
		seen[entry] = struct{}{}
		candidates = append(candidates, entry)
	}

	defaults := []string{
		baseRoot + "/sitemap.xml",
		baseRoot + "/sitemap_index.xml",
		baseRoot + "/sitemap-index.xml",
	}
	for _, entry := range defaults {
		if _, ok := seen[entry]; ok {
			continue
		}
		seen[entry] = struct{}{}
		candidates = append(candidates, entry)
	}

	merged := make([]string, 0)
	seenURLs := make(map[string]struct{})
	for i, candidate := range candidates {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if progress != nil {
			progress.update("Discovering: checking sitemap %d/%d...", i+1, len(candidates))
		}
		data, err := sitemap.Fetch(ctx, candidate)
		if err != nil {
			if !sitemap.IsNotFound(err) {
				log.Printf("warning: sitemap fetch failed for %s: %v", candidate, err)
			}
			continue
		}
		urls, err := sitemap.Parse(data, log.Printf)
		if err != nil {
			log.Printf("warning: sitemap parse failed for %s: %v", candidate, err)
			continue
		}
		if len(urls) == 0 {
			log.Printf("warning: sitemap %s contained no URLs", candidate)
			continue
		}

		resolved, err := resolveSitemapURLs(urls, baseRoot)
		if err != nil {
			log.Printf("warning: sitemap %s URL resolve failed: %v", candidate, err)
			continue
		}
		if len(resolved) == 0 {
			log.Printf("warning: sitemap %s yielded no usable URLs", candidate)
			continue
		}
		for _, entry := range resolved {
			if entry == "" {
				continue
			}
			if _, ok := seenURLs[entry]; ok {
				continue
			}
			seenURLs[entry] = struct{}{}
			merged = append(merged, entry)
		}
	}

	return merged, nil
}

func discoverRobotsSitemaps(ctx context.Context, robotsURL string) ([]string, error) {
	if robotsURL == "" {
		return nil, fmt.Errorf("robots URL is required")
	}
	data, err := fetchURL(ctx, robotsURL)
	if err != nil {
		return nil, err
	}
	parsedRobotsURL, err := url.Parse(robotsURL)
	if err != nil {
		return nil, fmt.Errorf("invalid robots URL %q: %w", robotsURL, err)
	}
	return parseRobotsSitemaps(string(data), parsedRobotsURL), nil
}

func parseRobotsSitemaps(body string, robotsURL *url.URL) []string {
	if body == "" || robotsURL == nil {
		return nil
	}

	entries := []string{}
	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := stripRobotsComment(scanner.Text())
		if line == "" {
			continue
		}
		key, value, ok := splitRobotsDirective(line)
		if !ok || key != "sitemap" {
			continue
		}
		if value == "" {
			continue
		}
		loc := sitemap.SanitizeLoc(value)
		if loc == "" {
			continue
		}
		parsed, err := url.Parse(loc)
		if err != nil {
			continue
		}
		if !parsed.IsAbs() {
			parsed = robotsURL.ResolveReference(parsed)
		}
		entries = append(entries, parsed.String())
	}

	return entries
}

func crawlSite(ctx context.Context, baseURL *url.URL, options crawlOptions, progress *crawlProgress) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if baseURL == nil {
		return nil, fmt.Errorf("base URL is required for crawl")
	}
	if options.MaxDepth <= 0 {
		options.MaxDepth = defaultCrawlDepth
	}
	if options.Delay < 0 {
		return nil, fmt.Errorf("crawl delay must be non-negative")
	}

	baseHost := baseURL.Hostname()
	if baseHost == "" {
		return nil, fmt.Errorf("base URL must include a host")
	}

	robots, err := fetchRobotsRules(ctx, baseURL)
	if err != nil {
		if !strings.Contains(err.Error(), "HTTP 404") {
			log.Printf("warning: robots fetch failed for %s: %v", baseURL.String(), err)
		}
	}
	if robots.blocksAll() {
		log.Printf("warning: robots.txt blocks all paths for User-agent: *; ignoring for accessibility scan")
		robots = robotsRules{}
	}
	client := &http.Client{Timeout: 15 * time.Second}

	queue := []crawlItem{{url: stripFragment(baseURL), depth: 0}}
	visited := map[string]struct{}{normalizeURL(stripFragment(baseURL)): {}}
	results := make([]string, 0)
	firstRequest := true

	for len(queue) > 0 {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if options.MaxURLs > 0 && len(results) >= options.MaxURLs {
			break
		}

		current := queue[0]
		queue = queue[1:]

		if !isSameHost(current.url, baseHost) {
			continue
		}
		if !isHTTPURL(current.url) {
			continue
		}
		if !robots.allows(pathForRobots(current.url)) {
			continue
		}

		if !firstRequest && options.Delay > 0 {
			if err := sleepWithContext(ctx, options.Delay); err != nil {
				return nil, err
			}
		}
		firstRequest = false

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, current.url.String(), nil)
		if err != nil {
			log.Printf("warning: crawl request failed for %s: %v", current.url.String(), err)
			continue
		}
		req.Header.Set("User-Agent", useragent.CommonUserAgent)
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("warning: crawl fetch failed for %s: %v", current.url.String(), err)
			continue
		}
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
			_ = resp.Body.Close()
			continue
		}
		contentType := strings.ToLower(resp.Header.Get("Content-Type"))
		if contentType != "" && !strings.Contains(contentType, "text/html") {
			_ = resp.Body.Close()
			continue
		}

		if contentType == "" {
			bodyBytes, err := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if err != nil {
				log.Printf("warning: crawl read failed for %s: %v", current.url.String(), err)
				continue
			}
			if !looksLikeHTML(bodyBytes) {
				continue
			}
			results = append(results, current.url.String())
			if progress != nil {
				progress.update("Crawling: %d URLs found (depth %d/%d)", len(results), current.depth, options.MaxDepth)
			}
			if current.depth >= options.MaxDepth {
				continue
			}
			links := extractLinksFromHTML(bodyBytes, current.url)
			for _, link := range links {
				if options.MaxURLs > 0 && len(results) >= options.MaxURLs {
					break
				}
				normalized := normalizeURL(link)
				if normalized == "" {
					continue
				}
				if _, ok := visited[normalized]; ok {
					continue
				}
				visited[normalized] = struct{}{}
				queue = append(queue, crawlItem{url: link, depth: current.depth + 1})
			}
			continue
		}

		results = append(results, current.url.String())
		if progress != nil {
			progress.update("Crawling: %d URLs found (depth %d/%d)", len(results), current.depth, options.MaxDepth)
		}
		if current.depth >= options.MaxDepth {
			_ = resp.Body.Close()
			continue
		}

		bodyBytes, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			log.Printf("warning: crawl read failed for %s: %v", current.url.String(), err)
			continue
		}
		links := extractLinksFromHTML(bodyBytes, current.url)
		for _, link := range links {
			if options.MaxURLs > 0 && len(results) >= options.MaxURLs {
				break
			}
			normalized := normalizeURL(link)
			if normalized == "" {
				continue
			}
			if _, ok := visited[normalized]; ok {
				continue
			}
			visited[normalized] = struct{}{}
			queue = append(queue, crawlItem{url: link, depth: current.depth + 1})
		}
	}

	return results, nil
}

func fetchRobotsRules(ctx context.Context, baseURL *url.URL) (robotsRules, error) {
	if baseURL == nil {
		return robotsRules{}, fmt.Errorf("base URL is required")
	}
	robotsURL := fmt.Sprintf("%s://%s/robots.txt", baseURL.Scheme, baseURL.Host)
	data, err := fetchURL(ctx, robotsURL)
	if err != nil {
		return robotsRules{}, err
	}
	return parseRobotsRules(string(data)), nil
}

func parseRobotsRules(body string) robotsRules {
	var rules robotsRules
	var groupHasStar bool
	var sawDirective bool

	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := stripRobotsComment(scanner.Text())
		if line == "" {
			continue
		}
		key, value, ok := splitRobotsDirective(line)
		if !ok {
			continue
		}
		switch key {
		case "user-agent":
			agent := strings.ToLower(strings.TrimSpace(value))
			if sawDirective {
				groupHasStar = false
				sawDirective = false
			}
			if agent == "*" {
				groupHasStar = true
			}
		case "allow":
			sawDirective = true
			if groupHasStar {
				rules.allow = append(rules.allow, normalizeRobotsRule(value))
			}
		case "disallow":
			sawDirective = true
			if groupHasStar {
				rules.disallow = append(rules.disallow, normalizeRobotsRule(value))
			}
		}
	}

	return rules
}

func (rules robotsRules) allows(path string) bool {
	if path == "" {
		path = "/"
	}
	allowLen := longestMatch(path, rules.allow)
	disallowLen := longestMatch(path, rules.disallow)
	if disallowLen == 0 {
		return true
	}
	return allowLen >= disallowLen
}

func (rules robotsRules) blocksAll() bool {
	for _, rule := range rules.disallow {
		if rule == "/" {
			for _, allow := range rules.allow {
				if allow != "" {
					return false
				}
			}
			return true
		}
	}
	return false
}

func longestMatch(path string, rules []string) int {
	longest := 0
	for _, rule := range rules {
		if rule == "" {
			continue
		}
		if strings.HasPrefix(path, rule) {
			if len(rule) > longest {
				longest = len(rule)
			}
		}
	}
	return longest
}

func normalizeRobotsRule(rule string) string {
	trimmed := strings.TrimSpace(rule)
	if trimmed == "" {
		return ""
	}
	if idx := strings.Index(trimmed, "#"); idx >= 0 {
		trimmed = strings.TrimSpace(trimmed[:idx])
	}
	if strings.HasSuffix(trimmed, "$") {
		trimmed = strings.TrimSuffix(trimmed, "$")
	}
	if idx := strings.Index(trimmed, "*"); idx >= 0 {
		trimmed = trimmed[:idx]
	}
	return strings.TrimSpace(trimmed)
}

func stripRobotsComment(line string) string {
	if idx := strings.Index(line, "#"); idx >= 0 {
		line = line[:idx]
	}
	return strings.TrimSpace(line)
}

func splitRobotsDirective(line string) (string, string, bool) {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	key := strings.ToLower(strings.TrimSpace(parts[0]))
	value := strings.TrimSpace(parts[1])
	if key == "" {
		return "", "", false
	}
	return key, value, true
}

func fetchURL(ctx context.Context, rawURL string) ([]byte, error) {
	if rawURL == "" {
		return nil, fmt.Errorf("URL is required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", useragent.CommonUserAgent)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func extractLinksFromHTML(body []byte, baseURL *url.URL) []*url.URL {
	if len(body) == 0 || baseURL == nil {
		return nil
	}

	links := make([]*url.URL, 0)
	lowered := bytes.ToLower(body)
	marker := []byte("href")
	index := 0
	for index < len(lowered) {
		pos := bytes.Index(lowered[index:], marker)
		if pos == -1 {
			break
		}
		pos = pos + index
		end := pos + len(marker)
		if end >= len(body) {
			break
		}
		for end < len(body) && (body[end] == ' ' || body[end] == '\t' || body[end] == '\n' || body[end] == '\r') {
			end++
		}
		if end >= len(body) || body[end] != '=' {
			index = end
			continue
		}
		end++
		for end < len(body) && (body[end] == ' ' || body[end] == '\t' || body[end] == '\n' || body[end] == '\r') {
			end++
		}
		if end >= len(body) {
			break
		}
		quote := body[end]
		if quote != '"' && quote != '\'' {
			index = end
			continue
		}
		end++
		start := end
		for end < len(body) && body[end] != quote {
			end++
		}
		if end >= len(body) {
			break
		}
		href := strings.TrimSpace(string(body[start:end]))
		index = end + 1
		if href == "" {
			continue
		}
		parsed, err := url.Parse(href)
		if err != nil {
			continue
		}
		if !parsed.IsAbs() {
			parsed = baseURL.ResolveReference(parsed)
		}
		parsed.Fragment = ""
		links = append(links, parsed)
	}

	return links
}

func looksLikeHTML(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return false
	}
	lowered := bytes.ToLower(trimmed)
	return bytes.HasPrefix(lowered, []byte("<!doctype html")) || bytes.HasPrefix(lowered, []byte("<html"))
}

func normalizeURL(parsed *url.URL) string {
	if parsed == nil {
		return ""
	}
	parsed.Fragment = ""
	return parsed.String()
}

func stripFragment(parsed *url.URL) *url.URL {
	if parsed == nil {
		return nil
	}
	clone := *parsed
	clone.Fragment = ""
	return &clone
}

func isSameHost(candidate *url.URL, host string) bool {
	if candidate == nil {
		return false
	}
	return candidate.Hostname() == host
}

func isHTTPURL(candidate *url.URL) bool {
	if candidate == nil {
		return false
	}
	scheme := strings.ToLower(candidate.Scheme)
	return scheme == "http" || scheme == "https"
}

func pathForRobots(candidate *url.URL) string {
	if candidate == nil {
		return "/"
	}
	path := candidate.EscapedPath()
	if path == "" {
		path = "/"
	}
	if candidate.RawQuery == "" {
		return path
	}
	return path + "?" + candidate.RawQuery
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
