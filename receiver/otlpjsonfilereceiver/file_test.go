// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otlpjsonfilereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otlpjsonfilereceiver"

import (
	"context"
	"go.opentelemetry.io/collector/receiver"
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
	"go.opentelemetry.io/collector/pdata/testdata"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/attrs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/matcher"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otlpjsonfilereceiver/internal/metadata"
)

func TestDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	require.NotNil(t, cfg, "failed to create default config")
	require.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestFileMetricsReceiverWithReplay(t *testing.T) {
	tempFolder := t.TempDir()
	factory := NewFactory()
	cfg := createDefaultConfig().(*Config)
	cfg.Config.Include = []string{filepath.Join(tempFolder, "*")}
	cfg.Config.StartAt = "beginning"
	cfg.ReplayFile = true
	cfg.Config.PollInterval = 5 * time.Second

	sink := new(consumertest.MetricsSink)
	receiver, err := factory.CreateMetricsReceiver(context.Background(), receivertest.NewNopSettings(), cfg, sink)
	assert.NoError(t, err)
	err = receiver.Start(context.Background(), nil)
	assert.NoError(t, err)

	md := testdata.GenerateMetrics(5)
	marshaler := &pmetric.JSONMarshaler{}
	b, err := marshaler.MarshalMetrics(md)
	assert.NoError(t, err)
	b = append(b, '\n')
	err = os.WriteFile(filepath.Join(tempFolder, "metrics.json"), b, 0600)
	assert.NoError(t, err)

	// Wait for the first poll to complete.
	time.Sleep(cfg.Config.PollInterval + time.Second)
	require.Len(t, sink.AllMetrics(), 1)
	assert.EqualValues(t, md, sink.AllMetrics()[0])

	// Reset the sink and assert that the next poll replays all the existing metrics.
	sink.Reset()
	time.Sleep(cfg.Config.PollInterval + time.Second)
	require.Len(t, sink.AllMetrics(), 1)
	assert.EqualValues(t, md, sink.AllMetrics()[0])

	err = receiver.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestFileReceiver(t *testing.T) {
	type signalCtl struct {
		name           string
		createReceiver func(factory receiver.Factory, cfg *Config) (func() []any, component.Component, error)
		getCfg         func(*Config) *SignalConfig
		testFile       string
		testData       func() (any, []byte, error)
	}
	type testMode struct {
		name        string
		wantEntries int
		dataPrefix  string
		configure   func(config *SignalConfig)
	}
	type testCase struct {
		name string
		sig  signalCtl
		mode testMode
	}

	regEx := testMode{
		name:        "reg ex",
		wantEntries: 1,
		dataPrefix:  "prefix ",
		configure: func(config *SignalConfig) {
			config.RegEx = "prefix (.*)"
		},
	}
	defaultMode := testMode{
		name:        "default",
		wantEntries: 1,
		dataPrefix:  "",
		configure:   func(config *SignalConfig) {},
	}
	skipMode := testMode{
		name:        "skip",
		wantEntries: 0,
		dataPrefix:  "",
		configure: func(config *SignalConfig) {
			b := false
			config.Enabled = &b
		},
	}

	logs := signalCtl{
		name: "logs",
		createReceiver: func(factory receiver.Factory, cfg *Config) (func() []any, component.Component, error) {
			sink := new(consumertest.LogsSink)
			r, err := factory.CreateLogsReceiver(context.Background(), receivertest.NewNopSettings(), cfg, sink)
			return func() []any {
				logs := sink.AllLogs()
				res := make([]any, len(logs))
				for i, l := range logs {
					res[i] = l
				}
				return res
			}, r, err
		},
		getCfg: func(cfg *Config) *SignalConfig {
			return &cfg.Logs
		},
		testFile: "logs.json",
		testData: func() (any, []byte, error) {
			ld := testdata.GenerateLogs(5)
			m := &plog.JSONMarshaler{}
			b, err := m.MarshalLogs(ld)
			return ld, b, err
		},
	}
	traces := signalCtl{
		name: "traces",
		createReceiver: func(factory receiver.Factory, cfg *Config) (func() []any, component.Component, error) {
			sink := new(consumertest.TracesSink)
			r, err := factory.CreateTracesReceiver(context.Background(), receivertest.NewNopSettings(), cfg, sink)
			return func() []any {
				traces := sink.AllTraces()
				res := make([]any, len(traces))
				for i, l := range traces {
					res[i] = l
				}
				return res
			}, r, err
		},
		getCfg: func(cfg *Config) *SignalConfig {
			return &cfg.Traces
		},
		testFile: "traces.json",
		testData: func() (any, []byte, error) {
			td := testdata.GenerateTraces(2)
			m := &ptrace.JSONMarshaler{}
			b, err := m.MarshalTraces(td)
			return td, b, err
		},
	}
	metrics := signalCtl{
		name: "metrics",
		createReceiver: func(factory receiver.Factory, cfg *Config) (func() []any, component.Component, error) {
			sink := new(consumertest.MetricsSink)
			r, err := factory.CreateMetricsReceiver(context.Background(), receivertest.NewNopSettings(), cfg, sink)
			return func() []any {
				metrics := sink.AllMetrics()
				res := make([]any, len(metrics))
				for i, l := range metrics {
					res[i] = l
				}
				return res
			}, r, err

		},
		getCfg: func(cfg *Config) *SignalConfig {
			return &cfg.Metrics
		},
		testFile: "metrics.json",
		testData: func() (any, []byte, error) {
			md := testdata.GenerateMetrics(5)
			m := &pmetric.JSONMarshaler{}
			b, err := m.MarshalMetrics(md)
			return md, b, err
		},
	}

	var tests []testCase
	for _, sig := range []signalCtl{logs, traces, metrics} {
		for _, mode := range []testMode{defaultMode, skipMode, regEx} {
			tests = append(tests, testCase{
				name: sig.name + " " + mode.name,
				sig:  sig,
				mode: mode,
			})
		}
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sig := tt.sig
			mode := tt.mode
			tempFolder := t.TempDir()
			factory := NewFactory()
			cfg := createDefaultConfig().(*Config)
			cfg.Config.Include = []string{filepath.Join(tempFolder, "*")}
			cfg.Config.StartAt = "beginning"
			mode.configure(sig.getCfg(cfg))
			sink, r, err := sig.createReceiver(factory, cfg)
			assert.NoError(t, err)
			err = r.Start(context.Background(), nil)
			assert.NoError(t, err)

			ld, b, err := sig.testData()
			assert.NoError(t, err)
			b = append([]byte(mode.dataPrefix), b...)
			b = append(b, '\n')
			err = os.WriteFile(filepath.Join(tempFolder, sig.testFile), b, 0600)
			assert.NoError(t, err)
			time.Sleep(1 * time.Second)

			require.Len(t, sink(), mode.wantEntries)
			if mode.wantEntries > 0 {
				assert.EqualValues(t, ld, sink()[0])
			}
			err = r.Shutdown(context.Background())
			assert.NoError(t, err)
		})
	}
}

func testdataConfigYamlAsMap() *Config {
	return &Config{
		Config: fileconsumer.Config{
			Resolver: attrs.Resolver{
				IncludeFileName:         true,
				IncludeFilePath:         false,
				IncludeFileNameResolved: false,
				IncludeFilePathResolved: false,
			},
			PollInterval:       200 * time.Millisecond,
			Encoding:           "utf-8",
			StartAt:            "end",
			FingerprintSize:    1000,
			MaxLogSize:         1024 * 1024,
			MaxConcurrentFiles: 1024,
			FlushPeriod:        500 * time.Millisecond,
			Criteria: matcher.Criteria{
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
	require.NoError(t, sub.Unmarshal(cfg))

	assert.Equal(t, testdataConfigYamlAsMap(), cfg)
}

func TestFileMixedSignals(t *testing.T) {
	tempFolder := t.TempDir()
	factory := NewFactory()
	cfg := createDefaultConfig().(*Config)
	cfg.Config.Include = []string{filepath.Join(tempFolder, "*")}
	cfg.Config.StartAt = "beginning"
	cs := receivertest.NewNopSettings()
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

	md := testdata.GenerateMetrics(5)
	marshaler := &pmetric.JSONMarshaler{}
	b, err := marshaler.MarshalMetrics(md)
	assert.NoError(t, err)
	td := testdata.GenerateTraces(2)
	tmarshaler := &ptrace.JSONMarshaler{}
	b2, err := tmarshaler.MarshalTraces(td)
	assert.NoError(t, err)
	ld := testdata.GenerateLogs(5)
	lmarshaler := &plog.JSONMarshaler{}
	b3, err := lmarshaler.MarshalLogs(ld)
	assert.NoError(t, err)
	b = append(b, '\n')
	b = append(b, b2...)
	b = append(b, '\n')
	b = append(b, b3...)
	b = append(b, '\n')
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
