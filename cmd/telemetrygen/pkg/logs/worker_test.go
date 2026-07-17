// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/internal/config"
	types "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/pkg"
)

const (
	telemetryAttrKeyOne       = "k1"
	telemetryAttrKeyTwo       = "k2"
	telemetryAttrValueOne     = "v1"
	telemetryAttrValueTwo     = "v2"
	telemetryAttrIntKeyOne    = "intKey1"
	telemetryAttrIntValueOne  = 1
	telemetryAttrBoolKeyOne   = "boolKey1"
	telemetryAttrBoolValueOne = true
)

type mockExporter struct {
	logs        []sdklog.Record
	exportCalls [][]sdklog.Record // tracks each export call separately
}

func (m *mockExporter) Export(_ context.Context, records []sdklog.Record) error {
	m.logs = append(m.logs, records...)
	m.exportCalls = append(m.exportCalls, records)
	return nil
}

func (*mockExporter) Shutdown(context.Context) error {
	return nil
}

func (*mockExporter) ForceFlush(context.Context) error {
	return nil
}

func TestFixedNumberOfLogs(t *testing.T) {
	cfg := &Config{
		Config: config.Config{
			WorkerCount: 1,
		},
		NumLogs:        5,
		SeverityText:   "Info",
		SeverityNumber: 9,
	}

	m := &mockExporter{}
	expFunc := func() (sdklog.Exporter, error) {
		return m, nil
	}

	// test
	logger, _ := zap.NewDevelopment()
	require.NoError(t, run(cfg, expFunc, logger))

	time.Sleep(1 * time.Second)

	// verify
	require.Len(t, m.logs, 5)
}

func TestDurationInf(t *testing.T) {
	cfg := &Config{
		Config: config.Config{
			TotalDuration: types.DurationWithInf(-1),
		},
		SeverityText:   "Info",
		SeverityNumber: 9,
	}
	m := &mockExporter{}
	expFunc := func() (sdklog.Exporter, error) {
		return m, nil
	}

	// test
	require.NoError(t, run(cfg, expFunc, zap.NewNop()))
}

func TestRateOfLogs(t *testing.T) {
	cfg := &Config{
		Config: config.Config{
			Rate:          10,
			TotalDuration: types.DurationWithInf(time.Second / 2),
			WorkerCount:   1,
		},
		SeverityText:   "Info",
		SeverityNumber: 9,
	}
	m := &mockExporter{}
	expFunc := func() (sdklog.Exporter, error) {
		return m, nil
	}

	// test
	require.NoError(t, run(cfg, expFunc, zap.NewNop()))

	// verify
	// the minimum acceptable number of logs for the rate of 10/sec for half a second
	assert.GreaterOrEqual(t, len(m.logs), 5, "there should have been 5 or more logs, had %d", len(m.logs))
	// the maximum acceptable number of logs for the rate of 10/sec for half a second
	assert.LessOrEqual(t, len(m.logs), 20, "there should have been less than 20 logs, had %d", len(m.logs))
}

func TestUnthrottled(t *testing.T) {
	cfg := &Config{
		Config: config.Config{
			TotalDuration: types.DurationWithInf(1 * time.Second),
			WorkerCount:   1,
		},
		SeverityText:   "Info",
		SeverityNumber: 9,
	}
	m := &mockExporter{}
	expFunc := func() (sdklog.Exporter, error) {
		return m, nil
	}

	// test
	logger, _ := zap.NewDevelopment()
	require.NoError(t, run(cfg, expFunc, logger))

	assert.Greater(t, len(m.logs), 100, "there should have been more than 100 logs, had %d", len(m.logs))
}

func TestCustomBody(t *testing.T) {
	cfg := &Config{
		Body:    "custom body",
		NumLogs: 1,
		Config: config.Config{
			WorkerCount: 1,
		},
		SeverityText:   "Info",
		SeverityNumber: 9,
	}
	m := &mockExporter{}
	expFunc := func() (sdklog.Exporter, error) {
		return m, nil
	}

	// test
	logger, _ := zap.NewDevelopment()
	require.NoError(t, run(cfg, expFunc, logger))

	assert.Equal(t, "custom body", m.logs[0].Body().AsString())
}

func TestLogsWithNoTelemetryAttributes(t *testing.T) {
	cfg := configWithNoAttributes(2, "custom body")

	m := &mockExporter{}
	expFunc := func() (sdklog.Exporter, error) {
		return m, nil
	}

	// test
	logger, _ := zap.NewDevelopment()
	require.NoError(t, run(cfg, expFunc, logger))

	time.Sleep(1 * time.Second)

	// verify
	require.Len(t, m.logs, 2)
	for _, log := range m.logs {
		assert.Equal(t, 1, log.AttributesLen(), "shouldn't have more than 1 attribute")
	}
}

func TestLogsWithOneTelemetryAttributes(t *testing.T) {
	qty := 1
	cfg := configWithOneAttribute(qty, "custom body")

	m := &mockExporter{}
	expFunc := func() (sdklog.Exporter, error) {
		return m, nil
	}

	// test
	logger, _ := zap.NewDevelopment()
	require.NoError(t, run(cfg, expFunc, logger))

	time.Sleep(1 * time.Second)

	// verify
	require.Len(t, m.logs, qty)
	for _, l := range m.logs {
		assert.Equal(t, 2, l.AttributesLen(), "shouldn't have less than 2 attributes")

		l.WalkAttributes(func(attr log.KeyValue) bool {
			if attr.Key == telemetryAttrKeyOne {
				assert.Equal(t, telemetryAttrValueOne, attr.Value.AsString())
			}
			return true
		})
	}
}

func TestLogsWithMultipleTelemetryAttributes(t *testing.T) {
	qty := 1
	cfg := configWithMultipleAttributes(qty, "custom body")

	m := &mockExporter{}
	expFunc := func() (sdklog.Exporter, error) {
		return m, nil
	}

	// test
	logger, _ := zap.NewDevelopment()
	require.NoError(t, run(cfg, expFunc, logger))

	time.Sleep(1 * time.Second)

	// verify
	require.Len(t, m.logs, qty)
	for _, l := range m.logs {
		assert.Equal(t, 5, l.AttributesLen(), "it must have multiple attributes here")
		l.WalkAttributes(func(attr log.KeyValue) bool {
			if attr.Key == telemetryAttrKeyOne {
				assert.Equal(t, telemetryAttrValueOne, attr.Value.AsString())
			}
			if attr.Key == telemetryAttrKeyTwo {
				assert.Equal(t, telemetryAttrValueTwo, attr.Value.AsString())
			}
			if attr.Key == telemetryAttrIntKeyOne {
				assert.Equal(t, int64(telemetryAttrIntValueOne), attr.Value.AsInt64())
			}
			if attr.Key == telemetryAttrBoolKeyOne {
				assert.Equal(t, telemetryAttrBoolValueOne, attr.Value.AsBool())
			}
			return true
		})
	}
}

func TestLogsWithTraceIDAndSpanID(t *testing.T) {
	qty := 1
	cfg := configWithOneAttribute(qty, "custom body")
	cfg.TraceID = "ae87dadd90e9935a4bc9660628efd569"
	cfg.SpanID = "5828fa4960140870"

	m := &mockExporter{}
	expFunc := func() (sdklog.Exporter, error) {
		return m, nil
	}

	// test
	logger, _ := zap.NewDevelopment()
	require.NoError(t, run(cfg, expFunc, logger))

	// verify
	require.Len(t, m.logs, qty)
	for _, l := range m.logs {
		assert.Equal(t, "ae87dadd90e9935a4bc9660628efd569", l.TraceID().String())
		assert.Equal(t, "5828fa4960140870", l.SpanID().String())
	}
}

func TestBatching(t *testing.T) {
	cfg := &Config{
		NumLogs:        5,
		SeverityText:   "Info",
		SeverityNumber: 9,
	}
	cfg.WorkerCount = 1
	cfg.Batch = true
	cfg.BatchSize = 3
	m := &mockExporter{}
	expFunc := func() (sdklog.Exporter, error) {
		return m, nil
	}

	// test
	logger, _ := zap.NewDevelopment()
	require.NoError(t, run(cfg, expFunc, logger))

	// verify that logs were batched correctly
	assert.Len(t, m.logs, 5, "should have received all 5 logs")

	// verify batching behavior: should have 2 export calls
	// First call: 3 logs (batch size reached)
	// Second call: 2 logs (remaining logs flushed at end)
	assert.Len(t, m.exportCalls, 2, "should have made 2 export calls")
	assert.Len(t, m.exportCalls[0], 3, "first batch should have 3 logs")
	assert.Len(t, m.exportCalls[1], 2, "second batch should have 2 logs")
}

func TestNoBatching(t *testing.T) {
	cfg := &Config{
		NumLogs:        5,
		SeverityText:   "Info",
		SeverityNumber: 9,
	}
	cfg.WorkerCount = 1
	cfg.Batch = false // batching disabled
	m := &mockExporter{}
	expFunc := func() (sdklog.Exporter, error) {
		return m, nil
	}

	// test
	logger, _ := zap.NewDevelopment()
	require.NoError(t, run(cfg, expFunc, logger))

	// verify that logs were sent immediately (no batching)
	assert.Len(t, m.logs, 5, "should have received all 5 logs")

	// verify no batching behavior: should have 5 export calls, each with 1 log
	assert.Len(t, m.exportCalls, 5, "should have made 5 export calls (one per log)")
	for i, call := range m.exportCalls {
		assert.Len(t, call, 1, "export call %d should have exactly 1 log", i)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name           string
		cfg            *Config
		wantErrMessage string
	}{
		{
			name: "No duration, NumLogs",
			cfg: &Config{
				Config: config.Config{
					WorkerCount: 1,
				},
				TraceID: "123",
			},
			wantErrMessage: "either `logs` or `duration` must be greater than 0",
		},
		{
			name: "TraceID invalid",
			cfg: &Config{
				Config: config.Config{
					WorkerCount: 1,
				},
				NumLogs: 5,
				TraceID: "123",
			},
			wantErrMessage: "TraceID must be a 32 character hex string, like: 'ae87dadd90e9935a4bc9660628efd569'",
		},
		{
			name: "SpanID invalid",
			cfg: &Config{
				Config: config.Config{
					WorkerCount: 1,
				},
				NumLogs: 5,
				TraceID: "ae87dadd90e9935a4bc9660628efd569",
				SpanID:  "123",
			},
			wantErrMessage: "SpanID must be a 16 character hex string, like: '5828fa4960140870'",
		},
		{
			name: "LoadSize negative",
			cfg: &Config{
				Config: config.Config{
					WorkerCount: 1,
					LoadSize:    -1,
				},
				NumLogs: 5,
			},
			wantErrMessage: "load size must be non-negative, found -1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &mockExporter{}
			expFunc := func() (sdklog.Exporter, error) {
				return m, nil
			}
			logger, _ := zap.NewDevelopment()
			require.EqualError(t, run(tt.cfg, expFunc, logger), tt.wantErrMessage)
		})
	}
}

func configWithNoAttributes(qty int, body string) *Config {
	return &Config{
		Body:    body,
		NumLogs: qty,
		Config: config.Config{
			WorkerCount:         1,
			TelemetryAttributes: nil,
		},
		SeverityText:   "Info",
		SeverityNumber: 9,
	}
}

func configWithOneAttribute(qty int, body string) *Config {
	return &Config{
		Body:    body,
		NumLogs: qty,
		Config: config.Config{
			WorkerCount:         1,
			TelemetryAttributes: config.KeyValue{telemetryAttrKeyOne: telemetryAttrValueOne},
		},
		SeverityText:   "Info",
		SeverityNumber: 9,
	}
}

func configWithMultipleAttributes(qty int, body string) *Config {
	kvs := config.KeyValue{
		telemetryAttrKeyOne:     telemetryAttrValueOne,
		telemetryAttrKeyTwo:     telemetryAttrValueTwo,
		telemetryAttrIntKeyOne:  telemetryAttrIntValueOne,
		telemetryAttrBoolKeyOne: telemetryAttrBoolValueOne,
	}
	return &Config{
		Body:    body,
		NumLogs: qty,
		Config: config.Config{
			WorkerCount:         1,
			TelemetryAttributes: kvs,
		},
		SeverityText:   "Info",
		SeverityNumber: 9,
	}
}

func TestLogsWithLoadSize(t *testing.T) {
	// arrange
	cfg := &Config{
		Config: config.Config{
			WorkerCount: 1,
			LoadSize:    2, // 2MB of load data
		},
		NumLogs:        1,
		Body:           "test log",
		SeverityText:   "Info",
		SeverityNumber: 9,
	}
	m := &mockExporter{}
	expFunc := func() (sdklog.Exporter, error) {
		return m, nil
	}

	// act
	logger, _ := zap.NewDevelopment()
	require.NoError(t, run(cfg, expFunc, logger))

	time.Sleep(1 * time.Second)

	// assert
	require.Len(t, m.logs, 1)
	logRecord := m.logs[0]

	// Should have 3 attributes: "app" + 2 load attributes (load-0 and load-1)
	assert.Equal(t, 3, logRecord.AttributesLen(), "should have 3 attributes")

	// Check that load attributes exist and have the expected size
	var load0Found, load1Found bool
	logRecord.WalkAttributes(func(attr log.KeyValue) bool {
		if attr.Key == "load-0" {
			load0Found = true
			assert.Len(t, attr.Value.AsString(), config.CharactersPerMB, "load-0 should have 1MB of data")
		}
		if attr.Key == "load-1" {
			load1Found = true
			assert.Len(t, attr.Value.AsString(), config.CharactersPerMB, "load-1 should have 1MB of data")
		}
		return true
	})

	assert.True(t, load0Found, "should have load-0 attribute")
	assert.True(t, load1Found, "should have load-1 attribute")
}

func TestLogsWithDefaultLoadSize(t *testing.T) {
	// arrange
	cfg := NewConfig()
	cfg.NumLogs = 1
	cfg.Body = "test log"
	// LoadSize should default to 0

	m := &mockExporter{}
	expFunc := func() (sdklog.Exporter, error) {
		return m, nil
	}

	// act
	logger, _ := zap.NewDevelopment()
	require.NoError(t, run(cfg, expFunc, logger))

	time.Sleep(1 * time.Second)

	// assert
	require.Len(t, m.logs, 1)
	logRecord := m.logs[0]

	// Should have only 1 attribute ("app") by default (LoadSize = 0)
	assert.Equal(t, 1, logRecord.AttributesLen(), "should have only 1 attribute by default")
}

func TestLogsWithZeroLoadSize(t *testing.T) {
	// arrange
	cfg := &Config{
		Config: config.Config{
			WorkerCount: 1,
			LoadSize:    0, // Disable load size
		},
		NumLogs:        1,
		Body:           "test log",
		SeverityText:   "Info",
		SeverityNumber: 9,
	}
	m := &mockExporter{}
	expFunc := func() (sdklog.Exporter, error) {
		return m, nil
	}

	// act
	logger, _ := zap.NewDevelopment()
	require.NoError(t, run(cfg, expFunc, logger))

	time.Sleep(1 * time.Second)

	// assert
	require.Len(t, m.logs, 1)
	logRecord := m.logs[0]

	// Should have only 1 attribute ("app") when LoadSize is 0
	assert.Equal(t, 1, logRecord.AttributesLen(), "should have only 1 attribute when LoadSize is 0")
}
