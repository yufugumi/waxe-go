package sitemap

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode"
)

// urlset represents the root element of a sitemap XML
type urlset struct {
	XMLName xml.Name `xml:"urlset"`
	URLs    []url    `xml:"url"`
}

// url represents a single URL entry in the sitemap
type url struct {
	Loc string `xml:"loc"`
}

// Parse extracts all URLs from a sitemap XML document.
//
// warnf can be nil to disable warnings for skipped entries.
func Parse(data []byte, warnf func(string, ...any)) ([]string, error) {
	var sitemap urlset

	// Fail fast: return immediately if XML is invalid
	if err := xml.Unmarshal(data, &sitemap); err != nil {
		return nil, err
	}

	// Extract URLs into a clean slice
	urls := make([]string, 0, len(sitemap.URLs))
	for _, u := range sitemap.URLs {
		sanitized := SanitizeLoc(u.Loc)
		if sanitized == "" {
			if warnf != nil {
				warnf("warning: skipping sitemap URL with empty <loc> after sanitization: %q", summarizeLoc(u.Loc))
			}
			continue
		}
		urls = append(urls, sanitized)
	}

	return urls, nil
}

// SanitizeLoc trims whitespace and removes control characters from a sitemap <loc>.
func SanitizeLoc(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	var builder strings.Builder
	builder.Grow(len(trimmed))
	for _, r := range trimmed {
		if unicode.IsControl(r) {
			continue
		}
		builder.WriteRune(r)
	}

	return strings.TrimSpace(builder.String())
}

func summarizeLoc(raw string) string {
	const maxLen = 80
	if raw == "" {
		return ""
	}

	sanitized := SanitizeLoc(raw)
	if sanitized == "" {
		if raw == "" {
			return ""
		}
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			return truncateString(raw, maxLen)
		}
		return truncateString(trimmed, maxLen)
	}

	return truncateString(sanitized, maxLen)
}

func truncateString(value string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	if len(value) <= maxLen {
		return value
	}
	if maxLen <= 3 {
		return value[:maxLen]
	}
	return value[:maxLen-3] + "..."
}

// Fetch retrieves a sitemap from a URL with context for timeout/cancellation
func Fetch(ctx context.Context, url string) ([]byte, error) {
	// Create request with context for timeout/cancellation
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Execute the HTTP request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Fail fast: check HTTP status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Read entire body
	return io.ReadAll(resp.Body)
}
