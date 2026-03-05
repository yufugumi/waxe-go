package reporter

import (
	"bytes"
	"html/template"
	"path/filepath"
	"runtime"

	"github.com/yufugumi/waxe-go/internal/scanner"
)

// Generate creates an HTML accessibility report from scan results.
// It loads the report template and executes it with the provided data.
func Generate(results []*scanner.ScanResult, testName string, date string) ([]byte, error) {
	_, filename, _, _ := runtime.Caller(0)
	currentDir := filepath.Dir(filename)
	projectRoot := filepath.Join(currentDir, "..", "..")
	templatePath := filepath.Join(projectRoot, "templates", "report.html")

	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return nil, err
	}

	data := scanner.ReportData{
		TestName: testName,
		Date:     date,
		Results:  results,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
