package main

import (
	"fmt"

	"github.com/po3rin/gormgolden/common"
)

func main() {
	qm := common.NewQueryManager("test.golden.sql")

	// GORM v1 style query (期待値)
	gormv1Query := "SELECT COUNT(`id`) FROM `requests` LEFT JOIN `request_default_field_values` ON `requests`.`id`=`request_default_field_values`.`request_id` WHERE ((`requests`.`tenant_id`=`01H0000000000000000000769`) AND (`requests`.`status` IN (`in_progress`)) AND (`requests`.`num` IN (1,2)) AND (`requests`.`num`=1) AND (`requests`.`form_id`=_UTF8MB401H0000000000000000799000) AND (`author_group_id` IN (_UTF8MB4test-group-id-001)) AND (`opened_at`>=_UTF8MB42023-01-01) AND (`opened_at`<=_UTF8MB42023-12-31) AND (`approved_at`>=_UTF8MB42023-02-01) AND (`approved_at`<=_UTF8MB42023-12-31) AND (`requests`.`tenant_id`=_UTF8MB401H0000000000000000000769));"

	// GORM v2 style query (実際)
	gormv2Query := "SELECT COUNT(`id`) FROM `requests` LEFT JOIN `request_default_field_values` ON `requests`.`id`=`request_default_field_values`.`request_id` WHERE `requests`.`tenant_id`=_UTF8MB401H0000000000000000000769 AND `requests`.`status` IN (_UTF8MB4in_progress) AND `requests`.`num` IN (1,2) AND `requests`.`num`=1 AND `requests`.`form_id`=_UTF8MB401H0000000000000000799000 AND `author_group_id` IN (_UTF8MB4test-group-id-001) AND `opened_at`>=_UTF8MB42023-01-01 AND `opened_at`<=_UTF8MB42023-12-31 AND `approved_at`>=_UTF8MB42023-02-01 AND `approved_at`<=_UTF8MB42023-12-31 AND `requests`.`tenant_id`=_UTF8MB401H0000000000000000000769;"

	fmt.Println("=== Original Queries ===")
	fmt.Println("GORM v1:", gormv1Query)
	fmt.Println("GORM v2:", gormv2Query)

	// Debug WHERE clause processing
	fmt.Println("\n=== GORM v1 WHERE Debug ===")
	qm.DebugWhereClause(gormv1Query)

	fmt.Println("\n=== GORM v2 WHERE Debug ===")
	qm.DebugWhereClause(gormv2Query)

	// 正規化してテスト
	fmt.Println("\n=== Comparison Result ===")
	areEqual, norm1, norm2 := qm.CompareQueriesDebug(gormv1Query, gormv2Query)
	fmt.Printf("Are they equal? %v\n", areEqual)

	fmt.Println("\n=== Normalized Queries ===")
	fmt.Println("GORM v1 normalized:", norm1)
	fmt.Println("GORM v2 normalized:", norm2)

	if !areEqual {
		fmt.Println("\n=== Differences ===")
		fmt.Printf("Length: v1=%d, v2=%d\n", len(norm1), len(norm2))

		// Find first difference
		minLen := len(norm1)
		if len(norm2) < minLen {
			minLen = len(norm2)
		}

		for i := 0; i < minLen; i++ {
			if norm1[i] != norm2[i] {
				fmt.Printf("First difference at position %d: v1='%c' v2='%c'\n", i, norm1[i], norm2[i])
				break
			}
		}
	}
}
