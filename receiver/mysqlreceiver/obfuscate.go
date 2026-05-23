// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mysqlreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver"

import (
	"github.com/DataDog/datadog-agent/pkg/obfuscate"
)

var (
	obfuscateSQLConfig = obfuscate.SQLConfig{DBMS: "mysql"}
	obfuscatorConfig   = obfuscate.Config{
		SQLExecPlan: defaultSQLPlanObfuscateSettings,
	}
)

type obfuscator obfuscate.Obfuscator

func newObfuscator() *obfuscator {
	return (*obfuscator)(obfuscate.NewObfuscator(obfuscatorConfig))
}

func (o *obfuscator) obfuscateSQLString(sql string) (string, error) {
	obfuscatedQuery, err := (*obfuscate.Obfuscator)(o).ObfuscateSQLStringWithOptions(sql, &obfuscateSQLConfig, "")
	if err != nil {
		return "", err
	}
	return obfuscatedQuery.Query, nil
}

func (o *obfuscator) obfuscatePlan(plan string) (string, error) {
	obfuscated, err := (*obfuscate.Obfuscator)(o).ObfuscateSQLExecPlan(plan, false)
	if err != nil {
		return "", err
	}
	return obfuscated, nil
}

// For further information, see https://dev.mysql.com/doc/refman/8.4/en/explain.html
// MySQL 8.4 EXPLAIN FORMAT=JSON produces two formats depending on explain_json_format_version:
//   - Version 1 (default): query_block → ordering_operation → table → attached_condition
//   - Version 2: top-level query + inputs array, each node has condition/operation/access_type etc.
var defaultSQLPlanObfuscateSettings = obfuscate.JSONConfig{
	Enabled: true,
	ObfuscateSQLValues: []string{
		// v2: the full query text
		"query",
		// v2: SQL condition expression on a filter node
		"condition",
		// v2: human-readable description of a plan node (e.g. "Filter: (...)", "Table scan on ...")
		"operation",
		// v1: SQL condition expression attached to a table scan
		"attached_condition",
	},
	KeepValues: []string{
		// v1 structural fields
		"cost_info",
		"ordering_operation",
		"query_block",
		"query_plan",
		"query_type",
		"select_id",
		"table",
		"used_columns",
		"using_filesort",
		// v2 structural fields
		"access_type",
		"covering",
		"estimated_rows",
		"estimated_total_cost",
		"filter_columns",
		"index_access_type",
		"index_name",
		"inputs",
		"json_schema_version",
		"limit",
		"limit_offset",
		"per_chunk_limit",
		"ranges",
		"row_ids",
		"schema_name",
		"sort_fields",
		"table_name",
	},
}
