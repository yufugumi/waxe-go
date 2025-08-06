#!/usr/bin/env bun
import { runAccessibilityScan } from "./axe-scanner-lib";
import type { SiteConfig } from "../types";

const siteConfigs: Record<string, SiteConfig> = {
  test: {
    urlFile: "test.txt",
    testName: "test-site",
    logFile: "test.log",
  },
  wellington: {
    urlFile: "wellington.txt",
    testName: "wellington-govt-nz",
    logFile: "wellington.log",
    excludeRules: ["duplicate-id-active"],
  },
  letstalk: {
    urlFile: "letstalk.txt",
    testName: "lets-talk",
    logFile: "letstalk.log",
  },
  archives: {
    urlFile: "archives.txt",
    testName: "archives-online",
    logFile: "archives.log",
  },
  transportprojects: {
    urlFile: "transportprojects.txt",
    testName: "transportprojects",
    logFile: "transportprojects.log",
  },
  careers: {
    urlFile: "careers.txt",
    testName: "careers",
    logFile: "careers.log",
  },
};

async function main() {
  // Parse command line arguments
  const args = process.argv.slice(2);
  const siteArg = args.find((arg) => arg.startsWith("--site="))?.split("=")[1];

  if (!siteArg) {
    console.error("Usage: bun run scan-simple --site=<site_name>");
    console.error("Valid sites:", Object.keys(siteConfigs).join(", "));
    process.exit(1);
  }

  const config = siteConfigs[siteArg];

  if (!config) {
    console.error(
      `Invalid site: ${siteArg}. Valid options: ${Object.keys(siteConfigs).join(
        ", "
      )}`
    );
    process.exit(1);
  }

  try {
    console.log(`🔍 Wellington Axe - Accessibility Scanner`);
    console.log(`📋 Site: ${siteArg}`);
    console.log(`⚡ Starting accessibility scan...`);

    await runAccessibilityScan(config);

    console.log(`✅ Scan completed successfully!`);
    console.log(`📊 Check the reports/ directory for the HTML report.`);
  } catch (error) {
    console.error(`❌ Error running scan: ${error}`);
    process.exit(1);
  }
}

main().catch(console.error);
