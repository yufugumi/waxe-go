**axel** is a CLI accessibility (a11y) scanner that uses [chromedp](https://github.com/chromedp/chromedp) with [axe-core](https://github.com/dequelabs/axe-core) that finds common a11y issues and creates an HTML report based on its findings.

Create a standalone executable binary using `make build` that you can then use:

```bash
make build
./axel scan https://example.com
```

By default it will search common paths for sitemaps, and if not found will generate one. Otherwise, you can specify the sitemap location if known:

```bash
./axel scan --sitemap-url https://example.com/weird-path/sitemap.xml
```

Sitemap discovery order when using a positional URL:

1. `robots.txt` Sitemap entries (respects redirects)
2. `https://example.com/sitemap.xml`
3. `https://example.com/sitemap_index.xml`
4. `https://example.com/sitemap-index.xml`

If discovery yields no URLs, `axel` crawls the site (same host only) to build a URL list. Crawling respects `robots.txt`, uses a breadth-first traversal with depth 5, delays 300ms between requests, skips non-HTML responses, and de-duplicates URLs.

Set a per-URL timeout:

```bash
./axel scan https://example.com --timeout 45s
```

Tune concurrency and retry settings:

```bash
./axel scan https://example.com --workers 6 --retries 1 --retry-delay 1s
```

Adjust chunk pacing for large sitemaps:

```bash
./axel scan https://example.com --chunk-delay 250ms --chunk-delay-max 2s
```
