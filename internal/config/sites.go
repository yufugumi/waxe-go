package config

// SiteConfig holds configuration for a site to be tested
type SiteConfig struct {
	URLFile      string
	TestName     string
	URL          string
	ExcludeRules []string
	UserAgent    string
}

// Sites contains all site configurations keyed by site identifier
var Sites = map[string]SiteConfig{
	"test": {
		URLFile:  "urls/test.txt",
		TestName: "Test Site",
		URL:      "https://example.com/sitemap.xml",
	},
	"wellington": {
		URLFile:  "urls/wellington.txt",
		TestName: "Wellington City Council",
		URL:      "https://wellington.govt.nz/sitemap.xml",
	},
	"library": {
		URLFile:  "urls/library.txt",
		TestName: "Wellington City Libraries",
		URL:      "https://wcl.govt.nz/sitemap.xml",
	},
	"letstalk": {
		URLFile:      "urls/letstalk.txt",
		TestName:     "Let's Talk Wellington",
		URL:          "https://letstalk.wellington.govt.nz/sitemap.xml",
		ExcludeRules: []string{"color-contrast"},
	},
	"archives": {
		URLFile:  "urls/archives.txt",
		TestName: "Archives Online",
		URL:      "https://archivesonline.wcc.govt.nz/sitemap.xml",
	},
	"transportprojects": {
		URLFile:  "urls/transportprojects.txt",
		TestName: "Transport Projects",
		URL:      "https://transportprojects.org.nz/sitemap.xml",
	},
	"careers": {
		URLFile:  "urls/careers.txt",
		TestName: "Wellington Careers",
		URL:      "https://careers.wellington.govt.nz/sitemap.xml",
	},
}
