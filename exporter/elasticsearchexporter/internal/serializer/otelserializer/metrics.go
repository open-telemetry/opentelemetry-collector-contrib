// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelserializer // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer/otelserializer"

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"sort"
	"strconv"

	"github.com/elastic/go-structform"
	"github.com/elastic/go-structform/json"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/datapoints"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer"
)

func (*Serializer) SerializeMetrics(resource pcommon.Resource, resourceSchemaURL string, scope pcommon.InstrumentationScope, scopeSchemaURL string, dataPoints []datapoints.DataPoint, validationErrors *[]error, idx elasticsearch.Index, buf *bytes.Buffer) (map[string]string, error) {
	if len(dataPoints) == 0 {
		return nil, nil
	}
	dp0 := dataPoints[0]

	v := json.NewVisitor(buf)
	// Enable ExplicitRadixPoint such that 1.0 is encoded as 1.0 instead of 1.
	// This is required to generate the correct dynamic mapping in ES.
	v.SetExplicitRadixPoint(true)
	_ = v.OnObjectStart(-1, structform.AnyType)
	writeTimestampField(v, "@timestamp", dp0.Timestamp())
	if dp0.StartTimestamp() != 0 {
		writeTimestampField(v, "start_timestamp", dp0.StartTimestamp())
	}
	writeStringFieldSkipDefault(v, "unit", dp0.Metric().Unit())
	writeDataStream(v, idx)
	writeAttributes(v, dp0.Attributes(), true)
	writeResource(v, resource, resourceSchemaURL, true)
	writeScope(v, scope, scopeSchemaURL, true)
	dynamicTemplates := serializeDataPoints(v, dataPoints, validationErrors)
	_ = v.OnObjectFinished()
	return dynamicTemplates, nil
}

func serializeDataPoints(v *json.Visitor, dataPoints []datapoints.DataPoint, validationErrors *[]error) map[string]string {
	_ = v.OnKey("metrics")
	_ = v.OnObjectStart(-1, structform.AnyType)

	dynamicTemplates := make(map[string]string, len(dataPoints))
	var docCount uint64
	metricNamesSet := make(map[string]bool, len(dataPoints))
	metricNames := make([]string, 0, len(dataPoints))
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
		_ = v.OnKey(metric.Name())
		// TODO: support quantiles
		// https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/34561
		serializer.WriteValue(v, value, false)
		// DynamicTemplate returns the name of dynamic template that applies to the metric and data point,
		// so that the field is indexed into Elasticsearch with the correct mapping. The name should correspond to a
		// dynamic template that is defined in ES mapping, e.g.
		// https://github.com/elastic/elasticsearch/blob/8.15/x-pack/plugin/core/template-resources/src/main/resources/metrics%40mappings.json
		dynamicTemplates["metrics."+metric.Name()] = dp.DynamicTemplate(metric)
	}
	_ = v.OnObjectFinished()
	if docCount != 0 {
		writeUIntField(v, "_doc_count", docCount)
	}
	sort.Strings(metricNames)
	hasher := fnv.New32a()
	for _, name := range metricNames {
		_, _ = hasher.Write([]byte(name))
	}
	// workaround for https://github.com/elastic/elasticsearch/issues/99123
	// should use a string field to benefit from run-length encoding
	writeStringFieldSkipDefault(v, "_metric_names_hash", strconv.FormatUint(uint64(hasher.Sum32()), 16))

	return dynamicTemplates
}
