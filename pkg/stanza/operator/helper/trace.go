// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package helper // import "github.com/open-telemetry/opentelemetry-log-collection/operator/helper"

import (
	"encoding/hex"
	"fmt"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/open-telemetry/opentelemetry-log-collection/errors"
)

// NewTraceParser creates a new trace parser with default values
func NewTraceParser() TraceParser {
	traceId := entry.NewBodyField("trace_id")
	spanId := entry.NewBodyField("span_id")
	traceFlags := entry.NewBodyField("trace_flags")
	return TraceParser{
		TraceId: &TraceIdConfig{
			ParseFrom: &traceId,
		},
		SpanId: &SpanIdConfig{
			ParseFrom: &spanId,
		},
		TraceFlags: &TraceFlagsConfig{
			ParseFrom: &traceFlags,
		},
	}
}

// TraceParser is a helper that parses trace spans (and flags) onto an entry.
type TraceParser struct {
	TraceId    *TraceIdConfig    `mapstructure:"trace_id,omitempty"    json:"trace_id,omitempty"    yaml:"trace_id,omitempty"`
	SpanId     *SpanIdConfig     `mapstructure:"span_id,omitempty"     json:"span_id,omitempty"     yaml:"span_id,omitempty"`
	TraceFlags *TraceFlagsConfig `mapstructure:"trace_flags,omitempty" json:"trace_flags,omitempty" yaml:"trace_flags,omitempty"`
}

type TraceIdConfig struct {
	ParseFrom *entry.Field `mapstructure:"parse_from,omitempty"  json:"parse_from,omitempty"  yaml:"parse_from,omitempty"`
}

type SpanIdConfig struct {
	ParseFrom *entry.Field `mapstructure:"parse_from,omitempty"  json:"parse_from,omitempty"  yaml:"parse_from,omitempty"`
}

type TraceFlagsConfig struct {
	ParseFrom *entry.Field `mapstructure:"parse_from,omitempty"  json:"parse_from,omitempty"  yaml:"parse_from,omitempty"`
}

// Validate validates a TraceParser, and reconfigures it if necessary
func (t *TraceParser) Validate() error {
	if t.TraceId == nil {
		t.TraceId = &TraceIdConfig{}
	}
	if t.TraceId.ParseFrom == nil {
		field := entry.NewBodyField("trace_id")
		t.TraceId.ParseFrom = &field
	}
	if t.SpanId == nil {
		t.SpanId = &SpanIdConfig{}
	}
	if t.SpanId.ParseFrom == nil {
		field := entry.NewBodyField("span_id")
		t.SpanId.ParseFrom = &field
	}
	if t.TraceFlags == nil {
		t.TraceFlags = &TraceFlagsConfig{}
	}
	if t.TraceFlags.ParseFrom == nil {
		field := entry.NewBodyField("trace_flags")
		t.TraceFlags.ParseFrom = &field
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
	var errTraceId, errSpanId, errTraceFlags error
	entry.TraceId, errTraceId = parseHexField(entry, t.TraceId.ParseFrom)
	entry.SpanId, errSpanId = parseHexField(entry, t.SpanId.ParseFrom)
	entry.TraceFlags, errTraceFlags = parseHexField(entry, t.TraceFlags.ParseFrom)
	if errTraceId != nil || errTraceFlags != nil || errSpanId != nil {
		err := errors.NewError("Error decoding traces for logs", "")
		if errTraceId != nil {
			_ = err.WithDetails("trace_id", errTraceId.Error())
		}
		if errSpanId != nil {
			_ = err.WithDetails("span_id", errSpanId.Error())
		}
		if errTraceFlags != nil {
			_ = err.WithDetails("trace_flags", errTraceFlags.Error())
		}
		return err
	}
	return nil
}
