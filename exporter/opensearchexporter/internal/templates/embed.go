// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package templates embeds the composable index templates the exporter installs
// for the otel-v1 mapping mode. The templates mirror the Data Prepper schemas
// at https://github.com/opensearch-project/data-prepper/tree/main/data-prepper-plugins/opensearch/src/main/resources/index-template
// and ensure date_nanos timestamps and typed dynamic-attribute mappings before
// any documents are indexed.
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
