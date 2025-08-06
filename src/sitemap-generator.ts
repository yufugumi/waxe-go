#!/usr/bin/env bun
import { gotScraping } from "crawlee";
import * as fs from "fs-extra";
import * as path from "path";
import type { SitemapOptions } from "../types";

export async function parseSitemap(sitemapUrl: string): Promise<string[]> {
  try {
    const response = await gotScraping(sitemapUrl);
    const sitemapContent = response.body;

    if (typeof sitemapContent !== "string") {
      throw new Error("Sitemap response is not a string");
    }

    // Simple XML parsing to extract URLs
    const urlRegex = /<loc[^>]*>([^<]+)<\/loc>/g;
    const urls: string[] = [];
    let match;

    while ((match = urlRegex.exec(sitemapContent)) !== null) {
      if (match[1]) {
        urls.push(match[1]);
      }
    }

    return urls;
  } catch (error) {
    throw new Error(`Failed to fetch sitemap: ${error}`);
  }
}

export async function generateSitemapUrls(
  options: SitemapOptions
): Promise<void> {
  const { url, max = 1000, output } = options;

  try {
    console.log(`Fetching sitemap from: ${url}`);

    const urls = await parseSitemap(url);

    let filteredUrls = urls;
    if (max && urls.length > max) {
      filteredUrls = urls.slice(0, max);
      console.log(`Limited to ${max} URLs from ${urls.length} total`);
    }

    // Ensure output directory exists
    await fs.ensureDir(path.dirname(output));

    // Write URLs to file
    await fs.writeFile(output, filteredUrls.join("\n"), "utf-8");

    console.log(`Generated ${filteredUrls.length} URLs to ${output}`);
  } catch (error) {
    console.error(`Error generating sitemap URLs: ${error}`);
    process.exit(1);
  }
}

// CLI functionality
async function main() {
  if (import.meta.main) {
    const args = process.argv.slice(2);

    if (args.length < 1) {
      console.error(
        "Usage: bun run sitemap-generator.ts <url> [--max <number>] [--output <file>]"
      );
      process.exit(1);
    }

    const url = args[0];
    if (!url) {
      console.error("URL is required");
      process.exit(1);
    }

    let max: number | undefined;
    let output = "urls.txt";

    for (let i = 1; i < args.length; i++) {
      if (args[i] === "--max" && args[i + 1]) {
        const maxValue = args[i + 1];
        if (maxValue) {
          max = parseInt(maxValue);
        }
        i++;
      } else if (args[i] === "--output" && args[i + 1]) {
        const outputValue = args[i + 1];
        if (outputValue) {
          output = outputValue;
        }
        i++;
      }
    }

    await generateSitemapUrls({ url, max, output });
  }
}

main().catch(console.error);
