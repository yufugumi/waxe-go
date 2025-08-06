export interface SiteConfig {
  urlFile: string;
  testName: string;
  logFile: string;
  excludeRules?: string[];
}

export interface SitemapOptions {
  url: string;
  max?: number;
  output: string;
}

export interface ScanConfig {
  urlFile: string;
  testName: string;
  logFile?: string;
  excludeRules?: string[];
}
