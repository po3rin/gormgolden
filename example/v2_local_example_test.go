package example

import (
	"testing"

	"github.com/po3rin/gormgolden/gormgoldenv2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestGORMV2LocalManagement(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	// Create plugin with local management
	plugin := gormgoldenv2.New("testdata/v2_local_queries.golden.sql")
	err = db.Use(plugin)
	if err != nil {
		t.Fatal(err)
	}

	// Create table
	err = db.AutoMigrate(&User{})
	if err != nil {
		t.Fatal(err)
	}

	// Use local methods on plugin instance
	plugin.Clear() // Clear migration queries

	// Perform operations
	user := User{Name: "Alice", Email: "alice@example.com", Age: 28}
	db.Create(&user)

	var users []User
	db.Where("age > ?", 25).Find(&users)

	// Test local disable/enable
	plugin.Disable()
	db.Model(&user).Update("age", 29) // This won't be recorded
	
	plugin.Enable()
	db.Delete(&user) // This will be recorded

	// Verify queries using local method
	queries := plugin.GetQueries()
	if len(queries) != 3 { // CREATE, SELECT, DELETE (UPDATE was disabled)
		t.Errorf("expected 3 queries, got %d", len(queries))
	}

	// Assert using local method
	plugin.AssertGolden(t)
}

func TestMultiplePluginInstances(t *testing.T) {
	// Create first database with its own plugin
	db1, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	
	plugin1 := gormgoldenv2.New("testdata/v2_multi_db1.golden.sql")
	err = db1.Use(plugin1)
	if err != nil {
		t.Fatal(err)
	}

	// Create second database with its own plugin
	db2, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	
	plugin2 := gormgoldenv2.New("testdata/v2_multi_db2.golden.sql")
	err = db2.Use(plugin2)
	if err != nil {
		t.Fatal(err)
	}

	// Setup tables
	db1.AutoMigrate(&User{})
	db2.AutoMigrate(&User{})
	
	// Clear migration queries
	plugin1.Clear()
	plugin2.Clear()

	// Perform operations on db1
	user1 := User{Name: "Bob", Email: "bob@example.com", Age: 35}
	db1.Create(&user1)

	// Perform operations on db2
	user2 := User{Name: "Charlie", Email: "charlie@example.com", Age: 40}
	db2.Create(&user2)

	// Verify each plugin tracked its own queries
	queries1 := plugin1.GetQueries()
	queries2 := plugin2.GetQueries()

	if len(queries1) != 1 {
		t.Errorf("plugin1: expected 1 query, got %d", len(queries1))
	}
	if len(queries2) != 1 {
		t.Errorf("plugin2: expected 1 query, got %d", len(queries2))
	}

	// Each plugin can assert its own golden file
	plugin1.AssertGolden(t)
	plugin2.AssertGolden(t)
}