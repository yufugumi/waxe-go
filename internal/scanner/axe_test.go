package scanner

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/yufugumi/waxe-go/internal/browser"
)

func TestInjectAxeCore(t *testing.T) {
	ctx := context.Background()
	browserCtx, cancel := browser.NewBrowser(ctx)
	defer cancel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><body>ok</body></html>"))
	}))
	defer server.Close()

	timeoutCtx, timeoutCancel := context.WithTimeout(browserCtx, 30*time.Second)
	defer timeoutCancel()

	if err := browser.Navigate(timeoutCtx, server.URL); err != nil {
		t.Fatalf("Navigate failed: %v", err)
	}

	if err := InjectAxeCore(timeoutCtx); err != nil {
		t.Fatalf("InjectAxeCore failed: %v", err)
	}

	var exists bool
	if err := chromedp.Run(timeoutCtx, chromedp.Evaluate(`typeof axe === 'object'`, &exists)); err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if !exists {
		t.Fatal("Expected axe to be injected")
	}
}
