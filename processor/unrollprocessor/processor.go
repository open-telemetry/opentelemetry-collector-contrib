// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package unrollprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/unrollprocessor"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

type unrollProcessor struct {
	cfg *Config
}

// newUnrollProcessor returns a new unrollProcessor.
func newUnrollProcessor(config *Config) (*unrollProcessor, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &unrollProcessor{
		cfg: config,
	}, nil
}

// ProcessLogs implements the processor interface
func (p *unrollProcessor) ProcessLogs(_ context.Context, ld plog.Logs) (plog.Logs, error) {
	var errs error
	for _, rls := range ld.ResourceLogs().All() {
		for _, sls := range rls.ScopeLogs().All() {
			origLen := sls.LogRecords().Len()
			var last func() int
			if p.cfg.Recursive {
				last = sls.LogRecords().Len
			} else {
				last = func() int { return origLen }
			}
			for k := 0; k < last(); k++ {
				lr := sls.LogRecords().At(k)
				if lr.Body().Type() != pcommon.ValueTypeSlice {
					continue
				}
				for _, l := range lr.Body().Slice().All() {
					newRecord := sls.LogRecords().AppendEmpty()
					lr.CopyTo(newRecord)
					if err := setBody(newRecord, l); err != nil {
						errs = errors.Join(errs, fmt.Errorf("error setting body: %w", err))
					}
				}
			}
			sls.LogRecords().RemoveIf(func(lr plog.LogRecord) bool {
				if p.cfg.Recursive {
					return lr.Body().Type() == pcommon.ValueTypeSlice
				}
				if origLen > 0 {
					origLen--
					return lr.Body().Type() == pcommon.ValueTypeSlice
				}
				return false
			})
		}
	}
	return ld, errs
}

// setBody will set the body of the log record to the provided value
func setBody(newLogRecord plog.LogRecord, expansion pcommon.Value) error {
	switch expansion.Type() {
	case pcommon.ValueTypeStr:
		newLogRecord.Body().SetStr(expansion.Str())
	case pcommon.ValueTypeInt:
		newLogRecord.Body().SetInt(expansion.Int())
	case pcommon.ValueTypeDouble:
		newLogRecord.Body().SetDouble(expansion.Double())
	case pcommon.ValueTypeBool:
		newLogRecord.Body().SetBool(expansion.Bool())
	case pcommon.ValueTypeMap:
		expansion.Map().CopyTo(newLogRecord.Body().SetEmptyMap())
	case pcommon.ValueTypeSlice:
		expansion.Slice().CopyTo(newLogRecord.Body().SetEmptySlice())
	case pcommon.ValueTypeBytes:
		expansion.Bytes().CopyTo(newLogRecord.Body().SetEmptyBytes())
	case pcommon.ValueTypeEmpty:
		return newLogRecord.Body().FromRaw(nil)
	}
	return nil
}
