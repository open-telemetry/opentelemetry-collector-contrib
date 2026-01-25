// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package marshaler // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/marshaler"

import (
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"
)

var (
	_ LogsMarshaler     = pdataLogsMarshaler{}
	_ MetricsMarshaler  = pdataMetricsMarshaler{}
	_ TracesMarshaler   = pdataTracesMarshaler{}
	_ ProfilesMarshaler = pdataProfilesMarshaler{}
)

type pdataLogsMarshaler struct {
	marshaler plog.Marshaler
}

// NewPdataLogsMarshaler returns a new LogsMarshaler that marshals
// plog.Logs using the given plog.Marshaler. This can be used with
// the standard OTLP marshalers in the plog package, or with encoding
// extensions.
func NewPdataLogsMarshaler(m plog.Marshaler) LogsMarshaler {
	return pdataLogsMarshaler{marshaler: m}
}

func (p pdataLogsMarshaler) MarshalLogs(ld plog.Logs) ([]Message, error) {
	bts, err := p.marshaler.MarshalLogs(ld)
	if err != nil {
		return nil, err
	}
	return []Message{{Value: bts}}, nil
}

type pdataMetricsMarshaler struct {
	marshaler pmetric.Marshaler
}

// NewPdataMetricsMarshaler returns a new MetricsMarshaler that marshals
// pmetric.Metrics using the given pmetric.Marshaler. This can be used
// with the standard OTLP marshalers in the pmetric package, or with
// encoding extensions.
func NewPdataMetricsMarshaler(m pmetric.Marshaler) MetricsMarshaler {
	return pdataMetricsMarshaler{marshaler: m}
}

func (p pdataMetricsMarshaler) MarshalMetrics(ld pmetric.Metrics) ([]Message, error) {
	bts, err := p.marshaler.MarshalMetrics(ld)
	if err != nil {
		return nil, err
	}
	return []Message{{Value: bts}}, nil
}

type pdataTracesMarshaler struct {
	marshaler ptrace.Marshaler
}

// NewPdataTracesMarshaler returns a new TracesMarshaler that marshals
// ptrace.Traces using the given ptrace.Marshaler. This can be used
// with the standard OTLP marshalers in the ptrace package, or with
// encoding extensions.
//
// We split each trace's spans into its own payload to reduce the size of the
// payload and increase the chances it will fit a single kafka message.
func NewPdataTracesMarshaler(m ptrace.Marshaler) TracesMarshaler {
	return pdataTracesMarshaler{marshaler: m}
}

func (p pdataTracesMarshaler) MarshalTraces(td ptrace.Traces) ([]Message, error) {
	resourceSpans := td.ResourceSpans()
	var messages []Message
	var errs error

	for i := 0; i < resourceSpans.Len(); i++ {
		rs := resourceSpans.At(i)
		scopeSpans := rs.ScopeSpans()
		for j := 0; j < scopeSpans.Len(); j++ {
			ss := scopeSpans.At(j)
			spans := ss.Spans()
			for k := 0; k < spans.Len(); k++ {
				singleSpanTraces := ptrace.NewTraces()
				newRS := singleSpanTraces.ResourceSpans().AppendEmpty()
				rs.Resource().CopyTo(newRS.Resource())
				newRS.SetSchemaUrl(rs.SchemaUrl())
				newSS := newRS.ScopeSpans().AppendEmpty()
				ss.Scope().CopyTo(newSS.Scope())
				newSS.SetSchemaUrl(ss.SchemaUrl())
				spans.At(k).CopyTo(newSS.Spans().AppendEmpty())

				bts, err := p.marshaler.MarshalTraces(singleSpanTraces)
				if err != nil {
					errs = multierr.Append(errs, err)
					continue
				}
				messages = append(messages, Message{Value: bts})
			}
		}
	}
	return messages, errs
}

type pdataProfilesMarshaler struct {
	marshaler pprofile.Marshaler
}

// NewPdataProfilesMarshaler returns a new ProfilesMarshaler that marshals
// pprofile.Profiles using the given pprofile.Marshaler. This can be used with
// the standard OTLP marshalers in the pprofile package, or with encoding
// extensions.
func NewPdataProfilesMarshaler(m pprofile.Marshaler) ProfilesMarshaler {
	return pdataProfilesMarshaler{marshaler: m}
}

func (p pdataProfilesMarshaler) MarshalProfiles(ld pprofile.Profiles) ([]Message, error) {
	bts, err := p.marshaler.MarshalProfiles(ld)
	if err != nil {
		return nil, err
	}
	return []Message{{Value: bts}}, nil
}
