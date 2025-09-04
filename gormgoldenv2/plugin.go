package gormgoldenv2

import (
	"testing"

	"github.com/po3rin/gormgolden/common"
	"gorm.io/gorm"
)

type Plugin struct {
	GoldenFile   string
	queryManager *common.QueryManager
}

func New(filePath string) *Plugin {
	return &Plugin{
		GoldenFile:   filePath,
		queryManager: common.NewQueryManager(filePath),
	}
}

func (p *Plugin) Name() string {
	return "gormgolden"
}

func (p *Plugin) Initialize(db *gorm.DB) error {
	// Register callbacks for all operations
	callback := db.Callback()
	
	// Use closure to capture the plugin's queryManager
	afterCallbackFunc := func(db *gorm.DB) {
		if db.Statement != nil && db.Statement.SQL.String() != "" {
			sql := buildFullSQL(db)
			p.queryManager.AddQuery(sql)
		}
	}
	
	callback.Query().After("gorm:query").Register("gormgolden:after_query", afterCallbackFunc)
	callback.Create().After("gorm:create").Register("gormgolden:after_create", afterCallbackFunc)
	callback.Update().After("gorm:update").Register("gormgolden:after_update", afterCallbackFunc)
	callback.Delete().After("gorm:delete").Register("gormgolden:after_delete", afterCallbackFunc)
	callback.Raw().After("gorm:raw").Register("gormgolden:after_raw", afterCallbackFunc)
	callback.Row().After("gorm:row").Register("gormgolden:after_row", afterCallbackFunc)
	
	return nil
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

// Local methods on Plugin for managing queries
func (p *Plugin) Enable() {
	if p.queryManager != nil {
		p.queryManager.Enable()
	}
}

func (p *Plugin) Disable() {
	if p.queryManager != nil {
		p.queryManager.Disable()
	}
}

func (p *Plugin) Clear() {
	if p.queryManager != nil {
		p.queryManager.Clear()
	}
}

func (p *Plugin) GetQueries() []string {
	if p.queryManager != nil {
		return p.queryManager.GetQueries()
	}
	return []string{}
}

func (p *Plugin) SaveToFile(filePath string) error {
	if p.queryManager != nil {
		return p.queryManager.SaveToFile(filePath)
	}
	return nil
}

func (p *Plugin) AssertGolden(t *testing.T) {
	if p.queryManager != nil {
		p.queryManager.AssertGolden(t)
	}
}


