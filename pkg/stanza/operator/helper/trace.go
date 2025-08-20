// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"

import (
	"encoding/hex"
	"fmt"

	"go.opentelemetry.io/collector/config/configoptional"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/errors"
)

// NewTraceParser creates a new trace parser with default values
func NewTraceParser() TraceParser {
	traceID := entry.NewBodyField("trace_id")
	spanID := entry.NewBodyField("span_id")
	traceFlags := entry.NewBodyField("trace_flags")
	return TraceParser{
		TraceID: configoptional.Some(TraceIDConfig{
			ParseFrom: &traceID,
		}),
		SpanID: configoptional.Some(SpanIDConfig{
			ParseFrom: &spanID,
		}),
		TraceFlags: configoptional.Some(TraceFlagsConfig{
			ParseFrom: &traceFlags,
		}),
	}
}

// TraceParser is a helper that parses trace spans (and flags) onto an entry.
type TraceParser struct {
	TraceID    configoptional.Optional[TraceIDConfig]    `mapstructure:"trace_id,omitempty"`
	SpanID     configoptional.Optional[SpanIDConfig]     `mapstructure:"span_id,omitempty"`
	TraceFlags configoptional.Optional[TraceFlagsConfig] `mapstructure:"trace_flags,omitempty"`
}

type TraceIDConfig struct {
	ParseFrom *entry.Field `mapstructure:"parse_from,omitempty"`
}

type SpanIDConfig struct {
	ParseFrom *entry.Field `mapstructure:"parse_from,omitempty"`
}

type TraceFlagsConfig struct {
	ParseFrom *entry.Field `mapstructure:"parse_from,omitempty"`
}

// Validate validates a TraceParser, and reconfigures it if necessary
func (t *TraceParser) Validate() error {
	if !t.TraceID.HasValue() {
		t.TraceID = configoptional.Some(TraceIDConfig{})
	}
	if traceIDConfig := t.TraceID.Get(); traceIDConfig != nil && traceIDConfig.ParseFrom == nil {
		field := entry.NewBodyField("trace_id")
		traceIDConfig.ParseFrom = &field
	}
	if !t.SpanID.HasValue() {
		t.SpanID = configoptional.Some(SpanIDConfig{})
	}
	if spanIDConfig := t.SpanID.Get(); spanIDConfig != nil && spanIDConfig.ParseFrom == nil {
		field := entry.NewBodyField("span_id")
		spanIDConfig.ParseFrom = &field
	}
	if !t.TraceFlags.HasValue() {
		t.TraceFlags = configoptional.Some(TraceFlagsConfig{})
	}
	if traceFlagsConfig := t.TraceFlags.Get(); traceFlagsConfig != nil && traceFlagsConfig.ParseFrom == nil {
		field := entry.NewBodyField("trace_flags")
		traceFlagsConfig.ParseFrom = &field
	}
	return nil
}

// Best effort hex parsing for trace, spans and flags
func parseHexField(entry *entry.Entry, field *entry.Field) ([]byte, error) {
	value, ok := entry.Get(field)
	if !ok {
		return nil, nil
	}

	data, err := hex.DecodeString(fmt.Sprintf("%v", value))
	if err != nil {
		return nil, err
	}
	return data, nil
}

// Parse will parse a trace (trace_id, span_id and flags) from a field and attach it to the entry
func (t *TraceParser) Parse(entry *entry.Entry) error {
	var errTraceID, errSpanID, errTraceFlags error
	if t.TraceID.HasValue() {
		entry.TraceID, errTraceID = parseHexField(entry, t.TraceID.Get().ParseFrom)
	}
	if t.SpanID.HasValue() {
		entry.SpanID, errSpanID = parseHexField(entry, t.SpanID.Get().ParseFrom)
	}
	if t.TraceFlags.HasValue() {
		entry.TraceFlags, errTraceFlags = parseHexField(entry, t.TraceFlags.Get().ParseFrom)
	}
	if errTraceID != nil || errTraceFlags != nil || errSpanID != nil {
		err := errors.NewError("Error decoding traces for logs", "")
		if errTraceID != nil {
			_ = err.WithDetails("trace_id", errTraceID.Error())
		}
		if errSpanID != nil {
			_ = err.WithDetails("span_id", errSpanID.Error())
		}
		if errTraceFlags != nil {
			_ = err.WithDetails("trace_flags", errTraceFlags.Error())
		}
		return err
	}
	return nil
}
