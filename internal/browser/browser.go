package browser

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/yufugumi/axel/internal/useragent"
)

func NewAllocator(ctx context.Context) (context.Context, context.CancelFunc) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		// CI runners can be slow to boot Chrome under race detector.
		// Extend the websocket startup wait to avoid flaky timeouts.
		chromedp.WSURLReadTimeout(60*time.Second),
	)

	if chromePath := os.Getenv("CHROME_PATH"); chromePath != "" {
		opts = append(opts, chromedp.ExecPath(chromePath))
	}

	return chromedp.NewExecAllocator(ctx, opts...)
}

func NewTab(ctx context.Context) (context.Context, context.CancelFunc) {
	return chromedp.NewContext(ctx)
}

func NewBrowser(ctx context.Context) (context.Context, context.CancelFunc) {
	allocCtx, allocCancel := NewAllocator(ctx)
	tabCtx, tabCancel := NewTab(allocCtx)

	return tabCtx, func() {
		tabCancel()
		allocCancel()
	}
}

// NewBrowserContext creates a shared browser context from an allocator context
// and ensures the browser process is started. All tabs created as children of
// this context share the same browser process.
func NewBrowserContext(allocCtx context.Context) (context.Context, context.CancelFunc, error) {
	ctx, cancel := chromedp.NewContext(allocCtx)
	if err := chromedp.Run(ctx); err != nil {
		cancel()
		return nil, nil, fmt.Errorf("start browser: %w", err)
	}
	return ctx, cancel, nil
}

func Navigate(ctx context.Context, url string) error {
	return chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitReady("body"),
	)
}

var analyticsBlockedURLs = []string{
	"*google-analytics.com*",
	"*googletagmanager.com*",
	"*googlesyndication.com*",
	"*favicon.ico*",
}

var mediaBlockedURLs = []string{
	"*.png*",
	"*.jpg*",
	"*.jpeg*",
	"*.gif*",
	"*.webp*",
	"*.svg*",
	"*.ico*",
	"*.avif*",
	"*.mp4*",
	"*.webm*",
	"*.mov*",
	"*.m4v*",
	"*.mp3*",
	"*.wav*",
	"*.ogg*",
	"*.m4a*",
	"*.aac*",
}

func BlockRequests(ctx context.Context, blockMedia bool) error {
	blocked := make([]string, 0, len(analyticsBlockedURLs)+len(mediaBlockedURLs))
	blocked = append(blocked, analyticsBlockedURLs...)
	if blockMedia {
		blocked = append(blocked, mediaBlockedURLs...)
	}

	return chromedp.Run(ctx,
		network.Enable(),
		emulation.SetUserAgentOverride(useragent.CommonUserAgent),
		network.SetBlockedURLs(blocked),
	)
}

func BlockAnalytics(ctx context.Context) error {
	return BlockRequests(ctx, false)
}
