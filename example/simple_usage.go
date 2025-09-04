package example

import (
	"fmt"
	"log"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/po3rin/gormgolden/gormgoldenv1"
	"github.com/po3rin/gormgolden/gormgoldenv2"
	"gorm.io/driver/sqlite"
	gorm2 "gorm.io/gorm"
)

func ExampleGORMV2Usage() {
	// Setup database
	db, err := gorm2.Open(sqlite.Open("test.db"), &gorm2.Config{})
	if err != nil {
		log.Fatal(err)
	}

	// Initialize plugin
	plugin := gormgoldenv2.New("golden_files/user_operations.golden.sql")
	
	// Apply plugin to database
	err = db.Use(plugin)
	if err != nil {
		log.Fatal(err)
	}

	// Create table
	type User struct {
		ID    uint   `gorm:"primaryKey"`
		Name  string
		Email string
		Age   int
	}
	db.AutoMigrate(&User{})

	// Clear queries from migration using local method
	plugin.Clear()

	// Perform database operations
	user := User{Name: "Alice", Email: "alice@example.com", Age: 28}
	db.Create(&user)

	var users []User
	db.Where("age > ?", 25).Find(&users)

	db.Model(&user).Update("age", 29)

	// Save SQL queries to golden file using local method
	err = plugin.SaveToFile("user_operations.golden.sql")
	if err != nil {
		log.Fatal(err)
	}

	// Get queries programmatically using local method
	queries := plugin.GetQueries()
	for i, query := range queries {
		fmt.Printf("Query %d: %s\n", i+1, query)
	}

	// Disable recording temporarily using local method
	plugin.Disable()
	db.Delete(&user) // This query won't be recorded

	// Re-enable recording using local method
	plugin.Enable()
	db.First(&user, 1) // This query will be recorded
}

func ExampleGORMV1Usage() {
	// Setup database (GORM v1)
	db, err := gorm.Open("sqlite3", "test_v1.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Register callbacks
	err = gormgoldenv1.Register(db, "golden_files/product_operations.golden.sql")
	if err != nil {
		log.Fatal(err)
	}

	// Create table
	type Product struct {
		ID          uint `gorm:"primary_key"`
		Name        string
		Code        string
		Price       float64
	}
	db.AutoMigrate(&Product{})

	// Clear queries from migration
	gormgoldenv1.Clear()

	// Perform database operations
	product := Product{Name: "Laptop", Code: "LAP001", Price: 999.99}
	db.Create(&product)

	var products []Product
	db.Where("price > ?", 500).Find(&products)

	db.Model(&product).Update("price", 899.99)

	// Save SQL queries to golden file
	err = gormgoldenv1.SaveToFile("product_operations.golden.sql")
	if err != nil {
		log.Fatal(err)
	}

	// Get queries programmatically
	queries := gormgoldenv1.GetQueries()
	for i, query := range queries {
		fmt.Printf("Query %d: %s\n", i+1, query)
	}

	// Disable recording temporarily
	gormgoldenv1.Disable()
	db.Delete(&product) // This query won't be recorded

	// Re-enable recording
	gormgoldenv1.Enable()
	db.First(&product, 1) // This query will be recorded
}