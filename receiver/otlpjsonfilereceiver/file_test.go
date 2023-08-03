// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otlpjsonfilereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otlpjsonfilereceiver"

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otlpjsonfilereceiver/internal/metadata"
)

func TestDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	require.NotNil(t, cfg, "failed to create default config")
	require.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestFileTracesReceiver(t *testing.T) {
	tempFolder := t.TempDir()
	factory := NewFactory()
	cfg := createDefaultConfig().(*Config)
	cfg.Config.Include = []string{filepath.Join(tempFolder, "*")}
	cfg.Config.StartAt = "beginning"
	sink := new(consumertest.TracesSink)
	receiver, err := factory.CreateTracesReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, sink)
	assert.NoError(t, err)
	err = receiver.Start(context.Background(), nil)
	require.NoError(t, err)

	td := testdata.GenerateTracesTwoSpansSameResource()
	marshaler := &ptrace.JSONMarshaler{}
	b, err := marshaler.MarshalTraces(td)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempFolder, "traces.json"), b, 0600)
	assert.NoError(t, err)
	time.Sleep(1 * time.Second)
	require.Len(t, sink.AllTraces(), 1)

	assert.EqualValues(t, td, sink.AllTraces()[0])
	err = receiver.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestFileMetricsReceiver(t *testing.T) {
	tempFolder := t.TempDir()
	factory := NewFactory()
	cfg := createDefaultConfig().(*Config)
	cfg.Config.Include = []string{filepath.Join(tempFolder, "*")}
	cfg.Config.StartAt = "beginning"
	sink := new(consumertest.MetricsSink)
	receiver, err := factory.CreateMetricsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, sink)
	assert.NoError(t, err)
	err = receiver.Start(context.Background(), nil)
	assert.NoError(t, err)

	md := testdata.GenerateMetricsManyMetricsSameResource(5)
	marshaler := &pmetric.JSONMarshaler{}
	b, err := marshaler.MarshalMetrics(md)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempFolder, "metrics.json"), b, 0600)
	assert.NoError(t, err)
	time.Sleep(1 * time.Second)

	require.Len(t, sink.AllMetrics(), 1)
	assert.EqualValues(t, md, sink.AllMetrics()[0])
	err = receiver.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestFileLogsReceiver(t *testing.T) {
	tempFolder := t.TempDir()
	factory := NewFactory()
	cfg := createDefaultConfig().(*Config)
	cfg.Config.Include = []string{filepath.Join(tempFolder, "*")}
	cfg.Config.StartAt = "beginning"
	sink := new(consumertest.LogsSink)
	receiver, err := factory.CreateLogsReceiver(context.Background(), receivertest.NewNopCreateSettings(), cfg, sink)
	assert.NoError(t, err)
	err = receiver.Start(context.Background(), nil)
	assert.NoError(t, err)

	ld := testdata.GenerateLogsManyLogRecordsSameResource(5)
	marshaler := &plog.JSONMarshaler{}
	b, err := marshaler.MarshalLogs(ld)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempFolder, "logs.json"), b, 0600)
	assert.NoError(t, err)
	time.Sleep(1 * time.Second)

	require.Len(t, sink.AllLogs(), 1)
	assert.EqualValues(t, ld, sink.AllLogs()[0])
	err = receiver.Shutdown(context.Background())
	assert.NoError(t, err)
}

func testdataConfigYamlAsMap() *Config {
	return &Config{
		Config: fileconsumer.Config{
			IncludeFileName:         true,
			IncludeFilePath:         false,
			IncludeFileNameResolved: false,
			IncludeFilePathResolved: false,
			PollInterval:            200 * time.Millisecond,
			Splitter:                helper.NewSplitterConfig(),
			StartAt:                 "end",
			FingerprintSize:         1000,
			MaxLogSize:              1024 * 1024,
			MaxConcurrentFiles:      1024,
			MatchingCriteria: fileconsumer.MatchingCriteria{
				Include: []string{"/var/log/*.log"},
				Exclude: []string{"/var/log/example.log"},
			},
		},
	}
}

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	assert.Equal(t, testdataConfigYamlAsMap(), cfg)
}

func TestFileMixedSignals(t *testing.T) {
	tempFolder := t.TempDir()
	factory := NewFactory()
	cfg := createDefaultConfig().(*Config)
	cfg.Config.Include = []string{filepath.Join(tempFolder, "*")}
	cfg.Config.StartAt = "beginning"
	cs := receivertest.NewNopCreateSettings()
	ms := new(consumertest.MetricsSink)
	mr, err := factory.CreateMetricsReceiver(context.Background(), cs, cfg, ms)
	assert.NoError(t, err)
	err = mr.Start(context.Background(), nil)
	assert.NoError(t, err)
	ts := new(consumertest.TracesSink)
	tr, err := factory.CreateTracesReceiver(context.Background(), cs, cfg, ts)
	assert.NoError(t, err)
	err = tr.Start(context.Background(), nil)
	assert.NoError(t, err)
	ls := new(consumertest.LogsSink)
	lr, err := factory.CreateLogsReceiver(context.Background(), cs, cfg, ls)
	assert.NoError(t, err)
	err = lr.Start(context.Background(), nil)
	assert.NoError(t, err)

	md := testdata.GenerateMetricsManyMetricsSameResource(5)
	marshaler := &pmetric.JSONMarshaler{}
	b, err := marshaler.MarshalMetrics(md)
	assert.NoError(t, err)
	td := testdata.GenerateTracesTwoSpansSameResource()
	tmarshaler := &ptrace.JSONMarshaler{}
	b2, err := tmarshaler.MarshalTraces(td)
	assert.NoError(t, err)
	ld := testdata.GenerateLogsManyLogRecordsSameResource(5)
	lmarshaler := &plog.JSONMarshaler{}
	b3, err := lmarshaler.MarshalLogs(ld)
	assert.NoError(t, err)
	b = append(b, '\n')
	b = append(b, b2...)
	b = append(b, '\n')
	b = append(b, b3...)
	err = os.WriteFile(filepath.Join(tempFolder, "metrics.json"), b, 0600)
	assert.NoError(t, err)
	time.Sleep(1 * time.Second)

	require.Len(t, ms.AllMetrics(), 1)
	assert.EqualValues(t, md, ms.AllMetrics()[0])
	require.Len(t, ts.AllTraces(), 1)
	assert.EqualValues(t, td, ts.AllTraces()[0])
	require.Len(t, ls.AllLogs(), 1)
	assert.EqualValues(t, ld, ls.AllLogs()[0])
	err = mr.Shutdown(context.Background())
	assert.NoError(t, err)
	err = tr.Shutdown(context.Background())
	assert.NoError(t, err)
	err = lr.Shutdown(context.Background())
	assert.NoError(t, err)
}
