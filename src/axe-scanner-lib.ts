import { chromium } from "playwright";
import type { Browser, BrowserContext } from "playwright";
import AxeBuilder from "@axe-core/playwright";
import { format } from "date-fns";
import * as fs from "fs-extra";
import * as path from "path";
import Handlebars from "handlebars";
import type { ScanConfig } from "../types";

interface ProcessResult {
  url: string;
  violations: any[];
}

class ProgressTracker {
  private current = 0;
  private total: number;
  private startTime: number;

  constructor(total: number) {
    this.total = total;
    this.startTime = Date.now();
  }

  update(increment = 1) {
    this.current += increment;
    const percentage = Math.round((this.current / this.total) * 100);
    const elapsed = Date.now() - this.startTime;
    const eta =
      this.current > 0
        ? (elapsed / this.current) * (this.total - this.current)
        : 0;

    process.stdout.write(
      `\r[${percentage}%] ${this.current}/${
        this.total
      } URLs processed (ETA: ${Math.round(eta / 1000)}s)`
    );

    if (this.current === this.total) {
      console.log();
    }
  }

  write(message: string) {
    process.stdout.write(`\r${" ".repeat(80)}\r`);
    console.log(message);
    // Redraw progress
    const percentage = Math.round((this.current / this.total) * 100);
    process.stdout.write(
      `\r[${percentage}%] ${this.current}/${this.total} URLs processed`
    );
  }
}

export async function ensureDirectories(): Promise<void> {
  const directories = ["reports", "logs", "urls"];
  for (const dir of directories) {
    await fs.ensureDir(dir);
  }
}

export async function processUrl(
  url: string,
  browser: Browser,
  semaphore: { acquire: () => Promise<void>; release: () => void },
  progressBar: ProgressTracker,
  excludeRules?: string[]
): Promise<ProcessResult> {
  const maxRetries = 2;
  let retryCount = 0;

  while (retryCount <= maxRetries) {
    let context: BrowserContext | null = null;

    try {
      await semaphore.acquire();

      context = await browser.newContext({
        viewport: { width: 1280, height: 720 },
        userAgent: "WAXE accessibility testing bot",
      });

      // Block Google Analytics and GTM requests
      await context.route("**/*google-analytics*", (route) => route.abort());
      await context.route("**/*googletagmanager*", (route) => route.abort());
      await context.route("**/*gtm.js*", (route) => route.abort());
      await context.route("**/*analytics.js*", (route) => route.abort());
      await context.route("**/*ga.js*", (route) => route.abort());

      const page = await context.newPage();
      await page.goto(url, { timeout: 30000, waitUntil: "networkidle" });

      // Add script to block GTM iframes
      await page.addScriptTag({
        content: `
          (function() {
            const removeGTMIframes = () => {
              const iframes = document.querySelectorAll('iframe');
              iframes.forEach(iframe => {
                if (iframe.src && (
                  iframe.src.includes('googletagmanager') || 
                  iframe.src.includes('gtm') ||
                  iframe.src.includes('google-analytics')
                )) {
                  iframe.remove();
                }
              });
            };
            
            removeGTMIframes();
            
            const observer = new MutationObserver((mutations) => {
              removeGTMIframes();
            });
            
            observer.observe(document.documentElement, {
              childList: true,
              subtree: true
            });
          })();
        `,
      });

      await page.waitForLoadState("networkidle");
      await new Promise((resolve) => setTimeout(resolve, 2000));

      // Run accessibility tests
      let axeBuilder = new AxeBuilder({ page }).withTags(["wcag22aa"]);

      if (excludeRules) {
        axeBuilder = axeBuilder.disableRules(excludeRules);
      }

      const results = await axeBuilder.analyze();

      if (results.violations.length > 0) {
        progressBar.write(
          `${results.violations.length} violations found on ${url}`
        );
      } else {
        progressBar.write(`No violations found on ${url}`);
      }

      return { url, violations: results.violations };
    } catch (error) {
      retryCount++;
      if (retryCount <= maxRetries) {
        progressBar.write(
          `Error on ${url}, retrying (${retryCount}/${maxRetries}): ${error}`
        );
        await new Promise((resolve) => setTimeout(resolve, 2000));
      } else {
        progressBar.write(
          `Failed to process ${url} after ${maxRetries} retries: ${error}`
        );
        return { url, violations: [] };
      }
    } finally {
      if (context) {
        await context.close();
      }
      semaphore.release();
      progressBar.update(1);
    }
  }

  return { url, violations: [] };
}

export async function processUrls(
  urls: string[],
  excludeRules?: string[]
): Promise<ProcessResult[]> {
  const results: ProcessResult[] = [];
  const maxConcurrent = 10;

  let activeCount = 0;
  const semaphore = {
    acquire: async () => {
      while (activeCount >= maxConcurrent) {
        await new Promise((resolve) => setTimeout(resolve, 100));
      }
      activeCount++;
    },
    release: () => {
      activeCount--;
    },
  };

  const browser = await chromium.launch({ headless: true });

  try {
    const progressBar = new ProgressTracker(urls.length);

    const chunkSize = 20;
    for (let i = 0; i < urls.length; i += chunkSize) {
      const chunk = urls.slice(i, i + chunkSize);

      const chunkPromises = chunk.map((url) =>
        processUrl(url, browser, semaphore, progressBar, excludeRules)
      );

      const chunkResults = await Promise.all(chunkPromises);
      results.push(...chunkResults);

      await new Promise((resolve) => setTimeout(resolve, 2000));
    }
  } finally {
    await browser.close();
  }

  return results;
}

export async function runAccessibilityScan(config: ScanConfig): Promise<void> {
  await ensureDirectories();

  const {
    urlFile,
    testName,
    logFile = "accessibility_scan.log",
    excludeRules,
  } = config;

  const now = new Date();
  const dateString = format(now, "dd-MM-yyyy");

  const fullLogPath = path.join("logs", logFile);
  const logStream = fs.createWriteStream(fullLogPath, { flags: "a" });

  const log = (message: string) => {
    const timestamp = format(new Date(), "yyyy-MM-dd HH:mm:ss");
    const logMessage = `${timestamp} - INFO - ${message}`;
    console.log(logMessage);
    logStream.write(logMessage + "\n");
  };

  try {
    const fullUrlPath = path.join("urls", urlFile);
    const urlContent = await fs.readFile(fullUrlPath, "utf-8");
    const urls = urlContent
      .split("\n")
      .map((url) => url.trim())
      .filter((url) => url);

    log(`Starting accessibility scan of ${urls.length} URLs`);

    const results = await processUrls(urls, excludeRules);

    const templateContent = await fs.readFile(
      "template-handlebars.html",
      "utf-8"
    );
    const template = Handlebars.compile(templateContent);

    const htmlOutput = template({
      date: dateString,
      results,
      test_name: testName,
    });

    const reportFilename = `accessibility-report-${testName}-${dateString}.html`;
    const fullReportPath = path.join("reports", reportFilename);
    await fs.writeFile(fullReportPath, htmlOutput, "utf-8");

    log(`Accessibility report generated: ${fullReportPath}`);
  } finally {
    logStream.end();
  }
}
