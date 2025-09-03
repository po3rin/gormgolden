package common

import (
	"os"
	"path/filepath"
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
	golden.Assert(t, content, filename)
}