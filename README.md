# Wellington Axe Runners (WAXE)

WAXE is a Go-based accessibility scanner that uses [chromedp](https://github.com/chromedp/chromedp) with [axe-core](https://github.com/dequelabs/axe-core). It runs monthly via GitHub Actions against six Wellington Council sites and publishes HTML reports to GitHub Releases (no CSV output). The CLI runs locally and does not depend on GitHub Actions.

## Current behavior

- Runs monthly via GitHub Actions against six Wellington Council sites.
- Uses 10 concurrent workers.
- URLs are chunked into batches of 50 with a 2s delay between chunks.
- Retries each failed URL up to 2 times with a 2s delay.
- Outputs HTML reports into GitHub Releases.

## Run locally

The `axed` CLI runs locally and writes HTML reports to your current working directory by default.

While scans run, the CLI prints a single-line progress update with processed/total counts, percent, and current URL.

```bash
go test ./...
go build -o axed ./cmd/scanner
./axed scan --site=wellington
```

Scan a site via sitemap URL (no site config required):

```bash
go build -o axed ./cmd/scanner
./axed scan --sitemap-url https://example.com/sitemap.xml
```

Optionally override the base URL for relative sitemap entries and output dir:

```bash
go build -o axed ./cmd/scanner
WAXE_OUTPUT_DIR=./reports ./axed scan --sitemap-url https://example.com/sitemap.xml --base-url https://example.com
```

Set a per-URL timeout (Go duration format):

```bash
go build -o axed ./cmd/scanner
./axed scan --site=wellington --timeout 45s
```

## Environment overrides

- `WAXE_SITEMAP_URL`
- `WAXE_BASE_URL`
- `WAXE_FALLBACK_URLS`
- `WAXE_FALLBACK_URLS_FILE`
- `WAXE_MAX_URLS`
- `WAXE_OUTPUT_DIR`
- `CHROME_PATH`
- `WAXE_ALLOW_SITEMAP_OVERRIDE` (set to `true` to allow `WAXE_SITEMAP_URL` with `--site`)

> [!NOTE]
> `WAXE_SITEMAP_URL` is only used when `--site` is not provided. Use `--sitemap-url` to override the sitemap, or set `WAXE_ALLOW_SITEMAP_OVERRIDE=true` to opt into environment overrides while scanning a configured site.

> [!NOTE]
> This project is still a work in progress and has rough edges. It can scan other sites with axe-core, but it needs more configuration and polish to be broadly reusable.

## Known issues

- Some sites have slow or flaky pages that can still time out despite retries.
- The HTML reports are useful but still need refinement for long-term analysis and comparison.
- Runtime can be slow on large sitemaps even with concurrency.
