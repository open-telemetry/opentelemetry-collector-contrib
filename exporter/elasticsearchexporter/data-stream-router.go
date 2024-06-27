// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// routeLogRecord returns the name of the index to send the log record to according to data stream routing attributes.
// It searches for the routing attributes on the log record, scope, and resource.
// It creates missing routing attributes on the log record if they are not found.
func routeLogRecord(
	record *plog.LogRecord,
	scope pcommon.InstrumentationScope,
	resource pcommon.Resource,
) string {
	dataSet := ensureAttribute(dataStreamDataset, defaultDataStreamDataset, record.Attributes(), scope.Attributes(), resource.Attributes())
	namespace := ensureAttribute(dataStreamNamespace, defaultDataStreamNamespace, record.Attributes(), scope.Attributes(), resource.Attributes())
	dataType := ensureAttribute(dataStreamType, defaultDataStreamTypeLogs, record.Attributes(), scope.Attributes(), resource.Attributes())
	return fmt.Sprintf("%s-%s-%s", dataType, dataSet, namespace)
}

// routeDataPoint returns the name of the index to send the data point to according to data stream routing attributes.
// It searches for the routing attributes on the data point, scope, and resource.
// It creates missing routing attributes on the data point if they are not found.
func routeDataPoint(
	dataPoint pmetric.NumberDataPoint,
	scope pcommon.InstrumentationScope,
	resource pcommon.Resource,
) string {
	dataSet := ensureAttribute(dataStreamDataset, defaultDataStreamDataset, dataPoint.Attributes(), scope.Attributes(), resource.Attributes())
	namespace := ensureAttribute(dataStreamNamespace, defaultDataStreamNamespace, dataPoint.Attributes(), scope.Attributes(), resource.Attributes())
	dataType := ensureAttribute(dataStreamType, defaultDataStreamTypeMetrics, dataPoint.Attributes(), scope.Attributes(), resource.Attributes())
	return fmt.Sprintf("%s-%s-%s", dataType, dataSet, namespace)
}

// routeSpan returns the name of the index to send the span to according to data stream routing attributes.
// It searches for the routing attributes on the span, scope, and resource.
// It creates missing routing attributes on the span if they are not found.
func routeSpan(
	span ptrace.Span,
	scope pcommon.InstrumentationScope,
	resource pcommon.Resource,
) string {
	dataSet := ensureAttribute(dataStreamDataset, defaultDataStreamDataset, span.Attributes(), scope.Attributes(), resource.Attributes())
	namespace := ensureAttribute(dataStreamNamespace, defaultDataStreamNamespace, span.Attributes(), scope.Attributes(), resource.Attributes())
	dataType := ensureAttribute(dataStreamType, defaultDataStreamTypeTraces, span.Attributes(), scope.Attributes(), resource.Attributes())
	return fmt.Sprintf("%s-%s-%s", dataType, dataSet, namespace)
}

func ensureAttribute(attributeName string, defaultValue string, recordAttributes, scopeAttributes, resourceAttributes pcommon.Map) string {
	// Fetch value according to precedence and default.
	value := getFromAttributesNew(attributeName, defaultValue, recordAttributes, scopeAttributes, resourceAttributes)

	// Always set the value on the record, as record attributes have the highest precedence.
	recordAttributes.PutStr(attributeName, value)

	return value
}
