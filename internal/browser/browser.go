package browser

import (
	"context"
	"os"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

func NewBrowser(ctx context.Context) (context.Context, context.CancelFunc) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
	)

	if chromePath := os.Getenv("CHROME_PATH"); chromePath != "" {
		opts = append(opts, chromedp.ExecPath(chromePath))
	}

	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)

	ctx, cancel2 := chromedp.NewContext(allocCtx)

	// Return combined cancel function
	return ctx, func() {
		cancel2()
		cancel()
	}
}

func Navigate(ctx context.Context, url string) error {
	return chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitReady("body"),
	)
}

func BlockAnalytics(ctx context.Context) error {
	return chromedp.Run(ctx,
		network.Enable(),
		network.SetBlockedURLs([]string{
			"*google-analytics.com*",
			"*googletagmanager.com*",
			"*googlesyndication.com*",
			"*favicon.ico*",
		}),
	)
}
