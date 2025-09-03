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