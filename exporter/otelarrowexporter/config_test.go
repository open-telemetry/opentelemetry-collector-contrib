// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelarrowexporter

import (
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/open-telemetry/otel-arrow/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configauth"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

func TestUnmarshalDefaultConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "default.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NoError(t, component.UnmarshalConfig(cm, cfg))
	assert.Equal(t, factory.CreateDefaultConfig(), cfg)
	assert.Equal(t, "round_robin", cfg.(*Config).GRPCClientSettings.BalancerName)
}

func TestUnmarshalConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NoError(t, component.UnmarshalConfig(cm, cfg))
	assert.Equal(t,
		&Config{
			TimeoutSettings: exporterhelper.TimeoutSettings{
				Timeout: 10 * time.Second,
			},
			RetrySettings: exporterhelper.RetrySettings{
				Enabled:             true,
				InitialInterval:     10 * time.Second,
				RandomizationFactor: 0.7,
				Multiplier:          1.3,
				MaxInterval:         1 * time.Minute,
				MaxElapsedTime:      10 * time.Minute,
			},
			QueueSettings: exporterhelper.QueueSettings{
				Enabled:      true,
				NumConsumers: 2,
				QueueSize:    10,
			},
			GRPCClientSettings: configgrpc.GRPCClientSettings{
				Headers: map[string]configopaque.String{
					"can you have a . here?": "F0000000-0000-0000-0000-000000000000",
					"header1":                "234",
					"another":                "somevalue",
				},
				Endpoint:    "1.2.3.4:1234",
				Compression: "none",
				TLSSetting: configtls.TLSClientSetting{
					TLSSetting: configtls.TLSSetting{
						CAFile: "/var/lib/mycert.pem",
					},
					Insecure: false,
				},
				Keepalive: &configgrpc.KeepaliveClientConfig{
					Time:                20 * time.Second,
					PermitWithoutStream: true,
					Timeout:             30 * time.Second,
				},
				WriteBufferSize: 512 * 1024,
				BalancerName:    "experimental",
				Auth:            &configauth.Authentication{AuthenticatorID: component.NewID("nop")},
			},
			Arrow: ArrowSettings{
				NumStreams:         2,
				EnableMixedSignals: true,
				MaxStreamLifetime:  2 * time.Hour,
				PayloadCompression: configcompression.Zstd,
			},
		}, cfg)
}

func TestArrowSettingsValidate(t *testing.T) {
	settings := func(enabled bool, numStreams int, maxStreamLifetime time.Duration) *ArrowSettings {
		return &ArrowSettings{Disabled: !enabled, NumStreams: numStreams, MaxStreamLifetime: maxStreamLifetime}
	}
	require.NoError(t, settings(true, 1, 10*time.Second).Validate())
	require.NoError(t, settings(false, 1, 10*time.Second).Validate())
	require.NoError(t, settings(true, 2, 1*time.Second).Validate())
	require.NoError(t, settings(true, math.MaxInt, 10*time.Second).Validate())

	require.Error(t, settings(true, 0, 10*time.Second).Validate())
	require.Contains(t, settings(true, 0, 10*time.Second).Validate().Error(), "stream count must be")
	require.Contains(t, settings(true, 1, -1*time.Second).Validate().Error(), "max stream life must be")
	require.Error(t, settings(false, -1, 10*time.Second).Validate())
	require.Error(t, settings(false, 1, -1*time.Second).Validate())
	require.Error(t, settings(true, math.MinInt, 10*time.Second).Validate())
}

func TestDefaultSettingsValid(t *testing.T) {
	cfg := createDefaultConfig()
	// this must be set by the user and config
	// validation always checks that a value is set.
	cfg.(*Config).Arrow.MaxStreamLifetime = 2 * time.Second
	require.NoError(t, cfg.(*Config).Validate())
}

func TestArrowSettingsPayloadCompressionZstd(t *testing.T) {
	settings := ArrowSettings{
		PayloadCompression: configcompression.Zstd,
	}
	var config config.Config
	for _, opt := range settings.ToArrowProducerOptions() {
		opt(&config)
	}
	require.True(t, config.Zstd)
}

func TestArrowSettingsPayloadCompressionNone(t *testing.T) {
	for _, value := range []string{"", "none"} {
		settings := ArrowSettings{
			PayloadCompression: configcompression.CompressionType(value),
		}
		var config config.Config
		for _, opt := range settings.ToArrowProducerOptions() {
			opt(&config)
		}
		require.False(t, config.Zstd)
	}
}
