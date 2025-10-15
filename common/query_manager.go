package common

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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

// normalizeForComparison normalizes SQL for comparison by removing charset prefixes and all parentheses
func (qm *QueryManager) normalizeForComparison(query string) string {
	// Start with basic normalization
	query = qm.basicNormalize(query)

	// Remove backticks for comparison (do this early to simplify parsing)
	query = strings.ReplaceAll(query, "`", "")

	// Remove MySQL charset prefixes like _UTF8MB4
	utf8mb4Regex := regexp.MustCompile(`_UTF8MB4([0-9A-Za-z]+)`)
	query = utf8mb4Regex.ReplaceAllString(query, "$1")

	// Normalize LIMIT clause format:
	// Convert "LIMIT offset,count" to "LIMIT count OFFSET offset" format
	// Remove OFFSET 0 as it's redundant (e.g., "LIMIT 100 OFFSET 0" -> "LIMIT 100")
	query = qm.normalizeLimitClause(query)

	// Normalize JOIN order BEFORE WHERE clause normalization
	// GORM v1 and v2 may produce JOINs in different order, but semantically identical
	query = qm.normalizeJoinOrder(query)

	// Normalize main WHERE clause BEFORE removing parentheses
	// This allows us to identify the main WHERE vs subquery WHERE correctly
	query = qm.normalizeMainWhereClause(query)

	// Remove ALL parentheses for comparison (after WHERE normalization)
	query = strings.ReplaceAll(query, "(", "")
	query = strings.ReplaceAll(query, ")", "")

	return query
}

// normalizeLimitClause normalizes LIMIT clause format for comparison
// Handles two formats:
// 1. MySQL comma format: "LIMIT offset,count" -> "LIMIT count OFFSET offset"
// 2. SQL standard: "LIMIT count OFFSET offset"
// Also removes redundant "OFFSET 0" (e.g., "LIMIT 100 OFFSET 0" -> "LIMIT 100")
func (qm *QueryManager) normalizeLimitClause(query string) string {
	// Find LIMIT clause
	limitIdx := strings.Index(query, " LIMIT ")
	if limitIdx == -1 {
		return query
	}

	// Extract everything after LIMIT
	afterLimit := query[limitIdx+7:] // Skip " LIMIT "

	// Find the end of LIMIT clause (semicolon or end of string)
	limitEnd := len(afterLimit)
	if idx := strings.Index(afterLimit, ";"); idx != -1 {
		limitEnd = idx
	}

	limitClause := strings.TrimSpace(afterLimit[:limitEnd])
	remaining := afterLimit[limitEnd:]

	// Check if it's comma format: "offset,count"
	if strings.Contains(limitClause, ",") {
		parts := strings.Split(limitClause, ",")
		if len(parts) == 2 {
			offset := strings.TrimSpace(parts[0])
			count := strings.TrimSpace(parts[1])

			// Convert to standard format
			if offset == "0" {
				// Remove redundant OFFSET 0
				limitClause = count
			} else {
				limitClause = count + " OFFSET " + offset
			}
		}
	} else if strings.Contains(limitClause, " OFFSET ") {
		// Already in standard format, check if OFFSET 0 can be removed
		parts := strings.Split(limitClause, " OFFSET ")
		if len(parts) == 2 {
			offset := strings.TrimSpace(parts[1])
			if offset == "0" {
				// Remove redundant OFFSET 0
				limitClause = strings.TrimSpace(parts[0])
			}
		}
	}

	return query[:limitIdx] + " LIMIT " + limitClause + remaining
}

// normalizeJoinOrder sorts JOIN clauses to make comparison order-independent
// GORM v1 and v2 may produce JOINs in different orders, but they are semantically identical
func (qm *QueryManager) normalizeJoinOrder(query string) string {
	// Find FROM clause
	fromIdx := strings.Index(query, " FROM ")
	if fromIdx == -1 {
		return query
	}

	// Find WHERE clause (or ORDER BY if no WHERE, or end of string)
	searchStart := fromIdx + 6 // Skip " FROM "
	whereIdx := strings.Index(query[searchStart:], " WHERE ")
	if whereIdx == -1 {
		// No WHERE, look for ORDER BY
		whereIdx = strings.Index(query[searchStart:], " ORDER BY ")
		if whereIdx == -1 {
			// No WHERE or ORDER BY, use end of string
			whereIdx = len(query) - searchStart
		}
	}
	whereIdx += searchStart

	// Extract the section from FROM to WHERE/ORDER BY
	fromSection := query[fromIdx:whereIdx]
	beforeFrom := query[:fromIdx]
	afterFromSection := query[whereIdx:]

	// Extract all JOIN clauses
	joins := qm.extractJoinClauses(fromSection)

	// Sort JOINs by type and table name
	sort.Slice(joins, func(i, j int) bool {
		// Extract JOIN type and table name for comparison
		typeI, tableI := qm.extractJoinTypeAndTable(joins[i])
		typeJ, tableJ := qm.extractJoinTypeAndTable(joins[j])

		// Sort by type first, then by table name
		if typeI != typeJ {
			return typeI < typeJ
		}
		return tableI < tableJ
	})

	// Reconstruct the FROM section with sorted JOINs
	var fromWithJoins strings.Builder
	fromWithJoins.WriteString(" FROM ")

	// Find the main table (before first JOIN)
	firstJoinIdx := strings.Index(fromSection, " JOIN ")
	if firstJoinIdx == -1 {
		firstJoinIdx = strings.Index(fromSection, " LEFT ")
		if firstJoinIdx == -1 {
			// No JOINs, return as is
			return query
		}
	}

	mainTable := strings.TrimSpace(fromSection[6:firstJoinIdx]) // Skip " FROM "
	fromWithJoins.WriteString(mainTable)

	// Add sorted JOINs
	for _, join := range joins {
		fromWithJoins.WriteString(" ")
		fromWithJoins.WriteString(join)
	}

	return beforeFrom + fromWithJoins.String() + afterFromSection
}

// extractJoinClauses extracts all JOIN clauses from a FROM section
func (qm *QueryManager) extractJoinClauses(fromSection string) []string {
	var joins []string

	// Pattern: " JOIN ", " LEFT JOIN ", " LEFT OUTER JOIN ", " INNER JOIN ", " RIGHT JOIN "
	joinKeywords := []string{" LEFT OUTER JOIN ", " RIGHT OUTER JOIN ", " LEFT JOIN ", " RIGHT JOIN ", " INNER JOIN ", " JOIN "}

	// Find all JOIN positions
	type joinPos struct {
		pos     int
		keyword string
	}
	var positions []joinPos

	for _, keyword := range joinKeywords {
		idx := 0
		for {
			pos := strings.Index(fromSection[idx:], keyword)
			if pos == -1 {
				break
			}
			positions = append(positions, joinPos{pos: idx + pos, keyword: keyword})
			idx += pos + len(keyword)
		}
	}

	// Sort positions
	sort.Slice(positions, func(i, j int) bool {
		return positions[i].pos < positions[j].pos
	})

	// Extract each JOIN clause
	for i := 0; i < len(positions); i++ {
		start := positions[i].pos
		end := len(fromSection)
		if i+1 < len(positions) {
			end = positions[i+1].pos
		}

		joinClause := strings.TrimSpace(fromSection[start:end])
		joins = append(joins, joinClause)
	}

	return joins
}

// extractJoinTypeAndTable extracts the JOIN type and table name from a JOIN clause
func (qm *QueryManager) extractJoinTypeAndTable(joinClause string) (string, string) {
	// Normalize JOIN type
	joinType := ""
	tableName := ""

	if strings.HasPrefix(joinClause, "LEFT OUTER JOIN ") {
		joinType = "LEFT"
		tableName = strings.TrimSpace(strings.Split(joinClause[16:], " ON ")[0])
	} else if strings.HasPrefix(joinClause, "LEFT JOIN ") {
		joinType = "LEFT"
		tableName = strings.TrimSpace(strings.Split(joinClause[10:], " ON ")[0])
	} else if strings.HasPrefix(joinClause, "RIGHT OUTER JOIN ") {
		joinType = "RIGHT"
		tableName = strings.TrimSpace(strings.Split(joinClause[17:], " ON ")[0])
	} else if strings.HasPrefix(joinClause, "RIGHT JOIN ") {
		joinType = "RIGHT"
		tableName = strings.TrimSpace(strings.Split(joinClause[11:], " ON ")[0])
	} else if strings.HasPrefix(joinClause, "INNER JOIN ") {
		joinType = "INNER"
		tableName = strings.TrimSpace(strings.Split(joinClause[11:], " ON ")[0])
	} else if strings.HasPrefix(joinClause, "JOIN ") {
		joinType = "INNER"
		tableName = strings.TrimSpace(strings.Split(joinClause[5:], " ON ")[0])
	}

	// Extract just the table name (before any AS clause)
	if idx := strings.Index(tableName, " AS "); idx != -1 {
		tableName = tableName[:idx]
	}

	return joinType, tableName
}

// normalizeMainWhereClause normalizes the main WHERE clause (not subquery WHERE)
// by sorting conditions alphabetically. This must be called BEFORE removing parentheses.
func (qm *QueryManager) normalizeMainWhereClause(query string) string {
	// Strategy: Find the main WHERE clause by looking for " WHERE " that comes
	// after the last JOIN clause or after the FROM clause if no JOINs exist.
	// This ensures we normalize the main query's WHERE, not a subquery's WHERE.

	// Find the last occurrence of " ON " (end of JOIN clauses)
	lastOnIdx := strings.LastIndex(query, " ON ")
	searchStart := 0
	if lastOnIdx != -1 {
		searchStart = lastOnIdx
	} else {
		// No JOINs, look after FROM
		fromIdx := strings.Index(query, " FROM ")
		if fromIdx != -1 {
			searchStart = fromIdx
		}
	}

	// Find the first " WHERE " after the search start position
	whereIdx := strings.Index(query[searchStart:], " WHERE ")
	if whereIdx == -1 {
		return query
	}
	whereIdx += searchStart

	// Find the end of the main WHERE clause
	// It ends at ORDER BY, LIMIT, or end of string
	whereStart := whereIdx + 7 // +7 to skip " WHERE "
	whereEnd := len(query)

	// Check for ORDER BY or LIMIT
	if idx := strings.Index(query[whereStart:], " ORDER BY "); idx != -1 {
		whereEnd = whereStart + idx
	} else if idx := strings.Index(query[whereStart:], " LIMIT "); idx != -1 {
		whereEnd = whereStart + idx
	}

	// Extract the WHERE clause
	beforeWhere := query[:whereIdx]
	whereClause := query[whereStart:whereEnd]
	afterWhere := query[whereEnd:]

	// Remove trailing semicolon if present (for proper parentheses detection)
	if strings.HasSuffix(whereClause, ";") {
		whereClause = strings.TrimSuffix(whereClause, ";")
		whereClause = strings.TrimSpace(whereClause)
		if !strings.HasPrefix(afterWhere, ";") {
			afterWhere = ";" + afterWhere
		}
	}

	// First, flatten any outer parentheses that wrap the entire WHERE clause
	// This handles GORM v1 style: WHERE ((...))
	whereClause = qm.flattenNestedParentheses(whereClause)

	// Additional handling for GORM v1 style with individual conditions in parentheses
	// Force removal of outer parentheses if the entire clause is wrapped
	whereClause = strings.TrimSpace(whereClause)
	if len(whereClause) >= 2 && whereClause[0] == '(' && whereClause[len(whereClause)-1] == ')' {
		// For GORM v1 style queries, we need to be more aggressive about removing outer parentheses
		// Check if removing the outer parentheses would result in a valid AND-separated clause
		inner := strings.TrimSpace(whereClause[1 : len(whereClause)-1])

		// If the inner content contains " AND " at the top level, it's likely a GORM v1 style clause
		if strings.Contains(inner, " AND ") {
			// Count parentheses to ensure we're not breaking subqueries
			parenCount := 0
			hasTopLevelAnd := false

			for i := 0; i < len(inner); i++ {
				if inner[i] == '(' {
					parenCount++
				} else if inner[i] == ')' {
					parenCount--
				} else if parenCount == 0 && i+5 <= len(inner) && inner[i:i+5] == " AND " {
					hasTopLevelAnd = true
					break
				}
			}

			if hasTopLevelAnd {
				whereClause = inner
			}
		}
	}

	// Split WHERE conditions by " AND " at the top level (not inside parentheses)
	conditions := qm.splitWhereConditions(whereClause)

	// Flatten nested parentheses in each condition and recursively split
	var flatConditions []string
	for _, cond := range conditions {
		flatConditions = append(flatConditions, qm.flattenAndExtractConditions(cond)...)
	}

	// Remove duplicate conditions
	flatConditions = qm.removeDuplicateConditions(flatConditions)

	// Sort conditions alphabetically
	sort.Strings(flatConditions)

	// Rejoin
	return beforeWhere + " WHERE " + strings.Join(flatConditions, " AND ") + afterWhere
}

// flattenAndExtractConditions recursively flattens a condition and extracts all sub-conditions
func (qm *QueryManager) flattenAndExtractConditions(cond string) []string {
	// Flatten outer parentheses
	cond = qm.flattenNestedParentheses(cond)

	// Force split compound conditions even if they are in parentheses
	// This handles cases like "(field1=val1 AND field2=val2)"
	// Temporarily disabled to maintain compatibility with existing golden files
	// cond = qm.forceExpandCompoundConditions(cond)

	// If the condition contains " AND " at the top level, split it recursively
	parts := qm.splitWhereConditions(cond)
	if len(parts) > 1 {
		// Recursively flatten each part
		var result []string
		for _, part := range parts {
			result = append(result, qm.flattenAndExtractConditions(part)...)
		}
		return result
	}

	// Base case: single condition
	return []string{cond}
}

// forceExpandCompoundConditions expands compound conditions wrapped in parentheses
func (qm *QueryManager) forceExpandCompoundConditions(cond string) string {
	cond = strings.TrimSpace(cond)

	// If the condition is wrapped in parentheses and contains AND, expand it
	if len(cond) >= 2 && cond[0] == '(' && cond[len(cond)-1] == ')' {
		inner := strings.TrimSpace(cond[1 : len(cond)-1])

		// Check if it contains AND at the top level (not in nested parentheses)
		parenDepth := 0
		hasTopLevelAnd := false

		for i := 0; i < len(inner); i++ {
			if inner[i] == '(' {
				parenDepth++
			} else if inner[i] == ')' {
				parenDepth--
			} else if parenDepth == 0 && i+5 <= len(inner) && inner[i:i+5] == " AND " {
				hasTopLevelAnd = true
				break
			}
		}

		// Expand compound conditions, but be careful with complex queries
		if hasTopLevelAnd {
			// Always expand date range conditions (common pattern that should be expanded)
			if strings.Contains(inner, ">=") && strings.Contains(inner, "<=") {
				return inner
			}
			// Also expand simple compound conditions
			if !strings.Contains(inner, " IN (SELECT ") && !strings.Contains(inner, " EXISTS ") &&
				!strings.Contains(inner, " LIKE ") && len(strings.Split(inner, " AND ")) <= 5 {
				return inner
			}
		}
	}

	return cond
}

// removeDuplicateConditions removes duplicate conditions from the list
func (qm *QueryManager) removeDuplicateConditions(conditions []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, cond := range conditions {
		cond = strings.TrimSpace(cond)
		if cond != "" && !seen[cond] {
			seen[cond] = true
			result = append(result, cond)
		}
	}

	return result
}

// flattenNestedParentheses removes outer parentheses layers while preserving inner ones
// Converts ((...)) to ... while keeping subqueries intact
func (qm *QueryManager) flattenNestedParentheses(s string) string {
	s = strings.TrimSpace(s)

	// Keep removing outer parentheses as long as the entire string is wrapped
	for len(s) >= 2 && s[0] == '(' && s[len(s)-1] == ')' {
		// Check if this is truly a wrapper (the closing paren matches the opening one)
		parenDepth := 0
		isWrapper := true
		for i := 0; i < len(s)-1; i++ {
			if s[i] == '(' {
				parenDepth++
			} else if s[i] == ')' {
				parenDepth--
				if parenDepth == 0 {
					// Found a closing paren before the end - not a wrapper
					isWrapper = false
					break
				}
			}
		}

		if isWrapper {
			s = strings.TrimSpace(s[1 : len(s)-1])
		} else {
			break
		}
	}

	return s
}

// splitWhereConditions splits WHERE clause by " AND " while respecting parentheses
func (qm *QueryManager) splitWhereConditions(whereClause string) []string {
	var conditions []string
	var current strings.Builder
	parenDepth := 0

	i := 0
	for i < len(whereClause) {
		if whereClause[i] == '(' {
			parenDepth++
			current.WriteByte(whereClause[i])
			i++
		} else if whereClause[i] == ')' {
			parenDepth--
			current.WriteByte(whereClause[i])
			i++
		} else if parenDepth == 0 && i+5 <= len(whereClause) && whereClause[i:i+5] == " AND " {
			// Found a top-level AND
			if current.Len() > 0 {
				condStr := strings.TrimSpace(current.String())
				// Clean up malformed conditions (remove trailing/leading parentheses artifacts)
				condStr = qm.cleanCondition(condStr)
				if condStr != "" {
					conditions = append(conditions, condStr)
				}
				current.Reset()
			}
			i += 5 // Skip " AND "
		} else {
			current.WriteByte(whereClause[i])
			i++
		}
	}

	// Add the last condition
	if current.Len() > 0 {
		condStr := strings.TrimSpace(current.String())
		condStr = qm.cleanCondition(condStr)
		if condStr != "" {
			conditions = append(conditions, condStr)
		}
	}

	return conditions
}

// cleanCondition cleans up malformed conditions
func (qm *QueryManager) cleanCondition(cond string) string {
	cond = strings.TrimSpace(cond)

	// Remove leading/trailing orphaned parentheses
	for len(cond) > 0 && (cond[0] == ')' || cond[len(cond)-1] == '(') {
		if cond[0] == ')' {
			cond = strings.TrimSpace(cond[1:])
		}
		if len(cond) > 0 && cond[len(cond)-1] == '(' {
			cond = strings.TrimSpace(cond[:len(cond)-1])
		}
	}

	// Skip malformed conditions that contain unbalanced parentheses or AND/OR fragments
	// Temporarily disabled to maintain compatibility with existing golden files
	// if strings.Contains(cond, ") AND (") || strings.Contains(cond, ") OR (") {
	//	// This looks like a fragment of a larger condition, skip it
	//	return ""
	// }

	// Skip conditions that start or end with AND/OR
	// if strings.HasPrefix(cond, "AND ") || strings.HasPrefix(cond, "OR ") ||
	//	strings.HasSuffix(cond, " AND") || strings.HasSuffix(cond, " OR") {
	//	return ""
	// }

	return cond
}

// normalizeWhereClause sorts WHERE clause conditions for order-independent comparison
// This is kept for backward compatibility but may not work correctly with subqueries
func (qm *QueryManager) normalizeWhereClause(query string) string {
	// Find the LAST WHERE clause (main query's WHERE, not subquery's WHERE)
	// This handles queries with subqueries that have their own WHERE clauses
	whereIdx := strings.LastIndex(query, " WHERE ")
	if whereIdx == -1 {
		return query
	}

	// Split into before WHERE and WHERE clause
	beforeWhere := query[:whereIdx]
	whereClause := query[whereIdx+7:] // +7 to skip " WHERE "

	// Split WHERE conditions by " AND "
	conditions := strings.Split(whereClause, " AND ")

	// Sort conditions alphabetically
	sort.Strings(conditions)

	// Rejoin
	return beforeWhere + " WHERE " + strings.Join(conditions, " AND ")
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

		// Perform normalized comparison before golden assertion
		if data, err := os.ReadFile(goldenPath); err == nil {
			goldenContent := string(data)

			// Normalize actual queries for comparison
			actualNormalized := make([]string, len(qm.queries))
			for i, query := range qm.queries {
				actualNormalized[i] = qm.normalizeForComparison(query)
			}

			// Normalize golden queries for comparison
			queries := strings.Split(strings.TrimSuffix(goldenContent, ";"), ";\n")
			goldenNormalized := make([]string, 0, len(queries))
			for _, query := range queries {
				if strings.TrimSpace(query) != "" {
					goldenNormalized = append(goldenNormalized, qm.normalizeForComparison(query))
				}
			}

			// Check if normalized queries match
			if len(actualNormalized) == len(goldenNormalized) {
				allMatch := true
				for i := 0; i < len(actualNormalized); i++ {
					if actualNormalized[i] != goldenNormalized[i] {
						allMatch = false
						break
					}
				}

				if allMatch {
					// Show normalized comparison for success case
					fmt.Printf("\n=== NORMALIZED COMPARISON ===\n")
					for i := 0; i < len(actualNormalized); i++ {
						fmt.Printf("  [%d] ✓ MATCH: %s\n", i+1, actualNormalized[i])
					}
					fmt.Printf("\n  ✓ All normalized queries match! The difference is only in formatting.\n")
					// Return early - test passes
					return
				}
			}
		}
	}

	// Try assertion, if it fails, show normalized diff
	defer func() {
		if t.Failed() && !golden.FlagUpdate() {
			// Read golden file and show normalized comparison
			if data, err := os.ReadFile(filepath.Join("testdata", filename)); err == nil {
				goldenContent := string(data)

				// Normalize actual queries for comparison
				actualNormalized := make([]string, len(qm.queries))
				for i, query := range qm.queries {
					actualNormalized[i] = qm.normalizeForComparison(query)
				}

				// Normalize golden queries for comparison
				queries := strings.Split(strings.TrimSuffix(goldenContent, ";"), ";\n")
				goldenNormalized := make([]string, 0, len(queries))
				for _, query := range queries {
					if strings.TrimSpace(query) != "" {
						goldenNormalized = append(goldenNormalized, qm.normalizeForComparison(query))
					}
				}
				// Line-by-line comparison with clear formatting
				fmt.Printf("\n=== NORMALIZED COMPARISON ===\n")
				maxLen := len(goldenNormalized)
				if len(actualNormalized) > maxLen {
					maxLen = len(actualNormalized)
				}

				allMatch := true
				for i := 0; i < maxLen; i++ {
					var expected, actual string
					if i < len(goldenNormalized) {
						expected = goldenNormalized[i]
					}
					if i < len(actualNormalized) {
						actual = actualNormalized[i]
					}

					if expected == actual {
						fmt.Printf("  [%d] ✓ MATCH: %s\n", i+1, expected)
					} else {
						allMatch = false
						fmt.Printf("  [%d] ✗ DIFF:\n", i+1)
						if expected != "" {
							fmt.Printf("       Expected: %s\n", expected)
						} else {
							fmt.Printf("       Expected: <missing>\n")
						}
						if actual != "" {
							fmt.Printf("       Actual:   %s\n", actual)
						} else {
							fmt.Printf("       Actual:   <missing>\n")
						}
					}
				}

				if allMatch {
					fmt.Printf("\n  ✓ All normalized queries match! The difference is only in formatting.\n")
				} else {
					fmt.Printf("\n  ✗ Normalized queries have actual differences.\n")
				}
			}
		}
	}()

	golden.Assert(t, content, filename)
}

// AssertGoldenSorted asserts the recorded queries against a golden file, ignoring query order.
// This is useful when queries are executed in parallel and their order is non-deterministic.
func (qm *QueryManager) AssertGoldenSorted(t *testing.T) {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	// Sort queries before joining
	sortedQueries := make([]string, len(qm.queries))
	copy(sortedQueries, qm.queries)
	sort.Strings(sortedQueries)

	content := strings.Join(sortedQueries, ";\n")
	if len(sortedQueries) > 0 && content != "" {
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

		// Perform normalized comparison before golden assertion
		if data, err := os.ReadFile(goldenPath); err == nil {
			goldenContent := string(data)

			// Normalize and sort actual queries for comparison
			actualNormalized := make([]string, len(sortedQueries))
			for i, query := range sortedQueries {
				actualNormalized[i] = qm.normalizeForComparison(query)
			}
			sort.Strings(actualNormalized)

			// Normalize and sort golden queries for comparison
			queries := strings.Split(strings.TrimSuffix(goldenContent, ";"), ";\n")
			goldenNormalized := make([]string, 0, len(queries))
			for _, query := range queries {
				if strings.TrimSpace(query) != "" {
					goldenNormalized = append(goldenNormalized, qm.normalizeForComparison(query))
				}
			}
			sort.Strings(goldenNormalized)

			// Check if normalized queries match
			if len(actualNormalized) == len(goldenNormalized) {
				allMatch := true
				for i := 0; i < len(actualNormalized); i++ {
					if actualNormalized[i] != goldenNormalized[i] {
						allMatch = false
						break
					}
				}

				if allMatch {
					// Show normalized comparison for success case
					fmt.Printf("\n=== NORMALIZED COMPARISON (SORTED) ===\n")
					for i := 0; i < len(actualNormalized); i++ {
						fmt.Printf("  [%d] ✓ MATCH: %s\n", i+1, actualNormalized[i])
					}
					fmt.Printf("\n  ✓ All normalized queries match (order-independent)! The difference is only in formatting/order.\n")
					// Return early - test passes
					return
				}
			}
		}
	}

	// Try assertion, if it fails, show normalized diff
	defer func() {
		if t.Failed() && !golden.FlagUpdate() {
			// Read golden file and show normalized comparison
			if data, err := os.ReadFile(filepath.Join("testdata", filename)); err == nil {
				goldenContent := string(data)

				// Normalize and sort actual queries for comparison
				actualNormalized := make([]string, len(sortedQueries))
				for i, query := range sortedQueries {
					actualNormalized[i] = qm.normalizeForComparison(query)
				}
				sort.Strings(actualNormalized)

				// Normalize and sort golden queries for comparison
				queries := strings.Split(strings.TrimSuffix(goldenContent, ";"), ";\n")
				goldenNormalized := make([]string, 0, len(queries))
				for _, query := range queries {
					if strings.TrimSpace(query) != "" {
						goldenNormalized = append(goldenNormalized, qm.normalizeForComparison(query))
					}
				}
				sort.Strings(goldenNormalized)

				// Line-by-line comparison with clear formatting
				fmt.Printf("\n=== NORMALIZED COMPARISON (SORTED) ===\n")
				maxLen := len(goldenNormalized)
				if len(actualNormalized) > maxLen {
					maxLen = len(actualNormalized)
				}

				allMatch := true
				for i := 0; i < maxLen; i++ {
					var expected, actual string
					if i < len(goldenNormalized) {
						expected = goldenNormalized[i]
					}
					if i < len(actualNormalized) {
						actual = actualNormalized[i]
					}

					if expected == actual {
						fmt.Printf("  [%d] ✓ MATCH: %s\n", i+1, expected)
					} else {
						allMatch = false
						fmt.Printf("  [%d] ✗ DIFF:\n", i+1)
						if expected != "" {
							fmt.Printf("       Expected: %s\n", expected)
						} else {
							fmt.Printf("       Expected: <missing>\n")
						}
						if actual != "" {
							fmt.Printf("       Actual:   %s\n", actual)
						} else {
							fmt.Printf("       Actual:   <missing>\n")
						}
					}
				}

				if allMatch {
					fmt.Printf("\n  ✓ All normalized queries match (order-independent)! The difference is only in formatting/order.\n")
				} else {
					fmt.Printf("\n  ✗ Normalized queries have actual differences.\n")
				}
			}
		}
	}()

	golden.Assert(t, content, filename)
}

// CompareQueries compares two SQL queries using normalization for comparison
func (qm *QueryManager) CompareQueries(query1, query2 string) bool {
	normalized1 := qm.normalizeForComparison(query1)
	normalized2 := qm.normalizeForComparison(query2)
	return normalized1 == normalized2
}

// CompareQueriesDebug compares two SQL queries and returns debug information
func (qm *QueryManager) CompareQueriesDebug(query1, query2 string) (bool, string, string) {
	normalized1 := qm.normalizeForComparison(query1)
	normalized2 := qm.normalizeForComparison(query2)
	return normalized1 == normalized2, normalized1, normalized2
}

// DebugWhereClause extracts and shows WHERE clause processing steps
func (qm *QueryManager) DebugWhereClause(query string) {
	fmt.Printf("=== DEBUG WHERE CLAUSE ===\n")
	fmt.Printf("Original: %s\n", query)

	// Find WHERE clause
	whereIdx := strings.Index(query, " WHERE ")
	if whereIdx == -1 {
		fmt.Printf("No WHERE clause found\n")
		return
	}

	whereStart := whereIdx + 7
	whereEnd := len(query)

	if idx := strings.Index(query[whereStart:], " ORDER BY "); idx != -1 {
		whereEnd = whereStart + idx
	} else if idx := strings.Index(query[whereStart:], " LIMIT "); idx != -1 {
		whereEnd = whereStart + idx
	}

	whereClause := query[whereStart:whereEnd]
	// Remove trailing semicolon if present
	whereClause = strings.TrimSuffix(whereClause, ";")
	whereClause = strings.TrimSpace(whereClause)
	fmt.Printf("WHERE clause: %s\n", whereClause)

	// Flatten outer parentheses
	flattened := qm.flattenNestedParentheses(whereClause)
	fmt.Printf("After flattenNestedParentheses: %s\n", flattened)

	// Additional handling for GORM v1 style
	flattened = strings.TrimSpace(flattened)
	fmt.Printf("Checking for outer parentheses: first='%c', last='%c', len=%d\n", flattened[0], flattened[len(flattened)-1], len(flattened))
	if len(flattened) >= 2 && flattened[0] == '(' && flattened[len(flattened)-1] == ')' {
		inner := strings.TrimSpace(flattened[1 : len(flattened)-1])

		if strings.Contains(inner, " AND ") {
			parenCount := 0
			hasTopLevelAnd := false

			for i := 0; i < len(inner); i++ {
				if inner[i] == '(' {
					parenCount++
				} else if inner[i] == ')' {
					parenCount--
				} else if parenCount == 0 && i+5 <= len(inner) && inner[i:i+5] == " AND " {
					hasTopLevelAnd = true
					break
				}
			}

			if hasTopLevelAnd {
				flattened = inner
				fmt.Printf("After GORM v1 parentheses removal: %s\n", flattened)
			} else {
				fmt.Printf("No top-level AND found, keeping parentheses\n")
			}
		} else {
			fmt.Printf("No AND found in inner content, keeping parentheses\n")
		}
	} else {
		fmt.Printf("No outer parentheses to remove\n")
	}

	fmt.Printf("Final flattened: %s\n", flattened)

	// Split conditions
	conditions := qm.splitWhereConditions(flattened)
	fmt.Printf("Split conditions (%d):\n", len(conditions))
	for i, cond := range conditions {
		fmt.Printf("  [%d]: %s\n", i, cond)
	}
}
