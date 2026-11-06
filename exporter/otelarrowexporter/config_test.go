// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelarrowexporter

import (
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/open-telemetry/otel-arrow/go/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configauth"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"
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
	assert.Equal(t, "round_robin", cfg.(*Config).ClientConfig.BalancerName)
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
			QueueSettings: configoptional.Some(func() exporterhelper.QueueBatchConfig {
				queue := exporterhelper.NewDefaultQueueConfig()
				queue.NumConsumers = 2
				queue.QueueSize = 10
				queue.Sizer = exporterhelper.RequestSizerTypeItems
				queue.BlockOnOverflow = true
				batch := queue.Batch.GetOrInsertDefault()
				batch.FlushTimeout = 200 * time.Millisecond
				batch.Sizer = exporterhelper.RequestSizerTypeItems
				batch.MinSize = 1000
				batch.MaxSize = 10000
				return queue
			}()),
			ClientConfig: configgrpc.ClientConfig{
				Headers: configopaque.MapList{
					{Name: "another", Value: "somevalue"},
					{Name: "can you have a . here?", Value: "F0000000-0000-0000-0000-000000000000"},
					{Name: "header1", Value: "234"},
				},
				Endpoint:    "1.2.3.4:1234",
				Compression: "none",
				TLS: configtls.ClientConfig{
					Config: configtls.Config{
						CAFile: "/var/lib/mycert.pem",
					},
					Insecure: false,
				},
				Keepalive: configoptional.Some(configgrpc.KeepaliveClientConfig{
					Time:                20 * time.Second,
					PermitWithoutStream: true,
					Timeout:             30 * time.Second,
				}),
				WriteBufferSize: 512 * 1024,
				BalancerName:    "experimental",
				Auth:            configoptional.Some(configauth.Config{AuthenticatorID: component.NewID(component.MustNewType("nop"))}),
			},
			Arrow: ArrowConfig{
				StreamRestartBackOff: configretry.BackOffConfig{
					Enabled:             true,
					InitialInterval:     250 * time.Millisecond,
					RandomizationFactor: 0.2,
					Multiplier:          2,
					MaxInterval:         8 * time.Second,
				},
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
			StreamRestartBackOff: arrow.NewDefaultStreamRestartConfig(),
			Disabled:             !enabled,
			NumStreams:           numStreams,
			MaxStreamLifetime:    maxStreamLifetime,
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
	require.NoError(t, confmap.Validate(cfg))
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

func TestStreamRestartConfigValidate(t *testing.T) {
	for _, tc := range []struct {
		name      string
		change    func(*configretry.BackOffConfig)
		wantError string
	}{
		{"disabled", func(c *configretry.BackOffConfig) { c.Enabled = false }, "enabled must be true"},
		{"elapsed limit", func(c *configretry.BackOffConfig) { c.MaxElapsedTime = time.Minute }, "max_elapsed_time must be 0"},
		{"negative elapsed limit", func(c *configretry.BackOffConfig) { c.MaxElapsedTime = -time.Second }, "max_elapsed_time"},
		{"zero initial", func(c *configretry.BackOffConfig) { c.InitialInterval = 0 }, "initial_interval must be > 0"},
		{"negative initial", func(c *configretry.BackOffConfig) { c.InitialInterval = -time.Second }, "initial_interval"},
		{"max below initial", func(c *configretry.BackOffConfig) { c.MaxInterval = time.Millisecond }, "max_interval"},
		{"decreasing multiplier", func(c *configretry.BackOffConfig) { c.Multiplier = 0.5 }, "multiplier"},
		{"infinite multiplier", func(c *configretry.BackOffConfig) { c.Multiplier = math.Inf(1) }, "multiplier"},
		{"nan multiplier", func(c *configretry.BackOffConfig) { c.Multiplier = math.NaN() }, "multiplier"},
		{"negative randomization", func(c *configretry.BackOffConfig) { c.RandomizationFactor = -0.1 }, "randomization_factor"},
		{"large randomization", func(c *configretry.BackOffConfig) { c.RandomizationFactor = 1.1 }, "randomization_factor"},
		{"nan randomization", func(c *configretry.BackOffConfig) { c.RandomizationFactor = math.NaN() }, "randomization_factor"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			tc.change(&cfg.Arrow.StreamRestartBackOff)
			require.ErrorContains(t, confmap.Validate(cfg), tc.wantError)
		})
	}
}

func TestUnmarshalStreamRestartConfig(t *testing.T) {
	for _, tc := range []struct {
		name      string
		values    map[string]any
		wantError string
	}{
		{"default", map[string]any{}, ""},
		{"continuous", map[string]any{"enabled": true, "max_elapsed_time": "0s"}, ""},
		{"disabled", map[string]any{"enabled": false}, "enabled must be true"},
		{"elapsed limit", map[string]any{"max_elapsed_time": "1m"}, "max_elapsed_time must be 0"},
		{"standard validation", map[string]any{"randomization_factor": 2}, "randomization_factor"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cm := confmap.NewFromStringMap(map[string]any{
				"endpoint": "localhost:4317",
				"arrow":    map[string]any{"stream_restart_backoff": tc.values},
			})
			require.NoError(t, cm.Unmarshal(cfg))
			if tc.wantError != "" {
				require.ErrorContains(t, confmap.Validate(cfg), tc.wantError)
				return
			}
			require.NoError(t, confmap.Validate(cfg))
			require.Equal(t, arrow.NewDefaultStreamRestartConfig(), cfg.Arrow.StreamRestartBackOff)
		})
	}
}
