// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/receiver/xreceiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver/internal/metadata"
)

func encodingFromReceiver(tb testing.TB, r any, section string) string {
	tb.Helper()

	switch rc := r.(type) {
	case *saramaConsumer:
		switch section {
		case "Traces":
			return rc.config.Traces.Encoding
		case "Metrics":
			return rc.config.Metrics.Encoding
		case "Logs":
			return rc.config.Logs.Encoding
		case "Profiles":
			return rc.config.Profiles.Encoding
		}
	case *franzConsumer:
		switch section {
		case "Traces":
			return rc.config.Traces.Encoding
		case "Metrics":
			return rc.config.Metrics.Encoding
		case "Logs":
			return rc.config.Logs.Encoding
		case "Profiles":
			return rc.config.Profiles.Encoding
		}
	}

	tb.Fatalf("unsupported receiver type %T or section %q", r, section)
	return ""
}

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
	assert.Equal(t, configkafka.NewDefaultClientConfig(), cfg.ClientConfig)
	assert.Equal(t, configkafka.NewDefaultConsumerConfig(), cfg.ConsumerConfig)
}

func TestCreateTraces(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Brokers = []string{"invalid:9092"}
	cfg.ProtocolVersion = "2.0.0"
	r, err := createTracesReceiver(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
	require.NoError(t, err)
	require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()))
	assert.NoError(t, r.Shutdown(t.Context()))
}

func TestWithTracesUnmarshalers(t *testing.T) {
	f := NewFactory()

	t.Run("custom_encoding/sarama", func(t *testing.T) {
		setFranzGo(t, false)
		cfg := createDefaultConfig().(*Config)
		cfg.Traces.Encoding = "custom"
		receiver, err := f.CreateTraces(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		require.NoError(t, err)
		require.NotNil(t, receiver)
		assert.Equal(t, "custom", encodingFromReceiver(t, receiver, "Traces"))
	})

	t.Run("custom_encoding/franzgo", func(t *testing.T) {
		setFranzGo(t, true)
		cfg := createDefaultConfig().(*Config)
		cfg.Traces.Encoding = "custom"
		receiver, err := f.CreateTraces(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		require.NoError(t, err)
		require.NotNil(t, receiver)
		assert.Equal(t, "custom", encodingFromReceiver(t, receiver, "Traces"))
	})

	t.Run("default_encoding/sarama", func(t *testing.T) {
		setFranzGo(t, false)
		cfg := createDefaultConfig()
		receiver, err := f.CreateTraces(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		require.NoError(t, err)
		require.NotNil(t, receiver)
		assert.Equal(t, defaultTracesEncoding, encodingFromReceiver(t, receiver, "Traces"))
	})

	t.Run("default_encoding/franzgo", func(t *testing.T) {
		setFranzGo(t, true)
		cfg := createDefaultConfig()
		receiver, err := f.CreateTraces(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		require.NoError(t, err)
		require.NotNil(t, receiver)
		assert.Equal(t, defaultTracesEncoding, encodingFromReceiver(t, receiver, "Traces"))
	})
}

func TestCreateMetrics(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Brokers = []string{"invalid:9092"}
	cfg.ProtocolVersion = "2.0.0"
	r, err := createMetricsReceiver(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
	require.NoError(t, err)
	require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()))
	assert.NoError(t, r.Shutdown(t.Context()))
}

func TestWithMetricsUnmarshalers(t *testing.T) {
	f := NewFactory()

	t.Run("custom_encoding/sarama", func(t *testing.T) {
		setFranzGo(t, false)
		cfg := createDefaultConfig().(*Config)
		cfg.Metrics.Encoding = "custom"
		receiver, err := f.CreateMetrics(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		require.NoError(t, err)
		require.NotNil(t, receiver)
		assert.Equal(t, "custom", encodingFromReceiver(t, receiver, "Metrics"))
	})

	t.Run("custom_encoding/franzgo", func(t *testing.T) {
		setFranzGo(t, true)
		cfg := createDefaultConfig().(*Config)
		cfg.Metrics.Encoding = "custom"
		receiver, err := f.CreateMetrics(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		require.NoError(t, err)
		require.NotNil(t, receiver)
		assert.Equal(t, "custom", encodingFromReceiver(t, receiver, "Metrics"))
	})

	t.Run("default_encoding/sarama", func(t *testing.T) {
		setFranzGo(t, false)
		cfg := createDefaultConfig()
		receiver, err := f.CreateMetrics(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		require.NoError(t, err)
		require.NotNil(t, receiver)
		assert.Equal(t, defaultMetricsEncoding, encodingFromReceiver(t, receiver, "Metrics"))
	})

	t.Run("default_encoding/franzgo", func(t *testing.T) {
		setFranzGo(t, true)
		cfg := createDefaultConfig()
		receiver, err := f.CreateMetrics(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		require.NoError(t, err)
		require.NotNil(t, receiver)
		assert.Equal(t, defaultMetricsEncoding, encodingFromReceiver(t, receiver, "Metrics"))
	})
}

func TestCreateLogs(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Brokers = []string{"invalid:9092"}
	cfg.ProtocolVersion = "2.0.0"
	r, err := createLogsReceiver(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
	require.NoError(t, err)
	require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()))
	assert.NoError(t, r.Shutdown(t.Context()))
}

func TestWithLogsUnmarshalers(t *testing.T) {
	f := NewFactory()

	t.Run("custom_encoding/sarama", func(t *testing.T) {
		setFranzGo(t, false)
		cfg := createDefaultConfig().(*Config)
		cfg.Logs.Encoding = "custom"
		receiver, err := f.CreateLogs(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		require.NoError(t, err)
		require.NotNil(t, receiver)
		assert.Equal(t, "custom", encodingFromReceiver(t, receiver, "Logs"))
	})

	t.Run("custom_encoding/franzgo", func(t *testing.T) {
		setFranzGo(t, true)
		cfg := createDefaultConfig().(*Config)
		cfg.Logs.Encoding = "custom"
		receiver, err := f.CreateLogs(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		require.NoError(t, err)
		require.NotNil(t, receiver)
		assert.Equal(t, "custom", encodingFromReceiver(t, receiver, "Logs"))
	})

	t.Run("default_encoding/sarama", func(t *testing.T) {
		setFranzGo(t, false)
		cfg := createDefaultConfig()
		receiver, err := f.CreateLogs(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		require.NoError(t, err)
		require.NotNil(t, receiver)
		assert.Equal(t, defaultLogsEncoding, encodingFromReceiver(t, receiver, "Logs"))
	})

	t.Run("default_encoding/franzgo", func(t *testing.T) {
		setFranzGo(t, true)
		cfg := createDefaultConfig()
		receiver, err := f.CreateLogs(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		require.NoError(t, err)
		require.NotNil(t, receiver)
		assert.Equal(t, defaultLogsEncoding, encodingFromReceiver(t, receiver, "Logs"))
	})
}

func TestCreateProfiles(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Brokers = []string{"invalid:9092"}
	cfg.ProtocolVersion = "2.0.0"
	r, err := createProfilesReceiver(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
	require.NoError(t, err)
	require.NoError(t, r.Start(t.Context(), componenttest.NewNopHost()))
	assert.NoError(t, r.Shutdown(t.Context()))
}

func TestWithProfilesUnmarshalers(t *testing.T) {
	f := NewFactory()

	t.Run("custom_encoding/sarama", func(t *testing.T) {
		setFranzGo(t, false)
		cfg := createDefaultConfig().(*Config)
		cfg.Profiles.Encoding = "custom"
		receiver, err := f.(xreceiver.Factory).CreateProfiles(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		require.NoError(t, err)
		require.NotNil(t, receiver)
		assert.Equal(t, "custom", encodingFromReceiver(t, receiver, "Profiles"))
	})

	t.Run("custom_encoding/franzgo", func(t *testing.T) {
		setFranzGo(t, true)
		cfg := createDefaultConfig().(*Config)
		cfg.Profiles.Encoding = "custom"
		receiver, err := f.(xreceiver.Factory).CreateProfiles(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		require.NoError(t, err)
		require.NotNil(t, receiver)
		assert.Equal(t, "custom", encodingFromReceiver(t, receiver, "Profiles"))
	})

	t.Run("default_encoding/sarama", func(t *testing.T) {
		setFranzGo(t, false)
		cfg := createDefaultConfig()
		receiver, err := f.(xreceiver.Factory).CreateProfiles(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		require.NoError(t, err)
		require.NotNil(t, receiver)
		assert.Equal(t, defaultProfilesEncoding, encodingFromReceiver(t, receiver, "Profiles"))
	})

	t.Run("default_encoding/franzgo", func(t *testing.T) {
		setFranzGo(t, true)
		cfg := createDefaultConfig()
		receiver, err := f.(xreceiver.Factory).CreateProfiles(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nil)
		require.NoError(t, err)
		require.NotNil(t, receiver)
		assert.Equal(t, defaultProfilesEncoding, encodingFromReceiver(t, receiver, "Profiles"))
	})
}
