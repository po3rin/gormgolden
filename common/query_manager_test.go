package common

import (
	"testing"
)

func TestQueryManager_normalize(t *testing.T) {
	qm := NewQueryManager("")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Basic SELECT normalization",
			input:    "select * from users where id = ?",
			expected: "SELECT * FROM `users` WHERE `id`=?",
		},
		{
			name:     "Remove comments",
			input:    "SELECT * FROM users /* comment */ WHERE id = ?",
			expected: "SELECT * FROM `users` WHERE `id`=?",
		},
		{
			name:     "GORM v1 unnecessary parentheses",
			input:    "SELECT * FROM users WHERE ( id > ? )",
			expected: "SELECT * FROM `users` WHERE (`id`>?)",
		},
		{
			name:     "Multiple whitespace normalization",
			input:    "SELECT  *   FROM\n\tusers\t\tWHERE    id = ?",
			expected: "SELECT * FROM `users` WHERE `id`=?",
		},
		{
			name:     "Empty query",
			input:    "",
			expected: "",
		},
		{
			name:     "Whitespace only",
			input:    "   \n\t   ",
			expected: "",
		},
		{
			name:     "Complex query with JOIN",
			input:    "SELECT u.name, p.title FROM users u JOIN posts p ON u.id = p.user_id WHERE u.age > ?",
			expected: "SELECT `u`.`name`,`p`.`title` FROM `users` AS `u` JOIN `posts` AS `p` ON `u`.`id`=`p`.`user_id` WHERE `u`.`age`>?",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := qm.normalize(tt.input)
			if result != tt.expected {
				t.Errorf("normalize() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestQueryManager_basicNormalize(t *testing.T) {
	qm := NewQueryManager("")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Remove extra whitespace",
			input:    "SELECT  *   FROM\n\tusers\t\tWHERE    id = ?",
			expected: "SELECT * FROM users WHERE id = ?",
		},
		{
			name:     "GORM v1 parentheses removal",
			input:    "SELECT * FROM users WHERE ( id > ? ) AND ( name = ? )",
			expected: "SELECT * FROM users WHERE (id > ?) AND (name = ?)",
		},
		{
			name:     "Empty query",
			input:    "",
			expected: "",
		},
		{
			name:     "Only whitespace",
			input:    "   \n\t   ",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := qm.basicNormalize(tt.input)
			if result != tt.expected {
				t.Errorf("basicNormalize() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestQueryManager_AddQueryWithNormalization(t *testing.T) {
	qm := NewQueryManager("")

	// Test that AddQuery normalizes queries
	qm.AddQuery("select * from users where id = ?")
	qm.AddQuery("SELECT  *   FROM\n\tusers\t\tWHERE    name = ?")

	queries := qm.GetQueries()
	if len(queries) != 2 {
		t.Errorf("Expected 2 queries, got %d", len(queries))
	}

	// Both should be normalized
	expected1 := "SELECT * FROM `users` WHERE `id`=?"
	expected2 := "SELECT * FROM `users` WHERE `name`=?"

	if queries[0] != expected1 {
		t.Errorf("First query = %q, want %q", queries[0], expected1)
	}

	if queries[1] != expected2 {
		t.Errorf("Second query = %q, want %q", queries[1], expected2)
	}
}

func TestQueryManager_NormalizationDisabled(t *testing.T) {
	qm := NewQueryManager("")

	// Disable query manager
	qm.Disable()
	qm.AddQuery("SELECT * FROM users")

	queries := qm.GetQueries()
	if len(queries) != 0 {
		t.Errorf("Expected 0 queries when disabled, got %d", len(queries))
	}

	// Enable and add query
	qm.Enable()
	qm.AddQuery("SELECT * FROM users")

	queries = qm.GetQueries()
	if len(queries) != 1 {
		t.Errorf("Expected 1 query when enabled, got %d", len(queries))
	}
}

func TestQueryManager_normalizeForComparison(t *testing.T) {
	qm := NewQueryManager("test.golden.sql")
	
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "remove UTF8MB4 prefix",
			input:    "SELECT * FROM `user_settings` WHERE `user_settings`.`org_id`=_UTF8MB4ABC123DEF456GHI789JKL012",
			expected: "SELECT * FROM `user_settings` WHERE `user_settings`.`org_id`=ABC123DEF456GHI789JKL012",
		},
		{
			name:     "remove parentheses around simple conditions",
			input:    "SELECT * FROM users WHERE (`id`=1) AND (`name`='test')",
			expected: "SELECT * FROM users WHERE `id`=1 AND `name`='test'",
		},
		{
			name:     "remove all parentheses including IN clauses",
			input:    "SELECT * FROM users WHERE id IN (1, 2, 3)",
			expected: "SELECT * FROM users WHERE id IN 1, 2, 3",
		},
		{
			name:     "complex query with UTF8MB4 and parentheses",
			input:    "SELECT * FROM `user_settings` WHERE (`user_settings`.`org_id`=_UTF8MB4ABC123DEF456GHI789JKL012) AND (`user_settings`.`config_id` IN (_UTF8MB4XYZ789ABC123DEF456GHI))",
			expected: "SELECT * FROM `user_settings` WHERE `user_settings`.`org_id`=ABC123DEF456GHI789JKL012 AND `user_settings`.`config_id` IN XYZ789ABC123DEF456GHI",
		},
		{
			name:     "multiple UTF8MB4 occurrences",
			input:    "WHERE org_id=_UTF8MB4123ABC AND user_id=_UTF8MB4456DEF",
			expected: "WHERE org_id=123ABC AND user_id=456DEF",
		},
		{
			name:     "remove complex conditions parentheses",
			input:    "WHERE (status='active' AND created_at > '2023-01-01' OR status='pending')",
			expected: "WHERE status='active' AND created_at > '2023-01-01' OR status='pending'",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := qm.normalizeForComparison(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeForComparison() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestQueryManager_CompareQueries(t *testing.T) {
	qm := NewQueryManager("test.golden.sql")
	
	tests := []struct {
		name     string
		query1   string
		query2   string
		expected bool
	}{
		{
			name:     "identical queries should match",
			query1:   "SELECT * FROM users WHERE id=1",
			query2:   "SELECT * FROM users WHERE id=1",
			expected: true,
		},
		{
			name:     "queries with and without UTF8MB4 should match",
			query1:   "SELECT * FROM users WHERE org_id=_UTF8MB4ABC123DEF456GHI789JKL012",
			query2:   "SELECT * FROM users WHERE org_id=ABC123DEF456GHI789JKL012",
			expected: true,
		},
		{
			name:     "queries with different parentheses should match",
			query1:   "SELECT * FROM users WHERE (`id`=1) AND (`name`='test')",
			query2:   "SELECT * FROM users WHERE `id`=1 AND `name`='test'",
			expected: true,
		},
		{
			name:     "different queries should not match",
			query1:   "SELECT * FROM users WHERE id=1",
			query2:   "SELECT * FROM users WHERE id=2",
			expected: false,
		},
		{
			name:     "complex real-world example should match",
			query1:   "SELECT * FROM `user_settings` WHERE (`user_settings`.`org_id`=_UTF8MB4ABC123DEF456GHI789JKL012) AND (`user_settings`.`config_id` IN (_UTF8MB4XYZ789ABC123DEF456GHI))",
			query2:   "SELECT * FROM `user_settings` WHERE `user_settings`.`org_id`=ABC123DEF456GHI789JKL012 AND `user_settings`.`config_id` IN XYZ789ABC123DEF456GHI",
			expected: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := qm.CompareQueries(tt.query1, tt.query2)
			if result != tt.expected {
				t.Errorf("CompareQueries() = %v, want %v", result, tt.expected)
				t.Logf("Query1 normalized: %q", qm.normalizeForComparison(tt.query1))
				t.Logf("Query2 normalized: %q", qm.normalizeForComparison(tt.query2))
			}
		})
	}
}