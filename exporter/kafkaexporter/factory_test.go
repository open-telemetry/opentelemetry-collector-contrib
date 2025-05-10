// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/exporter/exportertest"
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
	assert.Equal(t, []string{defaultBroker}, cfg.Brokers)
	assert.Equal(t, "", cfg.Topic)
	assert.Equal(t, "sarama", cfg.ClientID)
}

func TestCreateMetricExporter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		conf       *Config
		marshalers []MetricsMarshaler
		err        *net.DNSError
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
				conf.Encoding = defaultEncoding
			}),
			marshalers: nil,
			err:        nil,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := NewFactory()
			exporter, err := f.CreateMetrics(
				context.Background(),
				exportertest.NewNopSettings(),
				tc.conf,
			)
			require.NoError(t, err)
			assert.NotNil(t, exporter, "Must return valid exporter")
			err = exporter.Start(context.Background(), componenttest.NewNopHost())
			if tc.err != nil {
				assert.ErrorAs(t, err, &tc.err, "Must match the expected error")
				return
			}
			assert.NoError(t, err, "Must not error")
			assert.NotNil(t, exporter, "Must return valid exporter when no error is returned")
			assert.NoError(t, exporter.Shutdown(context.Background()))
		})
	}
}

func TestCreateLogExporter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		conf       *Config
		marshalers []LogsMarshaler
		err        *net.DNSError
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
				conf.Encoding = defaultEncoding
			}),
			marshalers: nil,
			err:        nil,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := NewFactory()
			exporter, err := f.CreateLogs(
				context.Background(),
				exportertest.NewNopSettings(),
				tc.conf,
			)
			require.NoError(t, err)
			assert.NotNil(t, exporter, "Must return valid exporter")
			err = exporter.Start(context.Background(), componenttest.NewNopHost())
			if tc.err != nil {
				assert.ErrorAs(t, err, &tc.err, "Must match the expected error")
				return
			}
			assert.NoError(t, err, "Must not error")
			assert.NotNil(t, exporter, "Must return valid exporter when no error is returned")
			assert.NoError(t, exporter.Shutdown(context.Background()))
		})
	}
}

func TestCreateTraceExporter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		conf       *Config
		marshalers []TracesMarshaler
		err        *net.DNSError
	}{
		{
			name: "valid config (no validating brokers)",
			conf: applyConfigOption(func(conf *Config) {
				conf.Metadata.Full = false
				conf.Brokers = []string{"invalid:9092"}
				conf.ProtocolVersion = "2.0.0"
			}),
			marshalers: nil,
			err:        nil,
		},
		{
			name: "invalid config (validating brokers)",
			conf: applyConfigOption(func(conf *Config) {
				conf.Brokers = []string{"invalid:9092"}
				conf.ProtocolVersion = "2.0.0"
			}),
			marshalers: nil,
			err:        &net.DNSError{},
		},
		{
			name: "default_encoding",
			conf: applyConfigOption(func(conf *Config) {
				// Disabling broker check to ensure encoding work
				conf.Metadata.Full = false
				conf.Encoding = defaultEncoding
			}),
			marshalers: nil,
			err:        nil,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := NewFactory()
			exporter, err := f.CreateTraces(
				context.Background(),
				exportertest.NewNopSettings(),
				tc.conf,
			)
			require.NoError(t, err)
			assert.NotNil(t, exporter, "Must return valid exporter")
			err = exporter.Start(context.Background(), componenttest.NewNopHost())
			if tc.err != nil {
				assert.ErrorAs(t, err, &tc.err, "Must match the expected error")
				return
			}
			assert.NoError(t, err, "Must not error")
			assert.NotNil(t, exporter, "Must return valid exporter when no error is returned")
			assert.NoError(t, exporter.Shutdown(context.Background()))
		})
	}
}
