package gormgoldenv2

import (
	"testing"

	"github.com/po3rin/gormgolden/common"
	"gorm.io/gorm"
)

var (
	queryManager *common.QueryManager
)

type Plugin struct {
	GoldenFile   string
}

func New(filePath string) *Plugin {
	return &Plugin{
		GoldenFile: filePath,
	}
}

func (p *Plugin) Name() string {
	return "gormgolden"
}

func (p *Plugin) Initialize(db *gorm.DB) error {
	queryManager = common.NewQueryManager(p.GoldenFile)
	
	// Register callbacks for all operations
	callback := db.Callback()
	
	callback.Query().After("gorm:query").Register("gormgolden:after_query", afterCallback)
	callback.Create().After("gorm:create").Register("gormgolden:after_create", afterCallback)
	callback.Update().After("gorm:update").Register("gormgolden:after_update", afterCallback)
	callback.Delete().After("gorm:delete").Register("gormgolden:after_delete", afterCallback)
	callback.Raw().After("gorm:raw").Register("gormgolden:after_raw", afterCallback)
	callback.Row().After("gorm:row").Register("gormgolden:after_row", afterCallback)
	
	return nil
}

func afterCallback(db *gorm.DB) {
	if db.Statement != nil && db.Statement.SQL.String() != "" {
		sql := buildFullSQL(db)
		queryManager.AddQuery(sql)
	}
}

func buildFullSQL(db *gorm.DB) string {
	if db.Statement == nil || db.Dialector == nil {
		return ""
	}
	
	sql := db.Statement.SQL.String()
	vars := db.Statement.Vars
	
	if sql == "" {
		return ""
	}
	
	// Use GORM's built-in Explain method to replace placeholders
	return db.Dialector.Explain(sql, vars...)
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

