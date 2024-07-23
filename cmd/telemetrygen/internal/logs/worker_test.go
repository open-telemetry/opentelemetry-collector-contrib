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

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/internal/common"
)

const (
	telemetryAttrKeyOne   = "k1"
	telemetryAttrKeyTwo   = "k2"
	telemetryAttrValueOne = "v1"
	telemetryAttrValueTwo = "v2"
)

type mockExporter struct {
	logs []sdklog.Record
}

func (m *mockExporter) Export(_ context.Context, records []sdklog.Record) error {
	m.logs = append(m.logs, records...)
	return nil
}

func (m *mockExporter) Shutdown(_ context.Context) error {
	return nil
}

func (m *mockExporter) ForceFlush(_ context.Context) error {
	return nil
}

func TestFixedNumberOfLogs(t *testing.T) {
	cfg := &Config{
		Config: common.Config{
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
	require.NoError(t, Run(cfg, expFunc, logger))

	time.Sleep(1 * time.Second)

	// verify
	require.Len(t, m.logs, 5)
}

func TestRateOfLogs(t *testing.T) {
	cfg := &Config{
		Config: common.Config{
			Rate:          10,
			TotalDuration: time.Second / 2,
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
	require.NoError(t, Run(cfg, expFunc, zap.NewNop()))

	// verify
	// the minimum acceptable number of logs for the rate of 10/sec for half a second
	assert.True(t, len(m.logs) >= 5, "there should have been 5 or more logs, had %d", len(m.logs))
	// the maximum acceptable number of logs for the rate of 10/sec for half a second
	assert.True(t, len(m.logs) <= 20, "there should have been less than 20 logs, had %d", len(m.logs))
}

func TestUnthrottled(t *testing.T) {
	cfg := &Config{
		Config: common.Config{
			TotalDuration: 1 * time.Second,
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
	require.NoError(t, Run(cfg, expFunc, logger))

	assert.True(t, len(m.logs) > 100, "there should have been more than 100 logs, had %d", len(m.logs))
}

func TestCustomBody(t *testing.T) {
	cfg := &Config{
		Body:    "custom body",
		NumLogs: 1,
		Config: common.Config{
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
	require.NoError(t, Run(cfg, expFunc, logger))

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
	require.NoError(t, Run(cfg, expFunc, logger))

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
	require.NoError(t, Run(cfg, expFunc, logger))

	time.Sleep(1 * time.Second)

	// verify
	require.Len(t, m.logs, qty)
	for _, l := range m.logs {
		assert.Equal(t, 2, l.AttributesLen(), "shouldn't have less than 2 attributes")

		l.WalkAttributes(func(attr log.KeyValue) bool {
			if attr.Key == telemetryAttrKeyOne {
				assert.EqualValues(t, attr.Value.AsString(), telemetryAttrValueOne)
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
	require.NoError(t, Run(cfg, expFunc, logger))

	time.Sleep(1 * time.Second)

	// verify
	require.Len(t, m.logs, qty)
	for _, l := range m.logs {
		assert.Equal(t, 3, l.AttributesLen(), "shouldn't have less than 3 attributes")
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
	require.NoError(t, Run(cfg, expFunc, logger))

	// verify
	require.Len(t, m.logs, qty)
	for _, l := range m.logs {
		assert.Equal(t, "ae87dadd90e9935a4bc9660628efd569", l.TraceID().String())
		assert.Equal(t, "5828fa4960140870", l.SpanID().String())
	}
}

func configWithNoAttributes(qty int, body string) *Config {
	return &Config{
		Body:    body,
		NumLogs: qty,
		Config: common.Config{
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
		Config: common.Config{
			WorkerCount:         1,
			TelemetryAttributes: common.KeyValue{telemetryAttrKeyOne: telemetryAttrValueOne},
		},
		SeverityText:   "Info",
		SeverityNumber: 9,
	}
}

func configWithMultipleAttributes(qty int, body string) *Config {
	kvs := common.KeyValue{telemetryAttrKeyOne: telemetryAttrValueOne, telemetryAttrKeyTwo: telemetryAttrValueTwo}
	return &Config{
		Body:    body,
		NumLogs: qty,
		Config: common.Config{
			WorkerCount:         1,
			TelemetryAttributes: kvs,
		},
		SeverityText:   "Info",
		SeverityNumber: 9,
	}
}
