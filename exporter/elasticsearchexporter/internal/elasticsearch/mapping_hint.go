// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearch // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"

import (
	"slices"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

const (
	MappingHintsAttrKey = "elasticsearch.mapping.hints"
)

type MappingHint string

const (
	HintAggregateMetricDouble MappingHint = "aggregate_metric_double"
	HintDocCount              MappingHint = "_doc_count"
)

type MappingHintGetter struct {
	hints []MappingHint
}

// NewMappingHintGetter creates a new MappingHintGetter
func NewMappingHintGetter(attr pcommon.Map) (g MappingHintGetter) {
	v, ok := attr.Get(MappingHintsAttrKey)
	if !ok || v.Type() != pcommon.ValueTypeSlice {
		return
	}
	slice := v.Slice()
	g.hints = slices.Grow(g.hints, slice.Len())
	for i := range slice.Len() {
		g.hints = append(g.hints, MappingHint(slice.At(i).Str()))
	}
	return
}

// HasMappingHint checks whether the getter contains the requested mapping hint
func (g MappingHintGetter) HasMappingHint(hint MappingHint) bool {
	return slices.Contains(g.hints, hint)
}
