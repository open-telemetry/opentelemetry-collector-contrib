// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lookupprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/lookupsource"
)

func testLookupConfig() LookupConfig {
	return LookupConfig{
		Key: `log.attributes["test.key"]`,
		Attributes: []AttributeMapping{
			{Destination: "test.result"},
		},
	}
}

func TestNewFactory(t *testing.T) {
	factory := NewFactory()

	assert.Equal(t, metadata.Type, factory.Type())

	cfg := factory.CreateDefaultConfig()
	require.NotNil(t, cfg)
	require.IsType(t, &Config{}, cfg)

	// Default source type should be noop
	assert.Equal(t, "noop", cfg.(*Config).Source.Type)
}

func TestNewFactoryWithOptions(t *testing.T) {
	mockFactory := lookupsource.NewSourceFactory(
		"mock",
		func() lookupsource.SourceConfig { return &mockSourceConfig{} },
		func(_ context.Context, _ lookupsource.CreateSettings, _ lookupsource.SourceConfig) (lookupsource.Source, error) {
			return lookupsource.NewSource(
				func(_ context.Context, key string) (any, bool, error) {
					return "mocked-" + key, true, nil
				},
				func() string { return "mock" },
				nil,
				nil,
			), nil
		},
	)

	factory := NewFactoryWithOptions(WithSources(mockFactory))

	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Source.Type = "mock"
	cfg.Lookups = []LookupConfig{testLookupConfig()}

	settings := processortest.NewNopSettings(metadata.Type)
	sink := consumertest.NewNop()

	proc, err := factory.CreateLogs(t.Context(), settings, cfg, sink)
	require.NoError(t, err)
	require.NotNil(t, proc)

	host := componenttest.NewNopHost()
	require.NoError(t, proc.Start(t.Context(), host))
	require.NoError(t, proc.Shutdown(t.Context()))
}

func TestFactoryCreatesLogsProcessor(t *testing.T) {
	factory := NewFactory()

	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Lookups = []LookupConfig{testLookupConfig()}

	settings := processortest.NewNopSettings(metadata.Type)

	proc, err := factory.CreateLogs(t.Context(), settings, cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, proc)

	host := componenttest.NewNopHost()
	require.NoError(t, proc.Start(t.Context(), host))
	require.NoError(t, proc.Shutdown(t.Context()))
}

func TestFactoryRejectsTracesProcessor(t *testing.T) {
	factory := NewFactory()

	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Lookups = []LookupConfig{testLookupConfig()}
	settings := processortest.NewNopSettings(metadata.Type)

	_, err := factory.CreateTraces(t.Context(), settings, cfg, consumertest.NewNop())
	require.Error(t, err)
}

func TestFactoryRejectsMetricsProcessor(t *testing.T) {
	factory := NewFactory()

	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Lookups = []LookupConfig{testLookupConfig()}
	settings := processortest.NewNopSettings(metadata.Type)

	_, err := factory.CreateMetrics(t.Context(), settings, cfg, consumertest.NewNop())
	require.Error(t, err)
}

func TestFactoryUnknownSourceType(t *testing.T) {
	factory := NewFactory()

	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Source.Type = "unknown"
	cfg.Lookups = []LookupConfig{testLookupConfig()}

	settings := processortest.NewNopSettings(metadata.Type)

	_, err := factory.CreateLogs(t.Context(), settings, cfg, consumertest.NewNop())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown source type")
}

func TestWithSourcesReplacesDefaults(t *testing.T) {
	customFactory := lookupsource.NewSourceFactory(
		"custom",
		func() lookupsource.SourceConfig { return &mockSourceConfig{} },
		func(_ context.Context, _ lookupsource.CreateSettings, _ lookupsource.SourceConfig) (lookupsource.Source, error) {
			return lookupsource.NewSource(
				func(_ context.Context, _ string) (any, bool, error) {
					return nil, false, nil
				},
				func() string { return "custom" },
				nil,
				nil,
			), nil
		},
	)

	// WithSources should replace default sources (noop)
	factory := NewFactoryWithOptions(WithSources(customFactory))

	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Lookups = []LookupConfig{testLookupConfig()}

	// noop should not be available anymore
	cfg.Source.Type = "noop"

	settings := processortest.NewNopSettings(metadata.Type)
	_, err := factory.CreateLogs(t.Context(), settings, cfg, consumertest.NewNop())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown source type")

	// custom should be available
	cfg.Source.Type = "custom"
	proc, err := factory.CreateLogs(t.Context(), settings, cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, proc)
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr string
	}{
		{
			name:    "no lookups",
			cfg:     &Config{},
			wantErr: "at least one lookup",
		},
		{
			name: "missing key",
			cfg: &Config{
				Lookups: []LookupConfig{{
					Attributes: []AttributeMapping{{Destination: "test"}},
				}},
			},
			wantErr: "key is required",
		},
		{
			name: "missing attributes",
			cfg: &Config{
				Lookups: []LookupConfig{{Key: `log.attributes["test"]`}},
			},
			wantErr: "at least one attribute mapping",
		},
		{
			name: "missing destination",
			cfg: &Config{
				Lookups: []LookupConfig{{
					Key:        `log.attributes["test"]`,
					Attributes: []AttributeMapping{{Source: "field"}},
				}},
			},
			wantErr: "destination is required",
		},
		{
			name: "valid config",
			cfg: &Config{
				Source: SourceConfig{Type: "noop"},
				Lookups: []LookupConfig{{
					Key: `log.attributes["test.key"]`,
					Attributes: []AttributeMapping{
						{Destination: "test.result"},
					},
				}},
			},
			wantErr: "",
		},
		{
			name: "valid config with all options",
			cfg: &Config{
				Source: SourceConfig{Type: "noop"},
				Lookups: []LookupConfig{{
					Key:     `log.attributes["user.id"]`,
					Context: ContextRecord,
					Attributes: []AttributeMapping{
						{
							Source:      "name",
							Destination: "user.name",
							Default:     "Unknown",
						},
						{
							Source:      "role",
							Destination: "user.role",
							Context:     ContextResource,
						},
					},
				}},
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestContextIDUnmarshalText(t *testing.T) {
	tests := []struct {
		input   string
		want    ContextID
		wantErr bool
	}{
		{"record", ContextRecord, false},
		{"RECORD", ContextRecord, false},
		{"Record", ContextRecord, false},
		{"resource", ContextResource, false},
		{"RESOURCE", ContextResource, false},
		{"invalid", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			var c ContextID
			err := c.UnmarshalText([]byte(tt.input))
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "invalid context")
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, c)
			}
		})
	}
}

func TestInvalidKeyExpression(t *testing.T) {
	factory := NewFactory()

	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Lookups = []LookupConfig{{
		Key: `invalid!!!expression`,
		Attributes: []AttributeMapping{
			{Destination: "test.result"},
		},
	}}

	settings := processortest.NewNopSettings(metadata.Type)
	_, err := factory.CreateLogs(t.Context(), settings, cfg, consumertest.NewNop())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse key expression")
}
