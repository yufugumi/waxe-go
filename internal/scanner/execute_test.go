package scanner

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yufugumi/waxe-go/internal/browser"
)

func TestExecuteAxe(t *testing.T) {
	ctx := context.Background()
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html>
<html lang="en">
<head><meta charset="utf-8" /><title>Low Contrast</title></head>
<body style="background:#ffffff;">
  <p style="color:#f7f7f7;">Low contrast text</p>
</body>
</html>`))
	}))
	defer testServer.Close()

	browserCtx, cancel := browser.NewBrowser(ctx)
	defer cancel()

	timeoutCtx, timeoutCancel := context.WithTimeout(browserCtx, 30*time.Second)
	defer timeoutCancel()

	if err := browser.Navigate(timeoutCtx, testServer.URL); err != nil {
		t.Fatalf("Navigate failed: %v", err)
	}

	if err := InjectAxeCore(timeoutCtx); err != nil {
		t.Fatalf("InjectAxeCore failed: %v", err)
	}

	violations, err := ExecuteAxe(timeoutCtx, nil)
	if err != nil {
		t.Fatalf("ExecuteAxe failed: %v", err)
	}

	if len(violations) == 0 {
		t.Fatal("Expected at least one violation")
	}
}
