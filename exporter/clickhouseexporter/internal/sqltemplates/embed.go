// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package sqltemplates // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal/sqltemplates"

import (
	_ "embed"
	"fmt"
	"strings"
	"text/template"
)

// templateFuncs provides helper functions available in all SQL templates.
var templateFuncs = template.FuncMap{
	// ident wraps a ClickHouse identifier in backticks, escaping any embedded backticks.
	"ident": func(s string) string {
		return fmt.Sprintf("`%s`", strings.ReplaceAll(s, "`", "\\`"))
	},
}

// newTemplate creates a named template with the shared function map.
func newTemplate(name, text string) *template.Template {
	return template.Must(template.New(name).Funcs(templateFuncs).Parse(text))
}

// LOGS

//go:embed logs_table.sql
var LogsCreateTable string

//go:embed logs_insert.sql
var LogsInsert string

//go:embed logs_json_table.sql
var LogsJSONCreateTable string

//go:embed logs_json_insert.sql
var LogsJSONInsert string

// Parsed templates for logs (text/template).
var (
	LogsCreateTableTmpl     = newTemplate("logs_table", LogsCreateTable)
	LogsInsertTmpl          = newTemplate("logs_insert", LogsInsert)
	LogsJSONCreateTableTmpl = newTemplate("logs_json_table", LogsJSONCreateTable)
	LogsJSONInsertTmpl      = newTemplate("logs_json_insert", LogsJSONInsert)
)

// CreateTableData contains the template parameters for creating a logs table.
type CreateTableData struct {
	Database          string
	TableName         string
	ClusterString     string
	Engine            string
	TTL               string
	HasFullTextSearch bool
}

// InsertData contains the template parameters for a logs INSERT statement.
type InsertData struct {
	Database               string
	TableName              string
	FeatureColumnNames     string
	FeatureColumnPositions string
}

// TRACES

//go:embed traces_table.sql
var TracesCreateTable string

//go:embed traces_json_table.sql
var TracesJSONCreateTable string

//go:embed traces_id_ts_lookup_table.sql
var TracesCreateTsTable string

//go:embed traces_id_ts_lookup_mv.sql
var TracesCreateTsView string

//go:embed traces_insert.sql
var TracesInsert string

//go:embed traces_json_insert.sql
var TracesJSONInsert string

// METRICS

//go:embed metrics_gauge_table.sql
var MetricsGaugeCreateTable string

//go:embed metrics_gauge_insert.sql
var MetricsGaugeInsert string

//go:embed metrics_exp_histogram_table.sql
var MetricsExpHistogramCreateTable string

//go:embed metrics_exp_histogram_insert.sql
var MetricsExpHistogramInsert string

//go:embed metrics_histogram_table.sql
var MetricsHistogramCreateTable string

//go:embed metrics_histogram_insert.sql
var MetricsHistogramInsert string

//go:embed metrics_sum_table.sql
var MetricsSumCreateTable string

//go:embed metrics_sum_insert.sql
var MetricsSumInsert string

//go:embed metrics_summary_table.sql
var MetricsSummaryCreateTable string

//go:embed metrics_summary_insert.sql
var MetricsSummaryInsert string
