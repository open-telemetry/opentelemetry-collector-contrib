// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package sqltemplates // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal/sqltemplates"

import _ "embed"

// LOGS

//go:embed logs_table.sql
var LogsCreateTable string

//go:embed logs_insert.sql
var LogsInsert string

// TRACES

//go:embed traces_table.sql
var TracesCreateTable string

//go:embed traces_id_ts_lookup_table.sql
var TracesCreateTsTable string

//go:embed traces_id_ts_lookup_mv.sql
var TracesCreateTsView string

//go:embed traces_insert.sql
var TracesInsert string

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
