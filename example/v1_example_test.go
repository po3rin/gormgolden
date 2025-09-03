package example

import (
	"testing"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/po3rin/gormgolden/gormgoldenv1"
)

type Product struct {
	ID          uint   `gorm:"primary_key"`
	Name        string `gorm:"not null"`
	Code        string `gorm:"unique_index"`
	Price       float64
	Description string
}

func TestGORMV1SQLCapture(t *testing.T) {
	db, err := gorm.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Register callbacks
	err = gormgoldenv1.Register(db, "testdata/v1_queries.golden.sql")
	if err != nil {
		t.Fatal(err)
	}

	db.AutoMigrate(&Product{})

	gormgoldenv1.Clear()

	product := Product{
		Name:        "Laptop",
		Code:        "LAP001",
		Price:       999.99,
		Description: "High-performance laptop",
	}
	db.Create(&product)

	var products []Product
	db.Where("price > ?", 500).Find(&products)

	db.Model(&product).Update("price", 899.99)

	db.Delete(&product)

	// Verify queries were recorded by callbacks and assert against golden file
	queries := gormgoldenv1.GetQueries()
	if len(queries) != 4 {
		t.Errorf("expected 4 queries, got %d", len(queries))
	}

	// Single line golden assertion
	gormgoldenv1.AssertGolden(t)
}

func TestGORMV1EnableDisable(t *testing.T) {
	db, err := gorm.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	err = gormgoldenv1.Register(db, "")
	if err != nil {
		t.Fatal(err)
	}

	db.AutoMigrate(&Product{})

	gormgoldenv1.Clear()

	gormgoldenv1.Disable()
	product := Product{
		Name:  "Mouse",
		Code:  "MOU001",
		Price: 29.99,
	}
	db.Create(&product)
	
	if len(gormgoldenv1.GetQueries()) != 0 {
		t.Error("expected no queries when disabled")
	}

	gormgoldenv1.Enable()
	db.First(&product, 1)
	
	if len(gormgoldenv1.GetQueries()) != 1 {
		t.Error("expected 1 query when enabled")
	}
}