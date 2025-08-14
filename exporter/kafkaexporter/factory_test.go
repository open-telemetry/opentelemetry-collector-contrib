// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/exporter/xexporter"
	"go.opentelemetry.io/collector/pdata/testdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
)

// applyConfigOption is used to modify values of the
// the default exporter config to make it easier to
// use the return in a test table set up
func applyConfigOption(option func(conf *Config)) *Config {
	conf := createDefaultConfig().(*Config)
	option(conf)
	return conf
}

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
	assert.Equal(t, configkafka.NewDefaultClientConfig(), cfg.ClientConfig)
	assert.Empty(t, cfg.Topic)
}

func TestCreateMetricExporter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		conf *Config
		err  *net.DNSError
	}{
		{
			name: "valid config (no validating broker)",
			conf: applyConfigOption(func(conf *Config) {
				// this disables contacting the broker so
				// we can successfully create the exporter
				conf.Metadata.Full = false
				conf.Brokers = []string{"invalid:9092"}
				conf.ProtocolVersion = "2.0.0"
			}),
			err: nil,
		},
		{
			name: "invalid config (validating broker)",
			conf: applyConfigOption(func(conf *Config) {
				conf.Brokers = []string{"invalid:9092"}
				conf.ProtocolVersion = "2.0.0"
			}),
			err: &net.DNSError{},
		},
		{
			name: "default_encoding",
			conf: applyConfigOption(func(conf *Config) {
				// Disabling broker check to ensure encoding work
				conf.Metadata.Full = false
				conf.Encoding = "otlp_proto"
			}),
			err: nil,
		},
		{
			name: "with include metadata keys and partitioner",
			conf: applyConfigOption(func(conf *Config) {
				// Disabling broker check
				conf.Metadata.Full = false
				conf.IncludeMetadataKeys = []string{"k1", "k2"}
				conf.QueueBatchConfig.Batch = configoptional.Some(exporterhelper.BatchConfig{
					Sizer: exporterhelper.RequestSizerTypeBytes,
				})
			}),
			err: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := NewFactory()
			exporter, err := f.CreateMetrics(
				t.Context(),
				exportertest.NewNopSettings(metadata.Type),
				tc.conf,
			)
			require.NoError(t, err)
			assert.NotNil(t, exporter, "Must return valid exporter")
			err = exporter.Start(t.Context(), componenttest.NewNopHost())
			if tc.err != nil {
				assert.ErrorAs(t, err, &tc.err, "Must match the expected error")
				return
			}
			assert.NoError(t, err, "Must not error")
			assert.NotNil(t, exporter, "Must return valid exporter when no error is returned")
			assert.NoError(t, exporter.ConsumeMetrics(t.Context(), testdata.GenerateMetrics(2)))
			assert.NoError(t, exporter.Shutdown(t.Context()))
		})
	}
}

func TestCreateLogExporter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		conf *Config
		err  *net.DNSError
	}{
		{
			name: "valid config (no validating broker)",
			conf: applyConfigOption(func(conf *Config) {
				// this disables contacting the broker so
				// we can successfully create the exporter
				conf.Metadata.Full = false
				conf.Brokers = []string{"invalid:9092"}
				conf.ProtocolVersion = "2.0.0"
			}),
			err: nil,
		},
		{
			name: "invalid config (validating broker)",
			conf: applyConfigOption(func(conf *Config) {
				conf.Brokers = []string{"invalid:9092"}
				conf.ProtocolVersion = "2.0.0"
			}),
			err: &net.DNSError{},
		},
		{
			name: "default_encoding",
			conf: applyConfigOption(func(conf *Config) {
				// Disabling broker check to ensure encoding work
				conf.Metadata.Full = false
				conf.Encoding = "otlp_proto"
			}),
			err: nil,
		},
		{
			name: "with include metadata keys and partitioner",
			conf: applyConfigOption(func(conf *Config) {
				// Disabling broker check
				conf.Metadata.Full = false
				conf.IncludeMetadataKeys = []string{"k1", "k2"}
				conf.QueueBatchConfig.Batch = configoptional.Some(exporterhelper.BatchConfig{
					Sizer: exporterhelper.RequestSizerTypeBytes,
				})
			}),
			err: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := NewFactory()
			exporter, err := f.CreateLogs(
				t.Context(),
				exportertest.NewNopSettings(metadata.Type),
				tc.conf,
			)
			require.NoError(t, err)
			assert.NotNil(t, exporter, "Must return valid exporter")
			err = exporter.Start(t.Context(), componenttest.NewNopHost())
			if tc.err != nil {
				assert.ErrorAs(t, err, &tc.err, "Must match the expected error")
				return
			}
			assert.NoError(t, err, "Must not error")
			assert.NotNil(t, exporter, "Must return valid exporter when no error is returned")
			assert.NoError(t, exporter.ConsumeLogs(t.Context(), testdata.GenerateLogs(2)))
			assert.NoError(t, exporter.Shutdown(t.Context()))
		})
	}
}

func TestCreateTraceExporter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		conf *Config
		err  *net.DNSError
	}{
		{
			name: "valid config (no validating brokers)",
			conf: applyConfigOption(func(conf *Config) {
				conf.Metadata.Full = false
				conf.Brokers = []string{"invalid:9092"}
				conf.ProtocolVersion = "2.0.0"
			}),
			err: nil,
		},
		{
			name: "invalid config (validating brokers)",
			conf: applyConfigOption(func(conf *Config) {
				conf.Brokers = []string{"invalid:9092"}
				conf.ProtocolVersion = "2.0.0"
			}),
			err: &net.DNSError{},
		},
		{
			name: "default_encoding",
			conf: applyConfigOption(func(conf *Config) {
				// Disabling broker check to ensure encoding work
				conf.Metadata.Full = false
				conf.Encoding = "otlp_proto"
			}),
			err: nil,
		},
		{
			name: "with include metadata keys and partitioner",
			conf: applyConfigOption(func(conf *Config) {
				// Disabling broker check
				conf.Metadata.Full = false
				conf.IncludeMetadataKeys = []string{"k1", "k2"}
				conf.QueueBatchConfig.Batch = configoptional.Some(exporterhelper.BatchConfig{
					Sizer: exporterhelper.RequestSizerTypeBytes,
				})
			}),
			err: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := NewFactory()
			exporter, err := f.CreateTraces(
				t.Context(),
				exportertest.NewNopSettings(metadata.Type),
				tc.conf,
			)
			require.NoError(t, err)
			assert.NotNil(t, exporter, "Must return valid exporter")
			err = exporter.Start(t.Context(), componenttest.NewNopHost())
			if tc.err != nil {
				assert.ErrorAs(t, err, &tc.err, "Must match the expected error")
				return
			}
			assert.NoError(t, err, "Must not error")
			assert.NotNil(t, exporter, "Must return valid exporter when no error is returned")
			assert.NoError(t, exporter.ConsumeTraces(t.Context(), testdata.GenerateTraces(2)))
			assert.NoError(t, exporter.Shutdown(t.Context()))
		})
	}
}

func TestCreateProfileExporter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		conf *Config
		err  *net.DNSError
	}{
		{
			name: "valid config (no validating broker)",
			conf: applyConfigOption(func(conf *Config) {
				// this disables contacting the broker so
				// we can successfully create the exporter
				conf.Metadata.Full = false
				conf.Brokers = []string{"invalid:9092"}
				conf.ProtocolVersion = "2.0.0"
			}),
			err: nil,
		},
		{
			name: "invalid config (validating broker)",
			conf: applyConfigOption(func(conf *Config) {
				conf.Brokers = []string{"invalid:9092"}
				conf.ProtocolVersion = "2.0.0"
			}),
			err: &net.DNSError{},
		},
		{
			name: "default_encoding",
			conf: applyConfigOption(func(conf *Config) {
				// Disabling broker check to ensure encoding work
				conf.Metadata.Full = false
				conf.Encoding = "otlp_proto"
			}),
			err: nil,
		},
		{
			name: "with include metadata keys and partitioner",
			conf: applyConfigOption(func(conf *Config) {
				// Disabling broker check
				conf.Metadata.Full = false
				conf.IncludeMetadataKeys = []string{"k1", "k2"}
				conf.QueueBatchConfig.Batch = configoptional.Some(exporterhelper.BatchConfig{
					Sizer: exporterhelper.RequestSizerTypeBytes,
				})
			}),
			err: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := NewFactory().(xexporter.Factory)
			exporter, err := f.CreateProfiles(
				t.Context(),
				exportertest.NewNopSettings(metadata.Type),
				tc.conf,
			)
			require.NoError(t, err)
			assert.NotNil(t, exporter, "Must return valid exporter")
			err = exporter.Start(t.Context(), componenttest.NewNopHost())
			if tc.err != nil {
				assert.ErrorAs(t, err, &tc.err, "Must match the expected error")
				return
			}
			assert.NoError(t, err, "Must not error")
			assert.NotNil(t, exporter, "Must return valid exporter when no error is returned")
			assert.NoError(t, exporter.ConsumeProfiles(t.Context(), testdata.GenerateProfiles(2)))
			assert.NoError(t, exporter.Shutdown(t.Context()))
		})
	}
}
