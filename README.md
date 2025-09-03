# gormgolden

A callback-based GORM plugin for recording SQL queries to files, making it easy to implement golden tests for database operations. Supports both GORM v1 and v2 using pure callback hooks.

Note: Package names are `gormgoldenv1` and `gormgoldenv2` to avoid confusion with GORM version numbers.

## Features

- Records all SQL queries executed through GORM using callbacks
- Saves queries to files for golden testing
- Thread-safe implementation with global state
- Enable/disable recording on demand
- Support for both GORM v1 and v2
- No recorder pattern - uses direct callback registration

## Installation

```bash
go get github.com/po3rin/gormgolden
```

## Usage

### GORM v2

```go
import (
    "github.com/po3rin/gormgolden/gormgoldenv2"
    "gorm.io/gorm"
)

// Initialize and register plugin
plugin := gormgoldenv2.New("testdata/queries.golden.sql")
db.Use(plugin)

// Execute your database operations
db.Create(&user)
db.Where("age > ?", 25).Find(&users)

// Save queries to file
err := gormgoldenv2.SaveToFile("queries.sql")

// Get queries programmatically
queries := gormgoldenv2.GetQueries()

// Clear recorded queries
gormgoldenv2.Clear()

// Disable/Enable recording
gormgoldenv2.Disable()
gormgoldenv2.Enable()
```

### GORM v1

```go
import (
    "github.com/jinzhu/gorm"
    "github.com/po3rin/gormgolden/gormgoldenv1"
)

// Register callbacks
gormgoldenv1.Register(db, "testdata/queries.golden.sql")

// Execute your database operations
db.Create(&product)
db.Where("price > ?", 500).Find(&products)

// Save queries to file
err := gormgoldenv1.SaveToFile("queries.sql")

// Get queries programmatically
queries := gormgoldenv1.GetQueries()

// Clear recorded queries
gormgoldenv1.Clear()

// Disable/Enable recording
gormgoldenv1.Disable()
gormgoldenv1.Enable()
```

## Golden Testing

Use with `gotest.tools/v3/golden` for golden file testing:

```go
import (
    "testing"
    "gotest.tools/v3/golden"
)

func TestDatabaseOperations(t *testing.T) {
    // Setup database and plugin
    plugin := gormgoldenv2.New("testdata/queries.golden.sql")
    db.Use(plugin)
    
    // Execute operations
    performDatabaseOperations(db)
    
    // Assert against golden file (simplified with plugin integration)
    gormgoldenv2.AssertGolden(t)
}
```

### Updating Golden Files

To update golden files when your SQL queries change:

```bash
# Update all golden files
go test -update

# Update specific test
go test -run TestDatabaseOperations -update
```

## API

### GORM v2 Functions

- `gormgoldenv2.New(filePath string) *Plugin` - Create new plugin with golden file path
- `gormgoldenv2.GetQueries() []string` - Get all recorded queries
- `gormgoldenv2.SaveToFile(filePath string) error` - Save queries to file with semicolon separator
- `gormgoldenv2.AssertGolden(t *testing.T)` - Assert queries against golden file
- `gormgoldenv2.Clear()` - Clear all recorded queries
- `gormgoldenv2.Enable()` - Enable query recording
- `gormgoldenv2.Disable()` - Disable query recording

### GORM v1 Functions

- `gormgoldenv1.Register(db *gorm.DB, filePath string) error` - Register callbacks to database
- `gormgoldenv1.GetQueries() []string` - Get all recorded queries
- `gormgoldenv1.SaveToFile(filePath string) error` - Save queries to file with semicolon separator
- `gormgoldenv1.AssertGolden(t *testing.T)` - Assert queries against golden file
- `gormgoldenv1.Clear()` - Clear all recorded queries
- `gormgoldenv1.Enable()` - Enable query recording
- `gormgoldenv1.Disable()` - Disable query recording

## License

MIT