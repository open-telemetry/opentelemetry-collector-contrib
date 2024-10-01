// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"slices"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

const (
	mappingHintsAttrKey = "elasticsearch.mapping.hints"
)

type mappingHint string

const (
	hintAggregateMetricDouble mappingHint = "aggregate_metric_double"
	hintDocCount              mappingHint = "_doc_count"
)

type mappingHintGetter struct {
	hints []mappingHint
}

func newMappingHintGetter(attr pcommon.Map) (g mappingHintGetter) {
	v, ok := attr.Get(mappingHintsAttrKey)
	if !ok || v.Type() != pcommon.ValueTypeSlice {
		return
	}
	slice := v.Slice()
	g.hints = slices.Grow(g.hints, slice.Len())
	for i := 0; i < slice.Len(); i++ {
		g.hints = append(g.hints, mappingHint(slice.At(i).Str()))
	}
	return
}

func (g mappingHintGetter) HasMappingHint(hint mappingHint) bool {
	return slices.Contains(g.hints, hint)
}
