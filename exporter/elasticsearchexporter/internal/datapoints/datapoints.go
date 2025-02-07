// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datapoints // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/datapoints"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
)

// DataPoint is an interface that allows specifying behavior for each type of data point
type DataPoint interface {
	Timestamp() pcommon.Timestamp
	StartTimestamp() pcommon.Timestamp
	Attributes() pcommon.Map
	Value() (pcommon.Value, error)
	DynamicTemplate(pmetric.Metric) string
	DocCount() uint64
	HasMappingHint(elasticsearch.MappingHint) bool
	Metric() pmetric.Metric
}
