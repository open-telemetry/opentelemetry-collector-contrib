// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lookupprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor"

import (
	"context"
	"fmt"
	"strconv"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/lookupsource"
)

type lookupProcessor struct {
	source     lookupsource.Source
	attributes []AttributeConfig
	logger     *zap.Logger
}

func newLookupProcessor(source lookupsource.Source, cfg *Config, logger *zap.Logger) *lookupProcessor {
	return &lookupProcessor{
		source:     source,
		attributes: cfg.Attributes,
		logger:     logger,
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

		// Process resource-level attributes
		for _, attrCfg := range p.attributes {
			if attrCfg.GetContext() == ContextResource {
				p.processAttribute(ctx, resourceAttrs, attrCfg)
			}
		}

		// Process log records
		scopeLogs := rl.ScopeLogs()
		for j := 0; j < scopeLogs.Len(); j++ {
			sl := scopeLogs.At(j)
			logRecords := sl.LogRecords()
			for k := 0; k < logRecords.Len(); k++ {
				lr := logRecords.At(k)
				logAttrs := lr.Attributes()

				for _, attrCfg := range p.attributes {
					if attrCfg.GetContext() == ContextRecord {
						p.processAttribute(ctx, logAttrs, attrCfg)
					}
				}
			}
		}
	}

	return ld, nil
}

func (p *lookupProcessor) processAttribute(ctx context.Context, attrs pcommon.Map, cfg AttributeConfig) {
	sourceVal, exists := attrs.Get(cfg.FromAttribute)
	if !exists {
		return
	}

	key := valueToString(sourceVal)
	if key == "" {
		return
	}

	result, found, err := p.source.Lookup(ctx, key)
	if err != nil {
		p.logger.Debug("lookup failed",
			zap.String("key", key),
			zap.String("from_attribute", cfg.FromAttribute),
			zap.Error(err),
		)
		return
	}

	action := cfg.GetAction()

	var shouldSet bool
	if action == ActionUpsert {
		shouldSet = true
	} else {
		_, targetExists := attrs.Get(cfg.Key)
		shouldSet = (action == ActionInsert && !targetExists) || (action == ActionUpdate && targetExists)
	}

	if !shouldSet {
		return
	}

	if !found {
		if cfg.Default == "" {
			return
		}
		attrs.PutStr(cfg.Key, cfg.Default)
		return
	}

	putAny(attrs, cfg.Key, result)
}

func putAny(attrs pcommon.Map, key string, v any) {
	switch val := v.(type) {
	case string:
		attrs.PutStr(key, val)
	case int:
		attrs.PutInt(key, int64(val))
	case int64:
		attrs.PutInt(key, val)
	case float64:
		attrs.PutDouble(key, val)
	case bool:
		attrs.PutBool(key, val)
	default:
		attrs.PutStr(key, fmt.Sprintf("%v", v))
	}
}

func valueToString(v pcommon.Value) string {
	switch v.Type() {
	case pcommon.ValueTypeStr:
		return v.Str()
	case pcommon.ValueTypeInt:
		return strconv.FormatInt(v.Int(), 10)
	case pcommon.ValueTypeDouble:
		return strconv.FormatFloat(v.Double(), 'g', -1, 64)
	case pcommon.ValueTypeBool:
		return strconv.FormatBool(v.Bool())
	default:
		return v.AsString()
	}
}
