package gormgoldenv2

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/po3rin/gormgolden/common"
	"gorm.io/gorm"
)

type Plugin struct {
	GoldenFile   string
	queryManager *common.QueryManager
	instanceID   string
}

func New(filePath string) *Plugin {
	rand.Seed(time.Now().UnixNano())
	instanceID := fmt.Sprintf("gormgolden_%d_%d", time.Now().UnixNano(), rand.Intn(100000))
	return &Plugin{
		GoldenFile:   filePath,
		queryManager: common.NewQueryManager(filePath),
		instanceID:   instanceID,
	}
}

func (p *Plugin) Name() string {
	return p.instanceID
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

	callback.Query().After("gorm:query").Register(fmt.Sprintf("%s:after_query", p.instanceID), afterCallbackFunc)
	callback.Create().After("gorm:create").Register(fmt.Sprintf("%s:after_create", p.instanceID), afterCallbackFunc)
	callback.Update().After("gorm:update").Register(fmt.Sprintf("%s:after_update", p.instanceID), afterCallbackFunc)
	callback.Delete().After("gorm:delete").Register(fmt.Sprintf("%s:after_delete", p.instanceID), afterCallbackFunc)
	callback.Raw().After("gorm:raw").Register(fmt.Sprintf("%s:after_raw", p.instanceID), afterCallbackFunc)
	callback.Row().After("gorm:row").Register(fmt.Sprintf("%s:after_row", p.instanceID), afterCallbackFunc)

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
