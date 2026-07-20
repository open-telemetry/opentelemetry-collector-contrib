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
	HintHistogramRaw          MappingHint = "histogram:raw"
	// HintNoIndex signals that a document should not be indexed.
	// When present on a signal's attributes, the exporter skips bulk-emission
	// of the resulting document while leaving the signal available to any
	// upstream connector/processor that may have consumed it.
	//
	// Experimental: This hint is experimental and may change or be removed in future releases.
	HintNoIndex MappingHint = "_noindex"
)

type MappingHintGetter struct {
	hints []MappingHint
}

// NewMappingHintGetter creates a new MappingHintGetter
func NewMappingHintGetter(attr pcommon.Map) (g MappingHintGetter) {
	v, ok := attr.Get(MappingHintsAttrKey)
	if !ok || v.Type() != pcommon.ValueTypeSlice {
		return g
	}
	slice := v.Slice()
	g.hints = slices.Grow(g.hints, slice.Len())
	for _, hint := range slice.All() {
		g.hints = append(g.hints, MappingHint(hint.Str()))
	}
	return g
}

// HasMappingHint checks whether the getter contains the requested mapping hint
func (g MappingHintGetter) HasMappingHint(hint MappingHint) bool {
	return slices.Contains(g.hints, hint)
}
