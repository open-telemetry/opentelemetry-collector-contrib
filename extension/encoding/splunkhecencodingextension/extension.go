// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/splunkhecencodingextension"

import (
	"context"

	jsoniter "github.com/json-iterator/go"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

var _ pmetric.Unmarshaler = (*splunkhecEncodingExtension)(nil)
var _ plog.Unmarshaler = (*splunkhecEncodingExtension)(nil)

var _ pmetric.Marshaler = (*splunkhecEncodingExtension)(nil)
var _ plog.Marshaler = (*splunkhecEncodingExtension)(nil)
var _ ptrace.Marshaler = (*splunkhecEncodingExtension)(nil)

type splunkhecEncodingExtension struct {
	config *Config
}

func (s *splunkhecEncodingExtension) MarshalMetrics(md pmetric.Metrics) ([]byte, error) {
	var events []*splunk.Event
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				m := sm.Metrics().At(k)
				evs, err := mapMetricToSplunkEvent(rm.Resource(), m, s.config)
				if err != nil {
					return nil, err
				}
				events = append(events, evs...)
			}
		}
	}
	stream := jsoniter.NewStream(jsoniter.ConfigDefault, nil, 512)
	for _, e := range events {
		stream.WriteVal(e)
		if stream.Error != nil {
			return nil, stream.Error
		}
	}
	return stream.Buffer(), nil
}

func (s *splunkhecEncodingExtension) MarshalLogs(ld plog.Logs) ([]byte, error) {
	var events []*splunk.Event
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rl := ld.ResourceLogs().At(i)
		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			sl := rl.ScopeLogs().At(j)
			for k := 0; k < sl.LogRecords().Len(); k++ {
				log := sl.LogRecords().At(k)
				ev := mapLogRecordToSplunkEvent(rl.Resource(), log, s.config)
				events = append(events, ev)
			}
		}
	}
	stream := jsoniter.NewStream(jsoniter.ConfigDefault, nil, 512)
	for _, e := range events {
		stream.WriteVal(e)
		if stream.Error != nil {
			return nil, stream.Error
		}
	}
	return stream.Buffer(), nil
}

func (s *splunkhecEncodingExtension) MarshalTraces(td ptrace.Traces) ([]byte, error) {
	var events []*splunk.Event
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rs := td.ResourceSpans().At(i)
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)
			for k := 0; k < ss.Spans().Len(); k++ {
				span := ss.Spans().At(k)
				ev := mapSpanToSplunkEvent(rs.Resource(), span, s.config)
				events = append(events, ev)
			}
		}
	}
	stream := jsoniter.NewStream(jsoniter.ConfigDefault, nil, 512)
	for _, e := range events {
		stream.WriteVal(e)
		if stream.Error != nil {
			return nil, stream.Error
		}
	}
	return stream.Buffer(), nil
}

func (s *splunkhecEncodingExtension) UnmarshalLogs(buf []byte) (plog.Logs, error) {
	var msg splunk.Event
	if err := jsoniter.Unmarshal(buf, &msg); err != nil {
		return plog.Logs{}, err
	}
	return splunkHecToLogData(msg, s.config)
}

func (s *splunkhecEncodingExtension) UnmarshalMetrics(buf []byte) (pmetric.Metrics, error) {
	var msg splunk.Event
	if err := jsoniter.Unmarshal(buf, &msg); err != nil {
		return pmetric.Metrics{}, err
	}
	return splunkHecToMetricsData(msg, s.config)
}

func (s *splunkhecEncodingExtension) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (s *splunkhecEncodingExtension) Shutdown(_ context.Context) error {
	return nil
}
