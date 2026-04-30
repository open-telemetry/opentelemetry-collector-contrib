// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package genainormalizerprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor"

import (
	"context"
	"slices"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// sourceNormalizer holds per-source state used during normalization.
type sourceNormalizer struct {
	lookupTable     map[string]mappingTarget
	removeOriginals bool
	overwrite       bool
}

// genaiNormalizerProcessor normalizes span attributes for each configured source.
type genaiNormalizerProcessor struct {
	sources []sourceNormalizer
}

// newGenaiNormalizerProcessor builds a processor from a validated Config.
func newGenaiNormalizerProcessor(cfg *Config) *genaiNormalizerProcessor {
	names := make([]SourceName, 0, len(cfg.Sources))
	for name := range cfg.Sources {
		names = append(names, name)
	}
	slices.Sort(names)

	p := &genaiNormalizerProcessor{sources: make([]sourceNormalizer, 0, len(names))}
	for _, name := range names {
		src := cfg.Sources[name]
		p.sources = append(p.sources, sourceNormalizer{
			lookupTable:     buildLookupTable(name, src.CustomMappings),
			removeOriginals: src.RemoveOriginals,
			overwrite:       src.Overwrite,
		})
	}
	return p
}

func (p *genaiNormalizerProcessor) processTraces(_ context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		ilss := rss.At(i).ScopeSpans()
		for j := 0; j < ilss.Len(); j++ {
			spans := ilss.At(j).Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				for s := range p.sources {
					p.sources[s].normalizeAttributes(span.Attributes())
				}
				events := span.Events()
				for e := 0; e < events.Len(); e++ {
					for s := range p.sources {
						p.sources[s].normalizeAttributes(events.At(e).Attributes())
					}
				}
			}
		}
	}
	return td, nil
}

func (sn *sourceNormalizer) normalizeAttributes(attrs pcommon.Map) {
	type rename struct {
		from   string
		target mappingTarget
	}
	var renames []rename

	attrs.Range(func(k string, _ pcommon.Value) bool {
		if target, ok := sn.lookupTable[k]; ok {
			renames = append(renames, rename{k, target})
		}
		return true
	})

	if len(renames) == 0 {
		return
	}

	for _, r := range renames {
		val, ok := attrs.Get(r.from)
		if !ok {
			continue
		}
		if _, exists := attrs.Get(r.target.key); exists && !sn.overwrite {
			continue
		}

		// Read scalar values up front so they remain valid across subsequent
		// Put* calls that may reallocate the backing slice.
		switch val.Type() {
		case pcommon.ValueTypeStr:
			s := val.Str()
			if r.target.wrapSlice {
				arr := attrs.PutEmptySlice(r.target.key)
				arr.AppendEmpty().SetStr(s)
			} else {
				attrs.PutStr(r.target.key, transformValue(r.target.key, s))
			}
		case pcommon.ValueTypeInt:
			attrs.PutInt(r.target.key, val.Int())
		case pcommon.ValueTypeDouble:
			attrs.PutDouble(r.target.key, val.Double())
		case pcommon.ValueTypeBool:
			attrs.PutBool(r.target.key, val.Bool())
		default:
			// Map / Slice / Bytes: snapshot before writing so the source read
			// is independent of any reallocation the destination write causes.
			snap := pcommon.NewValueEmpty()
			val.CopyTo(snap)
			snap.CopyTo(attrs.PutEmpty(r.target.key))
		}

		if sn.removeOriginals {
			attrs.Remove(r.from)
		}
	}
}
