package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestScanCommand(t *testing.T) {
	sitemapXML := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>/page</loc>
  </url>
</urlset>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sitemap.xml":
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(sitemapXML))
		case "/page":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte("<html><head><title>Test</title></head><body>ok</body></html>"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	outputDir := t.TempDir()
	fixedTime := time.Date(2026, time.March, 4, 10, 30, 0, 0, time.UTC)
	date := fixedTime.Format("2006-01-02")
	previousNow := nowFn
	nowFn = func() time.Time { return fixedTime }
	defer func() { nowFn = previousNow }()

	restoreOutputDir := setEnvForTest(t, "WAXE_OUTPUT_DIR", outputDir)
	defer func() {
		restoreOutputDir()
	}()

	previousArgs := os.Args
	os.Args = []string{"axel", "scan", server.URL}
	defer func() { os.Args = previousArgs }()

	if err := NewRootCommand().Execute(); err != nil {
		t.Fatalf("scan command failed: %v", err)
	}

	parsedURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	reportPath := filepath.Join(outputDir, "Sitemap "+parsedURL.Hostname()+"-"+date+".html")
	if _, err := os.Stat(reportPath); err != nil {
		t.Fatalf("expected report file at %s: %v", reportPath, err)
	}

	report, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}

	reportBody := string(report)
	if !strings.Contains(reportBody, "Sitemap ") {
		t.Fatalf("expected report to include derived test name")
	}
	if !strings.Contains(reportBody, server.URL+"/page") {
		t.Fatalf("expected report to include scanned URL")
	}
}

func TestScanCommandWithSitemapURLFlag(t *testing.T) {
	sitemapXML := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>/page</loc>
  </url>
</urlset>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sitemap.xml":
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(sitemapXML))
		case "/page":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte("<html><head><title>Test</title></head><body>ok</body></html>"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	outputDir := t.TempDir()
	fixedTime := time.Date(2026, time.March, 4, 11, 0, 0, 0, time.UTC)
	date := fixedTime.Format("2006-01-02")
	previousNow := nowFn
	nowFn = func() time.Time { return fixedTime }
	defer func() { nowFn = previousNow }()

	restoreOutputDir := setEnvForTest(t, "WAXE_OUTPUT_DIR", outputDir)
	defer func() {
		restoreOutputDir()
	}()

	previousArgs := os.Args
	os.Args = []string{"axel", "scan", "--sitemap-url", server.URL + "/sitemap.xml"}
	defer func() { os.Args = previousArgs }()

	if err := NewRootCommand().Execute(); err != nil {
		t.Fatalf("scan command failed: %v", err)
	}

	parsedURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	reportPath := filepath.Join(outputDir, "Sitemap "+parsedURL.Hostname()+"-"+date+".html")
	if _, err := os.Stat(reportPath); err != nil {
		t.Fatalf("expected report file at %s: %v", reportPath, err)
	}

	report, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}

	reportBody := string(report)
	if !strings.Contains(reportBody, "Sitemap ") {
		t.Fatalf("expected report to include sitemap test name")
	}
	if !strings.Contains(reportBody, server.URL+"/page") {
		t.Fatalf("expected report to include scanned URL")
	}
}

func TestScanCommandRequiresInput(t *testing.T) {
	previousArgs := os.Args
	os.Args = []string{"axel", "scan"}
	defer func() { os.Args = previousArgs }()

	err := NewRootCommand().Execute()
	if err == nil {
		t.Fatalf("expected error when no base URL or sitemap URL is provided")
	}
	if !strings.Contains(err.Error(), "base URL or sitemap URL is required") {
		t.Fatalf("expected missing input error, got %v", err)
	}
}

func TestScanCommandDefaultsOutputDirToCwd(t *testing.T) {
	sitemapXML := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>/page</loc>
  </url>
</urlset>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/sitemap.xml":
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(sitemapXML))
		case "/page":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte("<html><head><title>Test</title></head><body>ok</body></html>"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	workingDir := t.TempDir()
	fixedTime := time.Date(2026, time.March, 4, 11, 20, 0, 0, time.UTC)
	date := fixedTime.Format("2006-01-02")
	previousNow := nowFn
	nowFn = func() time.Time { return fixedTime }
	defer func() { nowFn = previousNow }()

	restoreSitemapURL := setEnvForTest(t, "WAXE_SITEMAP_URL", server.URL+"/sitemap.xml")
	defer func() {
		restoreSitemapURL()
	}()

	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(workingDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() {
		_ = os.Chdir(previousDir)
	}()

	previousArgs := os.Args
	os.Args = []string{"axel", "scan", server.URL}
	defer func() { os.Args = previousArgs }()

	if err := NewRootCommand().Execute(); err != nil {
		t.Fatalf("scan command failed: %v", err)
	}

	parsedURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	reportPath := filepath.Join(workingDir, "Sitemap "+parsedURL.Hostname()+"-"+date+".html")
	if _, err := os.Stat(reportPath); err != nil {
		t.Fatalf("expected report file at %s: %v", reportPath, err)
	}
}

func TestScanCommandRejectsPositionalBaseURLWithSitemapFlag(t *testing.T) {
	previousArgs := os.Args
	os.Args = []string{"axel", "scan", "https://example.com", "--sitemap-url", "https://example.com/sitemap.xml"}
	defer func() { os.Args = previousArgs }()

	err := NewRootCommand().Execute()
	if err == nil {
		t.Fatalf("expected error when positional base URL and --sitemap-url are both set")
	}
	if !strings.Contains(err.Error(), "positional base URL cannot be used with --sitemap-url") {
		t.Fatalf("expected flag conflict error, got %v", err)
	}
}

func setEnvForTest(t *testing.T, key string, value string) func() {
	previousValue, hadValue := os.LookupEnv(key)
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("set env %s: %v", key, err)
	}

	return func() {
		if !hadValue {
			_ = os.Unsetenv(key)
			return
		}
		_ = os.Setenv(key, previousValue)
	}
}
