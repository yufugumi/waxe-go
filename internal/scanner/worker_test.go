package scanner

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestScanURLsConcurrent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	const workers = 3
	gate := make(chan struct{})
	reachedTarget := make(chan struct{}, 1)

	var inFlight int64
	var maxInFlight int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := atomic.AddInt64(&inFlight, 1)
		for {
			observedMax := atomic.LoadInt64(&maxInFlight)
			if current <= observedMax {
				break
			}
			if atomic.CompareAndSwapInt64(&maxInFlight, observedMax, current) {
				break
			}
		}
		defer atomic.AddInt64(&inFlight, -1)

		if current == workers {
			select {
			case reachedTarget <- struct{}{}:
			default:
			}
		}

		<-gate
		time.Sleep(50 * time.Millisecond)

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><body>ok</body></html>"))
	}))
	defer server.Close()

	urls := make([]string, 0, 10)
	for i := 0; i < 10; i++ {
		urls = append(urls, fmt.Sprintf("%s/?page=%d", server.URL, i))
	}

	resultChan := make(chan []*ScanResult, 1)
	errChan := make(chan error, 1)
	go func() {
		results, err := ScanURLs(ctx, urls, workers, nil, DefaultPerURLTimeout)
		if err != nil {
			errChan <- err
			return
		}
		resultChan <- results
	}()

	select {
	case <-reachedTarget:
		close(gate)
	case <-ctx.Done():
		close(gate)
		t.Fatalf("timed out waiting for %d concurrent requests", workers)
	}

	var results []*ScanResult
	select {
	case err := <-errChan:
		t.Fatalf("ScanURLs returned error: %v", err)
	case results = <-resultChan:
	case <-ctx.Done():
		t.Fatalf("ScanURLs timed out: %v", ctx.Err())
	}
	if len(results) != len(urls) {
		t.Fatalf("expected %d results, got %d", len(urls), len(results))
	}

	if maxInFlight < workers {
		t.Fatalf("expected at least %d concurrent requests, saw %d", workers, maxInFlight)
	}
	if maxInFlight > workers {
		t.Fatalf("expected no more than %d concurrent requests, saw %d", workers, maxInFlight)
	}
}

func TestScanURLRetries(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var requestCount int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := atomic.AddInt64(&requestCount, 1)
		if attempt <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><body>ok</body></html>"))
	}))
	defer server.Close()

	urls := []string{server.URL}
	results, err := ScanURLs(ctx, urls, 1, nil, DefaultPerURLTimeout)
	if err != nil {
		t.Fatalf("expected ScanURLs to succeed after retries, got error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	observedRequests := atomic.LoadInt64(&requestCount)
	if observedRequests != 3 {
		t.Fatalf("expected 3 requests, got %d", observedRequests)
	}
}
