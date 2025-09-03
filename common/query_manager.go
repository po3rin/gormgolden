package common

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/pingcap/tidb/parser"
	"github.com/pingcap/tidb/parser/format"
	_ "github.com/pingcap/tidb/parser/test_driver"
	"gotest.tools/v3/golden"
)

// QueryManager manages SQL query recording with thread-safe operations
type QueryManager struct {
	mu         sync.Mutex
	queries    []string
	enabled    bool
	goldenFile string
}

// NewQueryManager creates a new QueryManager instance
func NewQueryManager(goldenFile string) *QueryManager {
	return &QueryManager{
		queries:    []string{},
		enabled:    true,
		goldenFile: goldenFile,
	}
}

// normalize normalizes SQL query using TiDB parser
func (qm *QueryManager) normalize(query string) string {
	if query == "" {
		return query
	}

	// Remove comments first
	query = parser.TrimComment(query)
	
	// After removing comments, check if query is empty or only whitespace
	if strings.TrimSpace(query) == "" {
		return ""
	}
	
	// Parse and normalize the SQL
	p := parser.New()
	stmts, _, err := p.Parse(query, "", "")
	if err != nil {
		// If parsing fails, fall back to basic normalization
		return qm.basicNormalize(query)
	}
	
	if len(stmts) == 0 {
		return query
	}
	
	// Use the normalized string representation
	var buf strings.Builder
	for i, stmt := range stmts {
		if i > 0 {
			buf.WriteString("; ")
		}
		if err := stmt.Restore(format.NewRestoreCtx(format.RestoreKeyWordUppercase|format.RestoreNameBackQuotes, &buf)); err != nil {
			// If restore fails, fall back to basic normalization
			return qm.basicNormalize(query)
		}
	}
	
	return buf.String()
}

// basicNormalize provides basic SQL normalization as fallback
func (qm *QueryManager) basicNormalize(query string) string {
	// Remove extra whitespace
	query = strings.TrimSpace(query)
	query = strings.ReplaceAll(query, "\n", " ")
	query = strings.ReplaceAll(query, "\t", " ")
	
	// Remove extra spaces
	for strings.Contains(query, "  ") {
		query = strings.ReplaceAll(query, "  ", " ")
	}
	
	// Remove unnecessary parentheses that GORM v1 adds
	query = strings.ReplaceAll(query, "( ", "(")
	query = strings.ReplaceAll(query, " )", ")")
	
	return query
}

// normalizeForComparison normalizes SQL for comparison by removing charset prefixes and extra parentheses
func (qm *QueryManager) normalizeForComparison(query string) string {
	// Start with basic normalization
	query = qm.basicNormalize(query)
	
	// Remove MySQL charset prefixes like _UTF8MB4
	utf8mb4Regex := regexp.MustCompile(`_UTF8MB4([0-9A-Za-z]+)`)
	query = utf8mb4Regex.ReplaceAllString(query, "$1")
	
	// Normalize parentheses around conditions
	// Remove outer parentheses around WHERE conditions
	parenRegex := regexp.MustCompile(`\(([^()]+)\)`)
	query = parenRegex.ReplaceAllStringFunc(query, func(match string) string {
		// Keep parentheses for IN clauses and function calls
		inner := match[1 : len(match)-1]
		if strings.Contains(inner, " IN ") || strings.Contains(inner, "(),") {
			return match
		}
		// Remove outer parentheses for simple conditions
		if strings.Count(inner, "=") == 1 && !strings.Contains(inner, " AND ") && !strings.Contains(inner, " OR ") {
			return inner
		}
		return match
	})
	
	return query
}

// AddQuery adds a SQL query to the recorded list
func (qm *QueryManager) AddQuery(query string) {
	if !qm.enabled || query == "" {
		return
	}
	
	// Normalize the query before adding
	normalizedQuery := qm.normalize(query)
	
	qm.mu.Lock()
	defer qm.mu.Unlock()
	qm.queries = append(qm.queries, normalizedQuery)
}

// Enable enables query recording
func (qm *QueryManager) Enable() {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	qm.enabled = true
}

// Disable disables query recording
func (qm *QueryManager) Disable() {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	qm.enabled = false
}

// Clear clears all recorded queries
func (qm *QueryManager) Clear() {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	qm.queries = []string{}
}

// GetQueries returns a copy of all recorded queries
func (qm *QueryManager) GetQueries() []string {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	result := make([]string, len(qm.queries))
	copy(result, qm.queries)
	return result
}

// SaveToFile saves all recorded queries to a file with semicolon separators
func (qm *QueryManager) SaveToFile(filePath string) error {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	
	// Create directory if it doesn't exist
	dir := filepath.Dir(filePath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	
	content := strings.Join(qm.queries, ";\n")
	if len(qm.queries) > 0 && content != "" {
		content += ";"
	}
	
	return os.WriteFile(filePath, []byte(content), 0644)
}

// AssertGolden asserts the recorded queries against a golden file
func (qm *QueryManager) AssertGolden(t *testing.T) {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	
	content := strings.Join(qm.queries, ";\n")
	if len(qm.queries) > 0 && content != "" {
		content += ";"
	}
	
	// Use only the filename part for golden.Assert since it automatically looks in testdata/
	filename := filepath.Base(qm.goldenFile)
	
	// Check if golden file exists and provide helpful error message (only when not updating)
	if !golden.FlagUpdate() {
		goldenPath := filepath.Join("testdata", filename)
		if _, err := os.Stat(goldenPath); os.IsNotExist(err) {
			t.Fatalf("Golden file '%s' does not exist.\n\nTo create the golden file:\n1. Run the test with -update flag: go test -update\n   OR\n2. Manually create the file with expected SQL queries\n   OR\n3. Use SaveToFile() method to generate the golden file from recorded queries", goldenPath)
		}
	}
	
	golden.Assert(t, content, filename)
}

// AssertGoldenWithComparison asserts the recorded queries against a golden file using comparison normalization
func (qm *QueryManager) AssertGoldenWithComparison(t *testing.T) {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	
	// Normalize queries for comparison
	normalizedQueries := make([]string, len(qm.queries))
	for i, query := range qm.queries {
		normalizedQueries[i] = qm.normalizeForComparison(query)
	}
	
	content := strings.Join(normalizedQueries, ";\n")
	if len(normalizedQueries) > 0 && content != "" {
		content += ";"
	}
	
	// Use only the filename part for golden.Assert since it automatically looks in testdata/
	filename := filepath.Base(qm.goldenFile)
	
	// Check if golden file exists and provide helpful error message (only when not updating)
	if !golden.FlagUpdate() {
		goldenPath := filepath.Join("testdata", filename)
		if _, err := os.Stat(goldenPath); os.IsNotExist(err) {
			t.Fatalf("Golden file '%s' does not exist.\n\nTo create the golden file:\n1. Run the test with -update flag: go test -update\n   OR\n2. Manually create the file with expected SQL queries\n   OR\n3. Use SaveToFile() method to generate the golden file from recorded queries", goldenPath)
		}
	}
	
	golden.Assert(t, content, filename)
}

// CompareQueries compares two SQL queries using normalization for comparison
func (qm *QueryManager) CompareQueries(query1, query2 string) bool {
	normalized1 := qm.normalizeForComparison(query1)
	normalized2 := qm.normalizeForComparison(query2)
	return normalized1 == normalized2
}