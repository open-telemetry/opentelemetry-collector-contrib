// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fileexporter

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exporterprofiles"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/otlpencodingextension"
)

type hostWithEncoding struct {
	encodings map[component.ID]component.Component
}

func (h hostWithEncoding) GetExtensions() map[component.ID]component.Component {
	return h.encodings
}

func TestEncoding(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Path = filepath.Join(t.TempDir(), "encoding.txt")
	id := component.MustNewID("otlpjson")
	cfg.Encoding = &id

	ef := otlpencodingextension.NewFactory()
	efCfg := ef.CreateDefaultConfig().(*otlpencodingextension.Config)
	efCfg.Protocol = "otlp_json"
	ext, err := ef.Create(context.Background(), extensiontest.NewNopSettings(), efCfg)
	require.NoError(t, err)
	require.NoError(t, ext.Start(context.Background(), componenttest.NewNopHost()))

	me, err := f.CreateMetrics(context.Background(), exportertest.NewNopSettings(), cfg)
	require.NoError(t, err)
	te, err := f.CreateTraces(context.Background(), exportertest.NewNopSettings(), cfg)
	require.NoError(t, err)
	le, err := f.CreateLogs(context.Background(), exportertest.NewNopSettings(), cfg)
	require.NoError(t, err)
	pe, err := f.(exporterprofiles.Factory).CreateProfiles(context.Background(), exportertest.NewNopSettings(), cfg)
	require.NoError(t, err)
	host := hostWithEncoding{
		map[component.ID]component.Component{id: ext},
	}
	require.NoError(t, me.Start(context.Background(), host))
	require.NoError(t, te.Start(context.Background(), host))
	require.NoError(t, le.Start(context.Background(), host))
	require.NoError(t, pe.Start(context.Background(), host))
	t.Cleanup(func() {
	})

	require.NoError(t, me.ConsumeMetrics(context.Background(), generateMetrics()))
	require.NoError(t, te.ConsumeTraces(context.Background(), generateTraces()))
	require.NoError(t, le.ConsumeLogs(context.Background(), generateLogs()))
	require.NoError(t, pe.ConsumeProfiles(context.Background(), generateProfiles()))

	require.NoError(t, me.Shutdown(context.Background()))
	require.NoError(t, te.Shutdown(context.Background()))
	require.NoError(t, le.Shutdown(context.Background()))
	require.NoError(t, pe.Shutdown(context.Background()))

	b, err := os.ReadFile(cfg.Path)
	require.NoError(t, err)
	require.Contains(t, string(b), `{"resourceMetrics":`)
	require.Contains(t, string(b), `{"resourceSpans":`)
	require.Contains(t, string(b), `{"resourceLogs":`)
	require.Contains(t, string(b), `{"resourceProfiles":`)
}

func generateLogs() plog.Logs {
	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("resource", "R1")
	l := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	l.Body().SetStr("test log message")
	l.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	return logs
}

func generateProfiles() pprofile.Profiles {
	proflies := pprofile.NewProfiles()
	rp := proflies.ResourceProfiles().AppendEmpty()
	rp.Resource().Attributes().PutStr("resource", "R1")
	p := rp.ScopeProfiles().AppendEmpty().Profiles().AppendEmpty()
	p.SetProfileID(pprofile.NewProfileIDEmpty())
	p.SetStartTime(pcommon.NewTimestampFromTime(time.Now().Add(-1 * time.Second)))
	p.SetEndTime(pcommon.NewTimestampFromTime(time.Now()))
	return proflies
}

func generateMetrics() pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("resource", "R1")
	m := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName("test_metric")
	dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.Attributes().PutStr("test_attr", "value_1")
	dp.SetIntValue(123)
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	return metrics
}

func generateTraces() ptrace.Traces {
	traces := ptrace.NewTraces()
	rs := traces.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("resource", "R1")
	span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.Attributes().PutStr("test_attr", "value_1")
	span.SetName("test_span")
	span.SetStartTimestamp(pcommon.NewTimestampFromTime(time.Now().Add(-1 * time.Second)))
	span.SetEndTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	return traces
}
