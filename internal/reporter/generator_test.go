package reporter

import (
	"strings"
	"testing"
	"time"

	"github.com/yufugumi/waxe-go/internal/scanner"
)

func TestGenerateReport(t *testing.T) {
	results := []*scanner.ScanResult{
		{
			URL: "https://example.com/",
			Violations: []scanner.Violation{
				{
					ID:          "color-contrast",
					Impact:      "serious",
					Help:        "Elements must have sufficient color contrast",
					Description: "Ensures the contrast between foreground and background colors meets WCAG 2 AA minimum contrast ratio thresholds",
					Nodes: []scanner.Node{
						{HTML: "<div>Some text</div>"},
					},
				},
			},
			Timestamp: time.Now(),
		},
	}

	html, err := Generate(results, "Test Site", "2026-03-05")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Verify HTML contains expected content
	htmlStr := string(html)
	if !strings.Contains(htmlStr, "Test Site") {
		t.Error("Expected HTML to contain test name")
	}
	if !strings.Contains(htmlStr, "2026-03-05") {
		t.Error("Expected HTML to contain date")
	}
	if !strings.Contains(htmlStr, "https://example.com/") {
		t.Error("Expected HTML to contain URL")
	}
	if !strings.Contains(htmlStr, "color-contrast") {
		t.Error("Expected HTML to contain violation ID")
	}
}
