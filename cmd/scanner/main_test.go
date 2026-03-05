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

	restoreSiteURL := setEnvForTest(t, "WAXE_BASE_URL", server.URL)
	restoreTestSitemapURL := setEnvForTest(t, "WAXE_TEST_SITEMAP_URL", server.URL+"/sitemap.xml")
	restoreOutputDir := setEnvForTest(t, "WAXE_OUTPUT_DIR", outputDir)
	defer func() {
		restoreSiteURL()
		restoreTestSitemapURL()
		restoreOutputDir()
	}()

	previousArgs := os.Args
	os.Args = []string{"axed", "scan", "--site=test"}
	defer func() { os.Args = previousArgs }()

	if err := NewRootCommand().Execute(); err != nil {
		t.Fatalf("scan command failed: %v", err)
	}

	reportPath := filepath.Join(outputDir, "Test Site-"+date+".html")
	if _, err := os.Stat(reportPath); err != nil {
		t.Fatalf("expected report file at %s: %v", reportPath, err)
	}

	report, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}

	reportBody := string(report)
	if !strings.Contains(reportBody, "Test Site") {
		t.Fatalf("expected report to include test name")
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
	os.Args = []string{"axed", "scan", "--sitemap-url", server.URL + "/sitemap.xml"}
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

func TestScanCommandUsesEnvSitemapWhenSiteMissing(t *testing.T) {
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
	fixedTime := time.Date(2026, time.March, 4, 11, 10, 0, 0, time.UTC)
	date := fixedTime.Format("2006-01-02")
	previousNow := nowFn
	nowFn = func() time.Time { return fixedTime }
	defer func() { nowFn = previousNow }()

	restoreOutputDir := setEnvForTest(t, "WAXE_OUTPUT_DIR", outputDir)
	restoreSitemapURL := setEnvForTest(t, "WAXE_SITEMAP_URL", server.URL+"/sitemap.xml")
	defer func() {
		restoreOutputDir()
		restoreSitemapURL()
	}()

	previousArgs := os.Args
	os.Args = []string{"axed", "scan"}
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
	os.Args = []string{"axed", "scan"}
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

func TestScanCommandRejectsEnvSitemapWithSiteFlag(t *testing.T) {
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

	restoreSitemapURL := setEnvForTest(t, "WAXE_SITEMAP_URL", server.URL+"/sitemap.xml")
	defer func() {
		restoreSitemapURL()
	}()

	previousArgs := os.Args
	os.Args = []string{"axed", "scan", "--site=test"}
	defer func() { os.Args = previousArgs }()

	err := NewRootCommand().Execute()
	if err == nil {
		t.Fatalf("expected error when WAXE_SITEMAP_URL is set with --site")
	}
	if !strings.Contains(err.Error(), "WAXE_SITEMAP_URL") {
		t.Fatalf("expected error to mention WAXE_SITEMAP_URL, got %v", err)
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
