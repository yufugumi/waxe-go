package scanner

import "time"

// Violation represents an accessibility violation from axe-core
type Violation struct {
	ID          string `json:"id"`
	Impact      string `json:"impact"`
	Help        string `json:"help"`
	Description string `json:"description"`
	Nodes       []Node `json:"nodes"`
}

// Node represents an HTML node with accessibility issues
type Node struct {
	HTML string `json:"html"`
}

// ScanResult represents the result of scanning a single URL
type ScanResult struct {
	URL        string      `json:"url"`
	Violations []Violation `json:"violations"`
	Timestamp  time.Time   `json:"timestamp"`
	Error      string      `json:"error,omitempty"`
}

// ProgressUpdate captures the scan progress for CLI reporting.
type ProgressUpdate struct {
	Processed int
	Total     int
	Percent   float64
	URL       string
}

// ProgressReporter receives scan progress updates.
type ProgressReporter func(ProgressUpdate)

// ReportData is the data structure passed to the HTML template
type ReportData struct {
	TestName string
	Date     string
	Results  []*ScanResult
}
