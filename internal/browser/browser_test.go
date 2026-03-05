package browser

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewBrowser(t *testing.T) {
	ctx := context.Background()

	browserCtx, cancel := NewBrowser(ctx)
	defer cancel()

	if browserCtx == nil {
		t.Fatal("Expected browser context to be created")
	}

	// Verify we can use the context (basic sanity check)
	select {
	case <-browserCtx.Done():
		t.Fatal("Browser context should not be done immediately")
	default:
		// Good - context is active
	}
}

func TestNavigate(t *testing.T) {
	ctx := context.Background()
	browserCtx, cancel := NewBrowser(ctx)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><body>ok</body></html>"))
	}))
	defer server.Close()

	// Create timeout context
	timeoutCtx, timeoutCancel := context.WithTimeout(browserCtx, 30*time.Second)
	defer timeoutCancel()

	// Navigate to local test server
	err := Navigate(timeoutCtx, server.URL)
	if err != nil {
		t.Fatalf("Navigate failed: %v", err)
	}
}

func TestBlockAnalytics(t *testing.T) {
	ctx := context.Background()
	browserCtx, cancel := NewBrowser(ctx)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><body>ok</body></html>"))
	}))
	defer server.Close()

	// Block analytics
	if err := BlockAnalytics(browserCtx); err != nil {
		t.Fatalf("BlockAnalytics failed: %v", err)
	}

	// Navigate to a page that would load analytics
	timeoutCtx, timeoutCancel := context.WithTimeout(browserCtx, 30*time.Second)
	defer timeoutCancel()

	// This should work even with analytics blocked
	err := Navigate(timeoutCtx, server.URL)
	if err != nil {
		t.Fatalf("Navigate failed: %v", err)
	}
}
