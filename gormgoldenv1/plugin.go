package gormgoldenv1

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/po3rin/gormgolden/common"
)

var (
	queryManagers  = &sync.Map{} // map[*gorm.DB]*common.QueryManager
	filePathToQM   = &sync.Map{} // map[string]*common.QueryManager (filePath -> queryManager)
	dbToFilePath   = &sync.Map{} // map[*gorm.DB]string (db -> filePath)
	currentFilePath string        // For backward compatibility with functions that don't take filePath
	currentMutex   sync.RWMutex
)

func Register(db *gorm.DB, filePath string) error {
	queryManager := common.NewQueryManager(filePath)
	queryManagers.Store(db, queryManager)
	filePathToQM.Store(filePath, queryManager)
	dbToFilePath.Store(db, filePath)

	// Update current filePath for backward compatibility
	currentMutex.Lock()
	currentFilePath = filePath
	currentMutex.Unlock()

	// Create a closure that captures the queryManager
	afterCallbackFunc := func(scope *gorm.Scope) {
		sql := scope.SQL
		vars := scope.SQLVars

		if sql == "" {
			return
		}

		fullSQL := buildFullSQL(sql, vars)
		queryManager.AddQuery(fullSQL)
	}

	// Register callbacks for all operations
	db.Callback().Create().After("gorm:create").Register("gormgolden:after_create", afterCallbackFunc)
	db.Callback().Query().After("gorm:query").Register("gormgolden:after_query", afterCallbackFunc)
	db.Callback().Update().After("gorm:update").Register("gormgolden:after_update", afterCallbackFunc)
	db.Callback().Delete().After("gorm:delete").Register("gormgolden:after_delete", afterCallbackFunc)
	db.Callback().RowQuery().After("gorm:row_query").Register("gormgolden:after_row_query", afterCallbackFunc)

	return nil
}

func buildFullSQL(sql string, vars []interface{}) string {
	if len(vars) == 0 {
		return sql
	}

	// Simple placeholder replacement for GORM v1
	// Replace ? placeholders with formatted values
	result := sql
	for _, v := range vars {
		value := formatValue(v)
		idx := strings.Index(result, "?")
		if idx != -1 {
			result = result[:idx] + value + result[idx+1:]
		}
	}

	return result
}

func formatValue(v interface{}) string {
	if v == nil {
		return "NULL"
	}

	// Handle pointers
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return "NULL"
		}
		v = rv.Elem().Interface()
	}

	switch val := v.(type) {
	case string:
		return fmt.Sprintf("'%s'", strings.ReplaceAll(val, "'", "''"))
	case time.Time:
		return fmt.Sprintf("'%s'", val.Format("2006-01-02 15:04:05"))
	case []byte:
		return fmt.Sprintf("'%s'", strings.ReplaceAll(string(val), "'", "''"))
	case bool:
		if val {
			return "TRUE"
		}
		return "FALSE"
	default:
		return fmt.Sprintf("%v", v)
	}
}

// getQueryManagerByFilePath returns the queryManager for a given filePath
func getQueryManagerByFilePath(filePath string) *common.QueryManager {
	if qm, ok := filePathToQM.Load(filePath); ok {
		if queryManager, ok := qm.(*common.QueryManager); ok {
			return queryManager
		}
	}
	return nil
}

// getQueryManagerByDB returns the queryManager for a given DB
func getQueryManagerByDB(db *gorm.DB) *common.QueryManager {
	if qm, ok := queryManagers.Load(db); ok {
		if queryManager, ok := qm.(*common.QueryManager); ok {
			return queryManager
		}
	}
	return nil
}

// getCurrentQueryManager returns the current queryManager (for backward compatibility)
func getCurrentQueryManager() *common.QueryManager {
	currentMutex.RLock()
	fp := currentFilePath
	currentMutex.RUnlock()
	return getQueryManagerByFilePath(fp)
}

// Public functions to control recording
func Enable() {
	if qm := getCurrentQueryManager(); qm != nil {
		qm.Enable()
	}
}

func Disable() {
	if qm := getCurrentQueryManager(); qm != nil {
		qm.Disable()
	}
}

func Clear() {
	if qm := getCurrentQueryManager(); qm != nil {
		qm.Clear()
	}
}

// ClearDB clears queries for a specific DB instance (thread-safe for parallel tests)
func ClearDB(db *gorm.DB) {
	if qm := getQueryManagerByDB(db); qm != nil {
		qm.Clear()
	}
}

func GetQueries() []string {
	if qm := getCurrentQueryManager(); qm != nil {
		return qm.GetQueries()
	}
	return []string{}
}

func SaveToFile(filePath string) error {
	if qm := getCurrentQueryManager(); qm != nil {
		return qm.SaveToFile(filePath)
	}
	return nil
}

func AssertGolden(t *testing.T) {
	if qm := getCurrentQueryManager(); qm != nil {
		qm.AssertGolden(t)
	}
}

// AssertGoldenDB asserts golden file for a specific DB instance (thread-safe for parallel tests)
func AssertGoldenDB(t *testing.T, db *gorm.DB) {
	if qm := getQueryManagerByDB(db); qm != nil {
		qm.AssertGolden(t)
	}
}
