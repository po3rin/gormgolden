package example

import (
	"testing"

	"github.com/po3rin/gormgolden/gormgoldenv2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type User struct {
	ID    uint   `gorm:"primaryKey"`
	Name  string `gorm:"not null"`
	Email string `gorm:"uniqueIndex"`
	Age   int
}

func TestGORMV2SQLCapture(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	// Register the plugin
	plugin := gormgoldenv2.New("testdata/v2_queries.golden.sql")
	err = db.Use(plugin)
	if err != nil {
		t.Fatal(err)
	}

	// Create table
	err = db.AutoMigrate(&User{})
	if err != nil {
		t.Fatal(err)
	}

	// Clear migration queries
	gormgoldenv2.Clear()

	// Perform operations
	user := User{Name: "John Doe", Email: "john@example.com", Age: 30}
	db.Create(&user)

	var users []User
	db.Where("age > ?", 25).Find(&users)

	db.Model(&user).Update("age", 31)

	db.Delete(&user)

	// Verify queries were recorded by callbacks and assert against golden file
	queries := gormgoldenv2.GetQueries()
	if len(queries) != 4 {
		t.Errorf("expected 4 queries, got %d", len(queries))
	}

	// Single line golden assertion
	gormgoldenv2.AssertGolden(t)
}

func TestGORMV2EnableDisable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}

	plugin := gormgoldenv2.New("")
	err = db.Use(plugin)
	if err != nil {
		t.Fatal(err)
	}

	err = db.AutoMigrate(&User{})
	if err != nil {
		t.Fatal(err)
	}

	gormgoldenv2.Clear()

	// Test disable
	gormgoldenv2.Disable()
	user := User{Name: "Jane Doe", Email: "jane@example.com", Age: 25}
	db.Create(&user)
	
	if len(gormgoldenv2.GetQueries()) != 0 {
		t.Error("expected no queries when disabled")
	}

	// Test enable
	gormgoldenv2.Enable()
	db.First(&user, 1)
	
	if len(gormgoldenv2.GetQueries()) != 1 {
		t.Error("expected 1 query when enabled")
	}
}