// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package genainormalizerprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/custom"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/openinference"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/openllmetry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/otelsemconv"
)

// valueTransformer applies source-specific value-level normalization into
// dst. A nil transformer falls back to a plain src.CopyTo(dst).
type valueTransformer func(targetKey string, src, dst pcommon.Value)

// attributeAggregator reconstructs structured attributes from multiple
// flat keys (e.g. flattened OpenInference messages → JSON string).
type attributeAggregator interface {
	AggregateAttributes(attrs pcommon.Map, removeOriginals, overwrite bool) bool
}

// sourceNormalizer holds per-source state used during normalization.
type sourceNormalizer struct {
	lookupTable     map[string]string
	transformValue  valueTransformer
	aggregators     []attributeAggregator
	removeOriginals bool
	overwrite       bool
}

// newSourceNormalizer wires up a sourceNormalizer from a validated Source
// config. Built-in source names use pre-defined mapping tables; any other
// name is a user-defined source driven by the config's Mappings and
// ValueMappings fields.
func newSourceNormalizer(src Source) sourceNormalizer {
	sn := sourceNormalizer{
		removeOriginals: src.RemoveOriginals,
		overwrite:       src.Overwrite,
	}
	switch src.Name {
	case SourceOpenInference:
		sn.lookupTable = openinference.LookupTable
		sn.transformValue = openinference.Transform
		sn.aggregators = []attributeAggregator{openinference.MessageAggregator{}}
	case SourceOpenLLMetry:
		sn.lookupTable = openllmetry.LookupTable
		sn.transformValue = openllmetry.Transform
	default:
		sn.lookupTable = src.Mappings
		if len(src.ValueMappings) > 0 {
			sn.transformValue = custom.Transform(src.ValueMappings)
		}
	}
	return sn
}

// genaiNormalizerProcessor normalizes span attributes for each configured source.
type genaiNormalizerProcessor struct {
	sources            []sourceNormalizer
	overwriteSchemaURL bool
}

// newGenaiNormalizerProcessor builds a processor from a validated Config.
// Sources are applied in the order specified in the configuration.
func newGenaiNormalizerProcessor(cfg *Config) *genaiNormalizerProcessor {
	p := &genaiNormalizerProcessor{
		sources:            make([]sourceNormalizer, 0, len(cfg.Sources)),
		overwriteSchemaURL: cfg.OverwriteSchemaURL,
	}
	for _, src := range cfg.Sources {
		p.sources = append(p.sources, newSourceNormalizer(src))
	}
	return p
}

func (p *genaiNormalizerProcessor) processTraces(_ context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		ilss := rss.At(i).ScopeSpans()
		for j := 0; j < ilss.Len(); j++ {
			ss := ilss.At(j)
			spans := ss.Spans()
			scopeWrote := false
			for k := 0; k < spans.Len(); k++ {
				attrs := spans.At(k).Attributes()
				for s := range p.sources {
					scopeWrote = p.sources[s].normalizeAttributes(attrs) || scopeWrote
				}
			}
			if scopeWrote && (ss.SchemaUrl() == "" || p.overwriteSchemaURL) {
				ss.SetSchemaUrl(otelsemconv.SchemaURL)
			}
		}
	}
	return td, nil
}

// normalizeAttributes applies the source's rename rules to attrs. It returns
// true if at least one attribute was written.
func (sn *sourceNormalizer) normalizeAttributes(attrs pcommon.Map) bool {
	wrote := false

	// Phase 1: aggregators (reconstruct structured data from flat keys)
	for _, agg := range sn.aggregators {
		if agg.AggregateAttributes(attrs, sn.removeOriginals, sn.overwrite) {
			wrote = true
		}
	}

	// Phase 2: simple key renames
	type rename struct {
		from string
		to   string
	}
	var renames []rename

	attrs.Range(func(k string, _ pcommon.Value) bool {
		if target, ok := sn.lookupTable[k]; ok {
			renames = append(renames, rename{k, target})
		}
		return true
	})

	if len(renames) == 0 {
		return wrote
	}

	for _, r := range renames {
		val, ok := attrs.Get(r.from)
		if !ok {
			continue
		}
		dest, existed := attrs.GetOrPutEmpty(r.to)
		if existed && !sn.overwrite {
			continue
		}

		if !otelsemconv.Coerce(r.to, val, dest) {
			if !existed {
				attrs.Remove(r.to)
			}
			continue
		}
		if sn.transformValue != nil {
			sn.transformValue(r.to, dest, dest)
		}
		wrote = true

		if sn.removeOriginals {
			attrs.Remove(r.from)
		}
	}
	return wrote
}
