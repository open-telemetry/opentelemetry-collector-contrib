// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package genainormalizerprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/openinference"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/otelsemconv"
)

// valueTransformer applies value-level normalization after an attribute is
// renamed. Sources that do not need value normalization may leave this nil.
// TODO [kylehounslow]: Review interface vs. typed function
type valueTransformer interface {
	TransformValue(targetKey, value string) string
}

// sourceNormalizer holds per-source state used during normalization.
type sourceNormalizer struct {
	lookupTable     map[string]string
	transformValue  valueTransformer
	removeOriginals bool
	overwrite       bool
}

// newSourceNormalizer wires up a sourceNormalizer from a validated Source
// config. Unknown source names produce a no-op normalizer; Config validation
// rejects them upstream.
func newSourceNormalizer(src Source) sourceNormalizer {
	sn := sourceNormalizer{
		removeOriginals: src.RemoveOriginals,
		overwrite:       src.Overwrite,
	}
	if src.Name == SourceOpenInference {
		sn.lookupTable = openinference.LookupTable
		sn.transformValue = openinference.Transformer{}
	}
	return sn
}

// genaiNormalizerProcessor normalizes span attributes for each configured source.
type genaiNormalizerProcessor struct {
	sources []sourceNormalizer
}

// newGenaiNormalizerProcessor builds a processor from a validated Config.
// Sources are applied in the order specified in the configuration.
func newGenaiNormalizerProcessor(cfg *Config) *genaiNormalizerProcessor {
	p := &genaiNormalizerProcessor{sources: make([]sourceNormalizer, 0, len(cfg.Sources))}
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
			if scopeWrote && ss.SchemaUrl() == "" {
				ss.SetSchemaUrl(otelsemconv.SchemaURL)
			}
		}
	}
	return td, nil
}

// normalizeAttributes applies the source's rename rules to attrs. It returns
// true if at least one attribute was written.
func (sn *sourceNormalizer) normalizeAttributes(attrs pcommon.Map) bool {
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
		return false
	}

	wrote := false
	for _, r := range renames {
		val, ok := attrs.Get(r.from)
		if !ok {
			continue
		}
		dest, existed := attrs.GetOrPutEmpty(r.to)
		if existed && !sn.overwrite {
			continue
		}

		switch val.Type() {
		case pcommon.ValueTypeStr:
			dest.SetStr(sn.applyValueTransform(r.to, val.Str()))
		case pcommon.ValueTypeInt:
			dest.SetInt(val.Int())
		case pcommon.ValueTypeDouble:
			dest.SetDouble(val.Double())
		case pcommon.ValueTypeBool:
			dest.SetBool(val.Bool())
		default:
			val.CopyTo(dest)
		}
		wrote = true

		if sn.removeOriginals {
			attrs.Remove(r.from)
		}
	}
	return wrote
}

// applyValueTransform runs the per-source value transformer if one is set.
func (sn *sourceNormalizer) applyValueTransform(targetKey, value string) string {
	if sn.transformValue == nil {
		return value
	}
	return sn.transformValue.TransformValue(targetKey, value)
}
