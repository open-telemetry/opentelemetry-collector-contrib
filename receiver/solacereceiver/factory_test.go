// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package solacereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver"

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver/internal/metadata"
)

func TestCreateTraces(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "primary").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	set := receivertest.NewNopSettings()
	set.ID = component.MustNewIDWithName("solace", "factory")
	receiver, err := factory.CreateTraces(
		context.Background(),
		set,
		cfg,
		consumertest.NewNop(),
	)
	assert.NoError(t, err)
	castedReceiver, ok := receiver.(*solaceTracesReceiver)
	assert.True(t, ok)
	assert.Equal(t, castedReceiver.config, cfg)
}

func TestCreateTracesWrongConfig(t *testing.T) {
	factory := NewFactory()
	_, err := factory.CreateTraces(context.Background(), receivertest.NewNopSettings(), nil, nil)
	assert.Equal(t, pipeline.ErrSignalNotSupported, err)
}

func TestCreateTracesBadConfigNoAuth(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Queue = "some-queue"
	factory := NewFactory()
	_, err := factory.CreateTraces(context.Background(), receivertest.NewNopSettings(), cfg, consumertest.NewNop())
	assert.Equal(t, errMissingAuthDetails, err)
}

func TestCreateTracesBadConfigIncompleteAuth(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Queue = "some-queue"
	cfg.Auth = Authentication{PlainText: &SaslPlainTextConfig{Username: "someUsername"}} // missing password
	factory := NewFactory()
	_, err := factory.CreateTraces(context.Background(), receivertest.NewNopSettings(), cfg, consumertest.NewNop())
	assert.Equal(t, errMissingPlainTextParams, err)
}

func TestCreateTracesBadMetrics(t *testing.T) {
	set := receivertest.NewNopSettings()
	set.ID = component.MustNewIDWithName("solace", "factory")
	// the code here sets up a custom meter provider
	// to trigger the error condition required for this test
	metricExp, err := stdoutmetric.New()
	require.NoError(t, err)
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp)),
		sdkmetric.WithView(sdkmetric.NewView(
			sdkmetric.Instrument{
				Name: "otelcol_solacereceiver_failed_reconnections",
			},
			sdkmetric.Stream{
				Aggregation: sdkmetric.AggregationLastValue{},
			},
		)),
	)
	defer func() {
		require.NoError(t, provider.Shutdown(context.Background()))
	}()
	set.MeterProvider = provider
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "primary").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))
	receiver, err := factory.CreateTraces(
		context.Background(),
		set,
		cfg,
		consumertest.NewNop(),
	)
	assert.Error(t, err)
	assert.Nil(t, receiver)
}
