# gormgolden

A callback-based GORM plugin for recording SQL queries to files, making it easy to implement golden tests for database operations. Supports both GORM v1 and v2 using pure callback hooks.

Note: Package names are `gormgoldenv1` and `gormgoldenv2` to avoid confusion with GORM version numbers.

## Features

- Records all SQL queries executed through GORM using callbacks
- Saves queries to files for golden testing
- Thread-safe implementation with local state management
- Enable/disable recording on demand
- Support for both GORM v1 and v2
- No recorder pattern - uses direct callback registration
- Support for multiple independent plugin instances (GORM v2)

## Installation

```bash
go get github.com/po3rin/gormgolden
```

## Usage

### Golden Testing

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
    
    // Assert against golden file using plugin method
    plugin.AssertGolden(t)
}
```

#### Updating Golden Files

To update golden files when your SQL queries change:

```bash
# Update all golden files
go test -update

# Update specific test
go test -run TestDatabaseOperations -update
```


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

// Use plugin methods for local management
err := plugin.SaveToFile("queries.sql")

// Get queries programmatically
queries := plugin.GetQueries()

// Clear recorded queries
plugin.Clear()

// Disable/Enable recording
plugin.Disable()
plugin.Enable()
```

#### Multiple Plugin Instances

```go
// Create independent plugins for different databases
plugin1 := gormgoldenv2.New("db1_queries.golden.sql")
plugin2 := gormgoldenv2.New("db2_queries.golden.sql")

db1.Use(plugin1)
db2.Use(plugin2)

// Each plugin tracks its own queries independently
plugin1.Clear()
plugin2.Clear()

// Assert against different golden files
plugin1.AssertGolden(t)
plugin2.AssertGolden(t)
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

## API

### GORM v2 Plugin Methods

- `gormgoldenv2.New(filePath string) *Plugin` - Create new plugin with golden file path
- `plugin.GetQueries() []string` - Get all recorded queries
- `plugin.SaveToFile(filePath string) error` - Save queries to file with semicolon separator
- `plugin.AssertGolden(t *testing.T)` - Assert queries against golden file
- `plugin.Clear()` - Clear all recorded queries
- `plugin.Enable()` - Enable query recording
- `plugin.Disable()` - Disable query recording

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