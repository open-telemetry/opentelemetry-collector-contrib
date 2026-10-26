// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package templates embeds the composable index templates the exporter installs
// when mapping.manage_index_template is enabled.
//
// The otel-v1 templates mirror the Data Prepper schemas at
// https://github.com/opensearch-project/data-prepper/tree/main/data-prepper-plugins/opensearch/src/main/resources/index-template
// and ensure date_nanos timestamps and typed dynamic-attribute mappings before
// any documents are indexed.
//
// The ss4o templates map the attribute-bearing objects as flat_object so
// OpenSearch does not expand dots in attribute keys into nested objects. This
// prevents mapper_parsing_exception mapping conflicts between an attribute used
// as a concrete value ("code.function") and the same prefix used as an object
// ("code.function.name"). See:
// https://docs.opensearch.org/latest/mappings/supported-field-types/flat-object/
package templates // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter/internal/templates"

import _ "embed"

// OtelV1APMSpan is the composable index template body for traces in otel-v1 mode.
//
//go:embed otel-v1-apm-span.json
var OtelV1APMSpan string

// OtelV1Logs is the composable index template body for logs in otel-v1 mode.
//
//go:embed otel-v1-logs.json
var OtelV1Logs string

// SS4OTraces is the composable index template body for traces in ss4o mode.
//
//go:embed ss4o-traces.json
var SS4OTraces string

// SS4OLogs is the composable index template body for logs in ss4o mode.
//
//go:embed ss4o-logs.json
var SS4OLogs string
