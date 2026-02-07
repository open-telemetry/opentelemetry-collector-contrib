// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lookupprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/lookupsource"
)

type lookupProcessor struct {
	source  lookupsource.Source
	lookups []parsedLookup
	logger  *zap.Logger
}

func newLookupProcessor(source lookupsource.Source, lookups []parsedLookup, logger *zap.Logger) *lookupProcessor {
	return &lookupProcessor{
		source:  source,
		lookups: lookups,
		logger:  logger,
	}
}

func (p *lookupProcessor) Start(ctx context.Context, host component.Host) error {
	return p.source.Start(ctx, host)
}

func (p *lookupProcessor) Shutdown(ctx context.Context) error {
	return p.source.Shutdown(ctx)
}

func (p *lookupProcessor) processLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error) {
	resourceLogs := ld.ResourceLogs()
	for i := 0; i < resourceLogs.Len(); i++ {
		rl := resourceLogs.At(i)
		resourceAttrs := rl.Resource().Attributes()

		scopeLogs := rl.ScopeLogs()
		for j := 0; j < scopeLogs.Len(); j++ {
			sl := scopeLogs.At(j)
			logRecords := sl.LogRecords()
			for k := 0; k < logRecords.Len(); k++ {
				lr := logRecords.At(k)
				logAttrs := lr.Attributes()

				tCtx := ottllog.NewTransformContextPtr(rl, sl, lr)

				for li := range p.lookups {
					p.processLookup(ctx, tCtx, &p.lookups[li], logAttrs, resourceAttrs)
				}

				tCtx.Close()
			}
		}
	}

	return ld, nil
}

func (p *lookupProcessor) processLookup(
	ctx context.Context,
	tCtx *ottllog.TransformContext,
	lookup *parsedLookup,
	logAttrs pcommon.Map,
	resourceAttrs pcommon.Map,
) {
	rawKey, err := lookup.keyExpr.Eval(ctx, tCtx)
	if err != nil {
		p.logger.Debug("failed to evaluate key expression", zap.Error(err))
		return
	}
	if rawKey == nil {
		return
	}

	key := anyToString(rawKey)
	if key == "" {
		return
	}

	result, found, err := p.source.Lookup(ctx, key)
	if err != nil {
		p.logger.Debug("lookup failed", zap.String("key", key), zap.Error(err))
		return
	}

	for ai := range lookup.attributes {
		attr := &lookup.attributes[ai]
		attrCtx := attr.GetContext(lookup.context)

		var attrs pcommon.Map
		if attrCtx == ContextResource {
			attrs = resourceAttrs
		} else {
			attrs = logAttrs
		}

		value, ok := extractValue(result, found, attr)
		if !ok {
			continue
		}

		if err := attrs.PutEmpty(attr.Destination).FromRaw(value); err != nil {
			attrs.PutStr(attr.Destination, fmt.Sprintf("%v", value))
		}
	}
}

// extractValue gets the value to write for a given attribute mapping.
// Returns (value, shouldWrite).
func extractValue(result any, found bool, attr *AttributeMapping) (any, bool) {
	if !found {
		if attr.Default == "" {
			return nil, false
		}
		return attr.Default, true
	}

	if attr.Source == "" {
		// 1:1 scalar lookup — use entire result
		return result, true
	}

	// 1:N map lookup — extract the named field
	m, ok := result.(map[string]any)
	if !ok {
		return nil, false
	}

	val, exists := m[attr.Source]
	if !exists {
		if attr.Default == "" {
			return nil, false
		}
		return attr.Default, true
	}

	return val, true
}

func anyToString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	case int64:
		return fmt.Sprintf("%d", val)
	case float64:
		return fmt.Sprintf("%g", val)
	case bool:
		return fmt.Sprintf("%t", val)
	default:
		return fmt.Sprintf("%v", v)
	}
}
