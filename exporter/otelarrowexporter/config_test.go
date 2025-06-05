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
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/otelarrowexporter/internal/arrow"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/compression/zstd"
)

func TestUnmarshalDefaultConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "default.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NoError(t, cm.Unmarshal(cfg))
	assert.Equal(t, factory.CreateDefaultConfig(), cfg)
	assert.Equal(t, "round_robin", cfg.(*Config).BalancerName)
	assert.Equal(t, arrow.DefaultPrioritizer, cfg.(*Config).Arrow.Prioritizer)
}

func TestUnmarshalConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NoError(t, cm.Unmarshal(cfg))
	assert.Equal(t,
		&Config{
			TimeoutSettings: exporterhelper.TimeoutConfig{
				Timeout: 10 * time.Second,
			},
			RetryConfig: configretry.BackOffConfig{
				Enabled:             true,
				InitialInterval:     10 * time.Second,
				RandomizationFactor: 0.7,
				Multiplier:          1.3,
				MaxInterval:         1 * time.Minute,
				MaxElapsedTime:      10 * time.Minute,
			},
			QueueSettings: exporterhelper.QueueBatchConfig{
				Enabled:      true,
				NumConsumers: 2,
				QueueSize:    10,
				Sizer:        exporterhelper.RequestSizerTypeRequests,
			},
			ClientConfig: configgrpc.ClientConfig{
				Headers: map[string]configopaque.String{
					"can you have a . here?": "F0000000-0000-0000-0000-000000000000",
					"header1":                "234",
					"another":                "somevalue",
				},
				Endpoint:    "1.2.3.4:1234",
				Compression: "none",
				TLS: configtls.ClientConfig{
					Config: configtls.Config{
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
				Auth:            &configauth.Config{AuthenticatorID: component.NewID(component.MustNewType("nop"))},
			},
			BatcherConfig: exporterhelper.BatcherConfig{ //nolint:staticcheck
				Enabled:      true,
				FlushTimeout: 200 * time.Millisecond,
				SizeConfig: exporterhelper.SizeConfig{ //nolint:staticcheck
					Sizer:   exporterhelper.RequestSizerTypeItems,
					MinSize: 1000,
					MaxSize: 10000,
				},
			},
			Arrow: ArrowConfig{
				NumStreams:         2,
				MaxStreamLifetime:  2 * time.Hour,
				PayloadCompression: configcompression.TypeZstd,
				Zstd:               zstd.DefaultEncoderConfig(),
				Prioritizer:        "leastloaded8",
			},
		}, cfg)
}

func TestArrowConfigValidate(t *testing.T) {
	settings := func(enabled bool, numStreams int, maxStreamLifetime time.Duration, level zstd.Level) *ArrowConfig {
		return &ArrowConfig{
			Disabled:          !enabled,
			NumStreams:        numStreams,
			MaxStreamLifetime: maxStreamLifetime,
			Zstd: zstd.EncoderConfig{
				Level: level,
			},
		}
	}
	require.NoError(t, settings(true, 1, 10*time.Second, zstd.DefaultLevel).Validate())
	require.NoError(t, settings(false, 1, 10*time.Second, zstd.DefaultLevel).Validate())
	require.NoError(t, settings(true, 2, 1*time.Second, zstd.DefaultLevel).Validate())
	require.NoError(t, settings(true, math.MaxInt, 10*time.Second, zstd.DefaultLevel).Validate())
	require.NoError(t, settings(true, math.MaxInt, 10*time.Second, zstd.MaxLevel).Validate())
	require.NoError(t, settings(true, math.MaxInt, 10*time.Second, zstd.MinLevel).Validate())

	require.Error(t, settings(true, 0, 10*time.Second, zstd.DefaultLevel).Validate())
	require.Contains(t, settings(true, 0, 10*time.Second, zstd.DefaultLevel).Validate().Error(), "stream count must be")
	require.Contains(t, settings(true, 1, -1*time.Second, zstd.DefaultLevel).Validate().Error(), "max stream life must be")
	require.Error(t, settings(false, -1, 10*time.Second, zstd.DefaultLevel).Validate())
	require.Error(t, settings(false, 1, -1*time.Second, zstd.DefaultLevel).Validate())
	require.Error(t, settings(true, math.MinInt, 10*time.Second, zstd.DefaultLevel).Validate())
	require.Error(t, settings(true, math.MaxInt, 10*time.Second, zstd.MinLevel-1).Validate())
	require.Error(t, settings(true, math.MaxInt, 10*time.Second, zstd.MaxLevel+1).Validate())
}

func TestDefaultConfigValid(t *testing.T) {
	cfg := createDefaultConfig()
	// this must be set by the user and config
	// validation always checks that a value is set.
	cfg.(*Config).Arrow.MaxStreamLifetime = 2 * time.Second
	require.NoError(t, xconfmap.Validate(cfg))
}

func TestArrowConfigPayloadCompressionZstd(t *testing.T) {
	settings := ArrowConfig{
		PayloadCompression: configcompression.TypeZstd,
	}
	var config config.Config
	for _, opt := range settings.toArrowProducerOptions() {
		opt(&config)
	}
	require.True(t, config.Zstd)
}

func TestArrowConfigPayloadCompressionNone(t *testing.T) {
	for _, value := range []string{"", "none"} {
		settings := ArrowConfig{
			PayloadCompression: configcompression.Type(value),
		}
		var config config.Config
		for _, opt := range settings.toArrowProducerOptions() {
			opt(&config)
		}
		require.False(t, config.Zstd)
	}
}
