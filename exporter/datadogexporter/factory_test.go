// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogexporter

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/DataDog/datadog-agent/comp/otelcol/otlp/testutil"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/inframetadata"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/inframetadata/payload"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata"
	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
)

var _ inframetadata.Pusher = (*testPusher)(nil)

type testPusher struct {
	mu       sync.Mutex
	payloads []payload.HostMetadata
	stopped  bool
	logger   *zap.Logger
	t        *testing.T
}

func newTestPusher(t *testing.T) *testPusher {
	return &testPusher{
		logger: zaptest.NewLogger(t),
		t:      t,
	}
}

func (p *testPusher) Push(_ context.Context, hm payload.HostMetadata) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stopped {
		p.logger.Error("Trying to push payload after stopping", zap.Any("payload", hm))
		p.t.Fail()
	}
	p.logger.Info("Storing host metadata payload", zap.Any("payload", hm))
	p.payloads = append(p.payloads, hm)
	return nil
}

func (p *testPusher) Payloads() []payload.HostMetadata {
	p.mu.Lock()
	p.stopped = true
	defer p.mu.Unlock()
	return p.payloads
}

func TestCreateAPIMetricsExporter(t *testing.T) {
	server := testutil.DatadogServerMock()
	defer server.Close()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "api").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	c := cfg.(*datadogconfig.Config)
	c.Metrics.Endpoint = server.URL
	c.HostMetadata.Enabled = false

	ctx := context.Background()
	exp, err := factory.CreateMetrics(
		ctx,
		exportertest.NewNopSettings(metadata.Type),
		cfg,
	)

	assert.NoError(t, err)
	assert.NotNil(t, exp)
}

func TestCreateAPIExporterFailOnInvalidKey_Zorkian(t *testing.T) {
	featuregateErr := featuregate.GlobalRegistry().Set("exporter.datadogexporter.UseLogsAgentExporter", false)
	assert.NoError(t, featuregateErr)
	server := testutil.DatadogServerMock(testutil.ValidateAPIKeyEndpointInvalid)
	defer server.Close()

	if isMetricExportV2Enabled() {
		require.NoError(t, enableZorkianMetricExport())
		defer require.NoError(t, enableNativeMetricExport())
	}

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "api").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	// Use the mock server for API key validation
	c := cfg.(*datadogconfig.Config)
	c.Metrics.Endpoint = server.URL
	c.HostMetadata.Enabled = false

	t.Run("true", func(t *testing.T) {
		c.API.FailOnInvalidKey = true
		ctx := context.Background()
		// metrics exporter
		mexp, err := factory.CreateMetrics(
			ctx,
			exportertest.NewNopSettings(metadata.Type),
			cfg,
		)
		assert.EqualError(t, err, "API Key validation failed")
		assert.Nil(t, mexp)

		texp, err := factory.CreateTraces(
			ctx,
			exportertest.NewNopSettings(metadata.Type),
			cfg,
		)
		assert.EqualError(t, err, "API Key validation failed")
		assert.Nil(t, texp)

		lexp, err := factory.CreateLogs(
			ctx,
			exportertest.NewNopSettings(metadata.Type),
			cfg,
		)
		assert.EqualError(t, err, "API Key validation failed")
		assert.Nil(t, lexp)
	})
	t.Run("false", func(t *testing.T) {
		c.API.FailOnInvalidKey = false
		ctx := context.Background()
		exp, err := factory.CreateMetrics(
			ctx,
			exportertest.NewNopSettings(metadata.Type),
			cfg,
		)
		assert.NoError(t, err)
		assert.NotNil(t, exp)

		texp, err := factory.CreateTraces(
			ctx,
			exportertest.NewNopSettings(metadata.Type),
			cfg,
		)
		assert.NoError(t, err)
		assert.NotNil(t, texp)

		lexp, err := factory.CreateLogs(
			ctx,
			exportertest.NewNopSettings(metadata.Type),
			cfg,
		)
		assert.NoError(t, err)
		assert.NotNil(t, lexp)
	})
	featuregateErr = featuregate.GlobalRegistry().Set("exporter.datadogexporter.UseLogsAgentExporter", true)
	assert.NoError(t, featuregateErr)
}

func TestCreateAPIExporterFailOnInvalidKey(t *testing.T) {
	featuregateErr := featuregate.GlobalRegistry().Set("exporter.datadogexporter.UseLogsAgentExporter", false)
	assert.NoError(t, featuregateErr)
	server := testutil.DatadogServerMock(testutil.ValidateAPIKeyEndpointInvalid)
	defer server.Close()

	if !isMetricExportV2Enabled() {
		require.NoError(t, enableNativeMetricExport())
		defer require.NoError(t, enableZorkianMetricExport())
	}

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "api").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	// Use the mock server for API key validation
	c := cfg.(*datadogconfig.Config)
	c.Metrics.Endpoint = server.URL
	c.HostMetadata.Enabled = false

	t.Run("true", func(t *testing.T) {
		c.API.FailOnInvalidKey = true
		ctx := context.Background()
		// metrics exporter
		mexp, err := factory.CreateMetrics(
			ctx,
			exportertest.NewNopSettings(metadata.Type),
			cfg,
		)
		assert.EqualError(t, err, "API Key validation failed")
		assert.Nil(t, mexp)

		texp, err := factory.CreateTraces(
			ctx,
			exportertest.NewNopSettings(metadata.Type),
			cfg,
		)
		assert.EqualError(t, err, "API Key validation failed")
		assert.Nil(t, texp)

		lexp, err := factory.CreateLogs(
			ctx,
			exportertest.NewNopSettings(metadata.Type),
			cfg,
		)
		assert.EqualError(t, err, "API Key validation failed")
		assert.Nil(t, lexp)
	})
	t.Run("false", func(t *testing.T) {
		c.API.FailOnInvalidKey = false
		ctx := context.Background()
		exp, err := factory.CreateMetrics(
			ctx,
			exportertest.NewNopSettings(metadata.Type),
			cfg,
		)
		assert.NoError(t, err)
		assert.NotNil(t, exp)

		texp, err := factory.CreateTraces(
			ctx,
			exportertest.NewNopSettings(metadata.Type),
			cfg,
		)
		assert.NoError(t, err)
		assert.NotNil(t, texp)

		lexp, err := factory.CreateLogs(
			ctx,
			exportertest.NewNopSettings(metadata.Type),
			cfg,
		)
		assert.NoError(t, err)
		assert.NotNil(t, lexp)
	})
	featuregateErr = featuregate.GlobalRegistry().Set("exporter.datadogexporter.UseLogsAgentExporter", true)
	assert.NoError(t, featuregateErr)
}

func TestCreateAPILogsExporter(t *testing.T) {
	server := testutil.DatadogLogServerMock()
	defer server.Close()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "api").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	c := cfg.(*datadogconfig.Config)
	c.Metrics.Endpoint = server.URL
	c.HostMetadata.Enabled = false

	ctx := context.Background()
	exp, err := factory.CreateLogs(
		ctx,
		exportertest.NewNopSettings(metadata.Type),
		cfg,
	)

	assert.NoError(t, err)
	assert.NotNil(t, exp)
}

func TestOnlyMetadata(t *testing.T) {
	server := testutil.DatadogServerMock()
	defer server.Close()

	factory := NewFactory()
	ctx := context.Background()
	cfg := &datadogconfig.Config{
		ClientConfig:  defaultClientConfig(),
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
		QueueSettings: exporterhelper.NewDefaultQueueConfig(),

		API:          datadogconfig.APIConfig{Key: "aaaaaaa"},
		Metrics:      datadogconfig.MetricsConfig{TCPAddrConfig: confignet.TCPAddrConfig{Endpoint: server.URL}},
		Traces:       datadogconfig.TracesExporterConfig{TCPAddrConfig: confignet.TCPAddrConfig{Endpoint: server.URL}},
		OnlyMetadata: true,

		HostMetadata: datadogconfig.HostMetadataConfig{
			Enabled:        true,
			ReporterPeriod: 30 * time.Minute,
		},
	}
	cfg.HostMetadata.SetSourceTimeout(50 * time.Millisecond)

	expTraces, err := factory.CreateTraces(
		ctx,
		exportertest.NewNopSettings(metadata.Type),
		cfg,
	)
	assert.NoError(t, err)
	assert.NotNil(t, expTraces)

	expMetrics, err := factory.CreateMetrics(
		ctx,
		exportertest.NewNopSettings(metadata.Type),
		cfg,
	)
	assert.NoError(t, err)
	assert.NotNil(t, expMetrics)

	err = expTraces.Start(ctx, nil)
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, expTraces.Shutdown(ctx))
	}()

	testTraces := ptrace.NewTraces()
	testutil.TestTraces.CopyTo(testTraces)
	err = expTraces.ConsumeTraces(ctx, testTraces)
	require.NoError(t, err)

	recvMetadata := <-server.MetadataChan
	assert.NotEmpty(t, recvMetadata.InternalHostname)
}

func TestStopExporters(t *testing.T) {
	server := testutil.DatadogServerMock()
	defer server.Close()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "api").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	c := cfg.(*datadogconfig.Config)
	c.Metrics.Endpoint = server.URL
	c.HostMetadata.Enabled = false

	ctx := context.Background()
	expTraces, err := factory.CreateTraces(
		ctx,
		exportertest.NewNopSettings(metadata.Type),
		cfg,
	)
	assert.NoError(t, err)
	assert.NotNil(t, expTraces)
	expMetrics, err := factory.CreateMetrics(
		ctx,
		exportertest.NewNopSettings(metadata.Type),
		cfg,
	)
	assert.NoError(t, err)
	assert.NotNil(t, expMetrics)

	err = expTraces.Start(ctx, nil)
	assert.NoError(t, err)
	err = expMetrics.Start(ctx, nil)
	assert.NoError(t, err)

	finishShutdown := make(chan bool)
	go func() {
		err = expMetrics.Shutdown(ctx)
		assert.NoError(t, err)
		err = expTraces.Shutdown(ctx)
		assert.NoError(t, err)
		finishShutdown <- true
	}()

	select {
	case <-finishShutdown:
		break
	case <-time.After(time.Second * 10):
		t.Fatal("Timed out")
	}
}
