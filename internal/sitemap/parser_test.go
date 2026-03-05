package sitemap

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseSitemap(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>https://example.com/</loc>
  </url>
  <url>
    <loc>https://example.com/about</loc>
  </url>
  <url>
    <loc>https://example.com/contact</loc>
  </url>
</urlset>`

	expected := []string{
		"https://example.com/",
		"https://example.com/about",
		"https://example.com/contact",
	}

	urls, err := Parse([]byte(xmlData), nil)

	// Guard clause: fail early if error occurs
	if err != nil {
		t.Fatalf("Parse() returned unexpected error: %v", err)
	}

	// Verify we got the expected number of URLs
	if len(urls) != len(expected) {
		t.Errorf("Parse() returned %d URLs, want %d", len(urls), len(expected))
	}

	// Verify each URL matches expected
	for i, url := range urls {
		if url != expected[i] {
			t.Errorf("urls[%d] = %q, want %q", i, url, expected[i])
		}
	}
}

func TestParseSitemapSanitizesLoc(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc> https://example.com/clean </loc>
  </url>
  <url>
    <loc>https://example.com/with-tab	path</loc>
  </url>
  <url>
    <loc>	</loc>
  </url>
</urlset>`

	urls, err := Parse([]byte(xmlData), nil)
	if err != nil {
		t.Fatalf("Parse() returned unexpected error: %v", err)
	}

	if len(urls) != 2 {
		t.Fatalf("Parse() returned %d URLs, want 2", len(urls))
	}

	expected := []string{
		"https://example.com/clean",
		"https://example.com/with-tabpath",
	}

	for i, url := range urls {
		if url != expected[i] {
			t.Errorf("urls[%d] = %q, want %q", i, url, expected[i])
		}
	}
}

func TestParseSitemapReturnsEmptySliceWhenNoValidLoc(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>   </loc>
  </url>
  <url>
    <loc>	</loc>
  </url>
</urlset>`

	urls, err := Parse([]byte(xmlData), nil)
	if err != nil {
		t.Fatalf("Parse() returned unexpected error: %v", err)
	}

	if len(urls) != 0 {
		t.Fatalf("Parse() returned %d URLs, want 0", len(urls))
	}
}

func TestParseSitemapWarnsWithRawLocWhenSanitizedEmpty(t *testing.T) {
	longValue := strings.Repeat(" ", 120)
	xmlData := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>` + longValue + `</loc>
  </url>
</urlset>`

	var warnings []string
	warnf := func(format string, args ...any) {
		warnings = append(warnings, fmt.Sprintf(format, args...))
	}

	_, err := Parse([]byte(xmlData), warnf)
	if err != nil {
		t.Fatalf("Parse() returned unexpected error: %v", err)
	}

	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warnings))
	}

	warning := warnings[0]
	if !strings.Contains(warning, "after sanitization") {
		t.Fatalf("expected warning to mention sanitization, got %q", warning)
	}
	if !strings.Contains(warning, "...") {
		t.Fatalf("expected warning to include truncated loc value, got %q", warning)
	}
}

func TestFetchSitemap(t *testing.T) {
	// Create test server with valid sitemap XML
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://example.com/</loc></url>
</urlset>`))
	}))
	defer ts.Close()

	// Test successful fetch
	ctx := context.Background()
	data, err := Fetch(ctx, ts.URL+"/sitemap.xml")
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if len(data) == 0 {
		t.Fatal("Expected non-empty data")
	}
}

func TestFetchSitemapWithTimeout(t *testing.T) {
	// Create test server that delays response
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://example.com/</loc></url>
</urlset>`))
	}))
	defer ts.Close()

	// Test with short timeout that should fail
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := Fetch(ctx, ts.URL+"/sitemap.xml")
	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}
}
