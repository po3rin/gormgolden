package gormgoldenv1

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/po3rin/gormgolden/common"
)

var (
	queryManager *common.QueryManager
)

func Register(db *gorm.DB, filePath string) error {
	queryManager = common.NewQueryManager(filePath)

	// Register callbacks for all operations
	db.Callback().Create().After("gorm:create").Register("gormgolden:after_create", afterCallback)
	db.Callback().Query().After("gorm:query").Register("gormgolden:after_query", afterCallback)
	db.Callback().Update().After("gorm:update").Register("gormgolden:after_update", afterCallback)
	db.Callback().Delete().After("gorm:delete").Register("gormgolden:after_delete", afterCallback)
	db.Callback().RowQuery().After("gorm:row_query").Register("gormgolden:after_row_query", afterCallback)

	return nil
}

func afterCallback(scope *gorm.Scope) {
	sql := scope.SQL
	vars := scope.SQLVars

	if sql == "" {
		return
	}

	fullSQL := buildFullSQL(sql, vars)
	queryManager.AddQuery(fullSQL)
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

// Public functions to control recording
func Enable() {
	if queryManager != nil {
		queryManager.Enable()
	}
}

func Disable() {
	if queryManager != nil {
		queryManager.Disable()
	}
}

func Clear() {
	if queryManager != nil {
		queryManager.Clear()
	}
}

func GetQueries() []string {
	if queryManager != nil {
		return queryManager.GetQueries()
	}
	return []string{}
}

func SaveToFile(filePath string) error {
	if queryManager != nil {
		return queryManager.SaveToFile(filePath)
	}
	return nil
}

func AssertGolden(t *testing.T) {
	if queryManager != nil {
		queryManager.AssertGolden(t)
	}
}
