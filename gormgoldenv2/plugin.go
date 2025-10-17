package gormgoldenv2

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/po3rin/gormgolden/common"
	"gorm.io/gorm"
)

type Plugin struct {
	GoldenFile   string
	queryManager *common.QueryManager
	instanceID   string
	mu           sync.Mutex // Protects access to Statement during parallel execution
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
		// Lock to protect Statement access from concurrent goroutines
		p.mu.Lock()
		defer p.mu.Unlock()

		if db.Statement != nil && db.Statement.SQL.String() != "" {
			// Immediately capture SQL and vars to avoid race conditions
			sql := db.Statement.SQL.String()
			vars := make([]interface{}, len(db.Statement.Vars))
			copy(vars, db.Statement.Vars)

			fullSQL := buildFullSQLWithVars(db.Dialector, sql, vars)

			// Filter: only record top-level queries
			trimmedSQL := strings.TrimSpace(fullSQL)

			// Strip leading SQL comments to find the actual SELECT statement
			sqlWithoutComments := trimmedSQL
			for strings.HasPrefix(sqlWithoutComments, "/*") {
				endComment := strings.Index(sqlWithoutComments, "*/")
				if endComment == -1 {
					break
				}
				sqlWithoutComments = strings.TrimSpace(sqlWithoutComments[endComment+2:])
			}

			// Record all queries (SELECT, INSERT, UPDATE, DELETE)
			// Note: Subqueries will be filtered out in post-processing by filterSubqueries()
			if len(sqlWithoutComments) > 0 {
				p.queryManager.AddQuery(fullSQL)
			}
		}
	}

	// Register callbacks for all query operations
	// Note: We record ALL queries here, and filter out subqueries later in post-processing
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

	if sql == "" {
		return ""
	}

	// Create a deep copy of vars to avoid race conditions in parallel execution
	// db.Statement.Vars is a []interface{} slice that can be modified by other goroutines
	vars := make([]interface{}, len(db.Statement.Vars))
	copy(vars, db.Statement.Vars)

	// Use GORM's built-in Explain method to replace placeholders
	return db.Dialector.Explain(sql, vars...)
}

// buildFullSQLWithVars builds full SQL from already-captured SQL and vars
func buildFullSQLWithVars(dialector gorm.Dialector, sql string, vars []interface{}) string {
	if dialector == nil || sql == "" {
		return ""
	}

	// Use GORM's built-in Explain method to replace placeholders
	return dialector.Explain(sql, vars...)
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

// AssertGoldenSorted asserts the recorded queries against a golden file, ignoring query order.
// This is useful when queries are executed in parallel and their order is non-deterministic.
func (p *Plugin) AssertGoldenSorted(t *testing.T) {
	if p.queryManager != nil {
		p.queryManager.AssertGoldenSorted(t)
	}
}
