// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mysqlreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver"

import (
	"github.com/DataDog/datadog-agent/pkg/obfuscate"
)

var (
	obfuscateSQLConfig = obfuscate.SQLConfig{DBMS: "mysql"}
	obfuscatorConfig   = obfuscate.Config{
		SQLExecPlan:          defaultSQLPlanObfuscateSettings,
		SQLExecPlanNormalize: defaultSQLPlanNormalizeSettings,
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
	obfuscated, err := (*obfuscate.Obfuscator)(o).ObfuscateSQLExecPlan(plan, true)
	if err != nil {
		return "", err
	}
	return obfuscated, nil
}

// defaultSQLPlanNormalizeSettings are the default JSON obfuscator settings for both obfuscating and normalizing SQL
// execution plans
var defaultSQLPlanNormalizeSettings = obfuscate.JSONConfig{
	Enabled: true,
	ObfuscateSQLValues: []string{
		// mysql
		"attached_condition",
	},
	KeepValues: []string{
		// mysql
		"access_type",
		"backward_index_scan",
		"cacheable",
		"delete",
		"dependent",
		"first_match",
		"key",
		"key_length",
		"possible_keys",
		"ref",
		"select_id",
		"table_name",
		"update",
		"used_columns",
		"used_key_parts",
		"using_MRR",
		"using_filesort",
		"using_index",
		"using_join_buffer",
		"using_temporary_table",
	},
}

// defaultSQLPlanObfuscateSettings builds upon sqlPlanNormalizeSettings by including cost & row estimates in the keep
// list
var defaultSQLPlanObfuscateSettings = obfuscate.JSONConfig{
	Enabled: true,
	KeepValues: append([]string{
		// mysql
		"cost_info",
		"filtered",
		"rows_examined_per_join",
		"rows_examined_per_scan",
		"rows_produced_per_join",
	}, defaultSQLPlanNormalizeSettings.KeepValues...),
	ObfuscateSQLValues: defaultSQLPlanNormalizeSettings.ObfuscateSQLValues,
}
