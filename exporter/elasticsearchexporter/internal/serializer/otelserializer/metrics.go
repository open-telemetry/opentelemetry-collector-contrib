// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelserializer // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer/otelserializer"

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"

	"github.com/cespare/xxhash/v2"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/datapoints"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
)

func (*Serializer) SerializeMetrics(resource pcommon.Resource, resourceSchemaURL string, scope pcommon.InstrumentationScope, scopeSchemaURL string, dataPoints []datapoints.DataPoint, validationErrors *[]error, idx elasticsearch.Index, buf *bytes.Buffer) (map[string]string, error) {
	if len(dataPoints) == 0 {
		return nil, nil
	}
	dp0 := dataPoints[0]

	w := newJSONWriter(buf)
	w.startObject()
	first := true
	first = w.writeTimestampEpochMillisField("@timestamp", dp0.Timestamp(), first)
	if dp0.StartTimestamp() != 0 {
		first = w.writeTimestampEpochMillisField("start_timestamp", dp0.StartTimestamp(), first)
	}
	first = w.writeStringFieldSkipDefault("unit", dp0.Metric().Unit(), first)
	first = w.writeDataStream(idx, first)
	first = w.writeAttributes(dp0.Attributes(), true, first)
	first = w.writeResource(resource, resourceSchemaURL, true, first)
	first = w.writeScope(scope, scopeSchemaURL, true, first)
	dynamicTemplates := serializeDataPoints(&w, dataPoints, validationErrors, first)
	w.endObject()
	return dynamicTemplates, nil
}

func serializeDataPoints(w *jsonWriter, dataPoints []datapoints.DataPoint, validationErrors *[]error, first bool) map[string]string {
	first = w.key("metrics", first)
	w.startObject()

	dynamicTemplates := make(map[string]string, len(dataPoints))
	var docCount uint64
	metricNamesSet := make(map[string]bool, len(dataPoints))
	metricNames := make([]string, 0, len(dataPoints))
	firstMetric := true
	for _, dp := range dataPoints {
		metric := dp.Metric()
		if _, present := metricNamesSet[metric.Name()]; present {
			*validationErrors = append(
				*validationErrors,
				fmt.Errorf(
					"metric with name '%s' has already been serialized in document with timestamp %s",
					metric.Name(),
					dp.Timestamp().AsTime().UTC().Format(tsLayout),
				),
			)
			continue
		}
		metricNamesSet[metric.Name()] = true
		metricNames = append(metricNames, metric.Name())
		// TODO here's potential for more optimization by directly serializing the value instead of allocating a pcommon.Value
		//  the tradeoff is that this would imply a duplicated logic for the ECS mode
		value, err := dp.Value()
		if dp.HasMappingHint(elasticsearch.HintDocCount) {
			docCount = dp.DocCount()
		}
		if err != nil {
			*validationErrors = append(*validationErrors, err)
			continue
		}
		firstMetric = w.key(metric.Name(), firstMetric)
		// TODO: support quantiles
		// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/34561
		w.writeValue(value, false)
		// DynamicTemplate returns the name of dynamic template that applies to the metric and data point,
		// so that the field is indexed into Elasticsearch with the correct mapping. The name should correspond to a
		// dynamic template that is defined in ES mapping, e.g.
		// https://github.com/elastic/elasticsearch/blob/8.15/x-pack/plugin/core/template-resources/src/main/resources/metrics%40mappings.json
		dynamicTemplates["metrics."+metric.Name()] = dp.DynamicTemplate(metric, datapoints.DynamicTemplateModeOTel)
	}
	w.endObject()
	if docCount != 0 {
		first = w.writeUIntField("_doc_count", docCount, first)
	}
	sort.Strings(metricNames)
	hasher := xxhash.New()
	for _, name := range metricNames {
		_, _ = hasher.WriteString(name)
	}
	// workaround for https://github.com/elastic/elasticsearch/issues/99123
	// should use a string field to benefit from run-length encoding
	_ = w.writeStringFieldSkipDefault("_metric_names_hash", strconv.FormatUint(hasher.Sum64(), 16), first)

	return dynamicTemplates
}
