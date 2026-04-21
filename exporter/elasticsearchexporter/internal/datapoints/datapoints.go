// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datapoints // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/datapoints"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
)

// DynamicTemplateMode selects which dynamic template names are returned by DynamicTemplate.
// OTel mode returns names matching Elasticsearch OTel metrics mapping; ECS mode returns
// names matching the APM metrics ingest pipeline (e.g. histogram_metrics, summary_metrics, double_metrics).
type DynamicTemplateMode int

const (
	DynamicTemplateModeOTel DynamicTemplateMode = iota
	DynamicTemplateModeECS
)

// DataPoint is an interface that allows specifying behavior for each type of data point
type DataPoint interface {
	Timestamp() pcommon.Timestamp
	StartTimestamp() pcommon.Timestamp
	Attributes() pcommon.Map
	Value() (pcommon.Value, error)
	DynamicTemplate(metric pmetric.Metric, mode DynamicTemplateMode) string
	DocCount() uint64
	HasMappingHint(elasticsearch.MappingHint) bool
	Metric() pmetric.Metric
}
