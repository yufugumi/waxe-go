#!/usr/bin/env bun
import React, { useState, useEffect } from "react";
import { render, Text, Box, Newline } from "ink";
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

interface AppProps {
  site: string;
}

const App: React.FC<AppProps> = ({ site }) => {
  const [status, setStatus] = useState<string>("Starting...");
  const [isComplete, setIsComplete] = useState<boolean>(false);
  const [error, setError] = useState<string>("");

  useEffect(() => {
    const config = siteConfigs[site];

    if (!config) {
      setError(
        `Invalid site: ${site}. Valid options: ${Object.keys(siteConfigs).join(
          ", "
        )}`
      );
      return;
    }

    const runScan = async () => {
      try {
        setStatus(`Running accessibility scan for ${site}...`);
        await runAccessibilityScan(config);
        setStatus("Scan completed successfully!");
        setIsComplete(true);
      } catch (err) {
        setError(`Error running scan: ${err}`);
      }
    };

    runScan();
  }, [site]);

  if (error) {
    return (
      <Box flexDirection="column">
        <Text color="red">Error: {error}</Text>
      </Box>
    );
  }

  return (
    <Box flexDirection="column" backgroundColor={"black"} padding={1}>
      <Text color="blue" bold>
        waxe 😤
      </Text>
      <Newline />
      <Text>Site: {site}</Text>
      <Text>Status: {status}</Text>
      {isComplete && (
        <>
          <Newline />
          <Text color="green">
            ✓ Accessibility report generated successfully!
          </Text>
          <Text>Check the reports/ directory for the HTML report.</Text>
        </>
      )}
    </Box>
  );
};

const args = process.argv.slice(2);
const siteArg = args.find((arg) => arg.startsWith("--site="))?.split("=")[1];

if (!siteArg) {
  console.error("Usage: bun run scan --site=<site_name>");
  console.error("Valid sites:", Object.keys(siteConfigs).join(", "));
  process.exit(1);
}

render(<App site={siteArg} />);
