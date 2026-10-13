// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package geoipprocessor

import (
	"context"
	"errors"
	"maps"
	"net/netip"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetricassert"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
	conventions "github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor/internal/convention"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor/internal/provider"
)

type providerConfigMock struct {
	ValidateF func() error
}

type providerFactoryMock struct {
	CreateDefaultConfigF func() provider.Config
	CreateGeoIPProviderF func(context.Context, processor.Settings, provider.Config) (provider.GeoIPProvider, error)
}

type providerMock struct {
	LocationF func(context.Context, netip.Addr) (attribute.Set, error)
	CloseF    func(context.Context) error
}

var (
	_ provider.GeoIPProvider        = (*providerMock)(nil)
	_ provider.GeoIPProviderFactory = (*providerFactoryMock)(nil)
)

func (cm *providerConfigMock) Validate() error {
	return cm.ValidateF()
}

func (fm *providerFactoryMock) CreateDefaultConfig() provider.Config {
	return fm.CreateDefaultConfigF()
}

func (fm *providerFactoryMock) CreateGeoIPProvider(ctx context.Context, settings processor.Settings, cfg provider.Config) (provider.GeoIPProvider, error) {
	return fm.CreateGeoIPProviderF(ctx, settings, cfg)
}

func (pm *providerMock) Location(ctx context.Context, ip netip.Addr) (attribute.Set, error) {
	return pm.LocationF(ctx, ip)
}

func (pm *providerMock) Close(ctx context.Context) error {
	return pm.CloseF(ctx)
}

var baseMockProvider = providerMock{
	LocationF: func(context.Context, netip.Addr) (attribute.Set, error) {
		return attribute.Set{}, nil
	},
	CloseF: func(context.Context) error {
		return nil
	},
}

var baseMockFactory = providerFactoryMock{
	CreateDefaultConfigF: func() provider.Config {
		return &providerConfigMock{ValidateF: func() error { return nil }}
	},
	CreateGeoIPProviderF: func(context.Context, processor.Settings, provider.Config) (provider.GeoIPProvider, error) {
		return &baseMockProvider, nil
	},
}

// newFactoryWithMocks returns a processor.Factory whose default Config carries the built-in
// provider factories plus the supplied extras. Tests that route through otelcoltest or
// otherwise rely on factory.CreateDefaultConfig() to construct the Config use this helper
// to inject mock providers without mutating package-level state.
func newFactoryWithMocks(extras map[string]provider.GeoIPProviderFactory) processor.Factory {
	createCfg := func() component.Config {
		factories := make(map[string]provider.GeoIPProviderFactory, len(defaultProviderFactories)+len(extras))
		maps.Copy(factories, defaultProviderFactories)
		maps.Copy(factories, extras)
		return &Config{
			Context:           resource,
			Attributes:        defaultAttributes,
			ErrorMode:         ottl.PropagateError,
			providerFactories: factories,
		}
	}
	return processor.NewFactory(metadata.Type, createCfg,
		processor.WithMetrics(createMetricsProcessor, metadata.MetricsStability),
		processor.WithLogs(createLogsProcessor, metadata.LogsStability),
		processor.WithTraces(createTracesProcessor, metadata.TracesStability),
	)
}

var testCases = []struct {
	name       string
	goldenDir  string
	context    ContextID
	attributes []attribute.Key
	errorMode  ottl.ErrorMode
}{
	{
		name:      "default source.address attribute, not found",
		goldenDir: "resource_no_source_address",
		context:   resource,
		errorMode: ottl.IgnoreError,
	},
	{
		name:      "default source.address attribute",
		goldenDir: "resource_source_address",
		context:   resource,
	},
	{
		name:      "default source.address attribute no geo metadata found by providers",
		goldenDir: "resource_source_address_geo_not_found",
		context:   resource,
	},
	{
		name:      "default source.ip attribute with an unspecified IP address should be skipped",
		goldenDir: "resource_unspecified_address",
		context:   resource,
		errorMode: ottl.IgnoreError,
	},
	{
		name:      "do not add resource attributes with an invalid ip",
		goldenDir: "resource_invalid_address",
		context:   resource,
		errorMode: ottl.IgnoreError,
	},
	{
		name:      "source address located in the record attributes",
		goldenDir: "record_source_address",
		context:   record,
	},
	{
		name:      "client address located in the record attributes",
		goldenDir: "record_client_address",
		context:   record,
	},
	{
		name:       "custom address located in the record attributes",
		goldenDir:  "record_custom_address",
		context:    record,
		attributes: []attribute.Key{"source.address", "client.address", "custom.address"},
	},
}

func compareAllSignals(cfg component.Config, goldenDir string) func(t *testing.T) {
	return func(t *testing.T) {
		dir := filepath.Join("testdata", goldenDir)
		factory := NewFactory()

		// compare metrics
		nextMetrics := new(consumertest.MetricsSink)
		metricsProcessor, err := factory.CreateMetrics(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, nextMetrics)
		require.NoError(t, err)

		inputMetrics, err := golden.ReadMetrics(filepath.Join(dir, "input-metrics.yaml"))
		require.NoError(t, err)

		err = metricsProcessor.ConsumeMetrics(t.Context(), inputMetrics)
		require.NoError(t, err)
		require.NoError(t, metricsProcessor.Shutdown(t.Context()))

		actualMetrics := nextMetrics.AllMetrics()
		require.Len(t, actualMetrics, 1)

		require.NoError(t, pmetricassert.AssertMetrics(filepath.Join(dir, "output-metrics.assert.yaml"), actualMetrics[0]))

		// compare traces
		nextTraces := new(consumertest.TracesSink)
		tracesProcessor, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, nextTraces)
		require.NoError(t, err)

		inputTraces, err := golden.ReadTraces(filepath.Join(dir, "input-traces.yaml"))
		require.NoError(t, err)

		expectedTraces, err := golden.ReadTraces(filepath.Join(dir, "output-traces.yaml"))
		require.NoError(t, err)

		err = tracesProcessor.ConsumeTraces(t.Context(), inputTraces)
		require.NoError(t, err)
		require.NoError(t, tracesProcessor.Shutdown(t.Context()))

		actualTraces := nextTraces.AllTraces()
		require.Len(t, actualTraces, 1)
		// golden.WriteTraces(t, filepath.Join(dir, "output-traces.yaml"), actualTraces[0])
		require.NoError(t, ptracetest.CompareTraces(expectedTraces, actualTraces[0]))

		// compare logs
		nextLogs := new(consumertest.LogsSink)
		logsProcessor, err := factory.CreateLogs(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, nextLogs)
		require.NoError(t, err)

		inputLogs, err := golden.ReadLogs(filepath.Join(dir, "input-logs.yaml"))
		require.NoError(t, err)

		err = logsProcessor.ConsumeLogs(t.Context(), inputLogs)
		require.NoError(t, err)

		expectedLogs, err := golden.ReadLogs(filepath.Join(dir, "output-logs.yaml"))
		require.NoError(t, err)

		actualLogs := nextLogs.AllLogs()
		require.Len(t, actualLogs, 1)
		// golden.WriteLogs(t, filepath.Join(dir, "output-logs.yaml"), actualLogs[0])
		require.NoError(t, plogtest.CompareLogs(expectedLogs, actualLogs[0]))
		require.NoError(t, logsProcessor.Shutdown(t.Context()))
	}
}

func TestProcessor(t *testing.T) {
	t.Parallel()

	mockFactory := providerFactoryMock{
		CreateDefaultConfigF: func() provider.Config {
			return &providerConfigMock{ValidateF: func() error { return nil }}
		},
		CreateGeoIPProviderF: func(context.Context, processor.Settings, provider.Config) (provider.GeoIPProvider, error) {
			return &providerMock{
				LocationF: func(_ context.Context, sourceIP netip.Addr) (attribute.Set, error) {
					if sourceIP.Compare(netip.AddrFrom4([4]byte{1, 2, 3, 4})) == 0 {
						return attribute.NewSet([]attribute.KeyValue{
							attribute.String(conventions.AttributeGeoCityName, "Boxford"),
							attribute.String(conventions.AttributeGeoContinentCode, "EU"),
							attribute.String(conventions.AttributeGeoContinentName, "Europe"),
							attribute.String(conventions.AttributeGeoCountryIsoCode, "GB"),
							attribute.String(conventions.AttributeGeoCountryName, "United Kingdom"),
							attribute.String(conventions.AttributeGeoTimezone, "Europe/London"),
							attribute.String(conventions.AttributeGeoRegionIsoCode, "WBK"),
							attribute.String(conventions.AttributeGeoRegionName, "West Berkshire"),
							attribute.String(conventions.AttributeGeoPostalCode, "OX1"),
							attribute.Float64(conventions.AttributeGeoLocationLat, 1234),
							attribute.Float64(conventions.AttributeGeoLocationLon, 5678),
						}...), nil
					}
					return attribute.Set{}, provider.ErrNoMetadataFound
				},
				CloseF: func(context.Context) error { return nil },
			}, nil
		},
	}
	const providerKey string = "mock"
	factories := map[string]provider.GeoIPProviderFactory{providerKey: &mockFactory}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			attributes := defaultAttributes
			if tt.attributes != nil {
				attributes = tt.attributes
			}
			errorMode := tt.errorMode
			if errorMode == "" {
				errorMode = ottl.PropagateError
			}
			cfg := &Config{
				Context:           tt.context,
				Providers:         map[string]provider.Config{providerKey: &providerConfigMock{}},
				Attributes:        attributes,
				ErrorMode:         errorMode,
				providerFactories: factories,
			}
			compareAllSignals(cfg, tt.goldenDir)(t)
		})
	}
}

func TestProcessorShutdownError(t *testing.T) {
	// processor with two mocked providers that return error on close
	processor := geoIPProcessor{
		providers: []provider.GeoIPProvider{
			&providerMock{
				CloseF: func(context.Context) error {
					return errors.New("test error 1")
				},
			},
			&providerMock{
				CloseF: func(context.Context) error {
					return errors.New("test error 2")
				},
			},
		},
	}

	assert.EqualError(t, processor.shutdown(t.Context()), "test error 1; test error 2")
}

func TestProcessor_ProcessAttributes_ErrorMode(t *testing.T) {
	tests := []struct {
		name            string
		errorMode       ottl.ErrorMode
		attributes      pcommon.Map
		expectErr       bool
		errCheck        func(error) bool
		expectLogsDebug int
		expectLogsWarn  int
		expectedLogMsg  string
	}{
		// propagate error mode
		{
			name:       "propagate missing IP attribute",
			errorMode:  ottl.PropagateError,
			attributes: pcommon.NewMap(),
			expectErr:  true,
			errCheck: func(err error) bool {
				return errors.Is(err, errIPNotFound)
			},
		},
		{
			name:      "propagate invalid IP",
			errorMode: ottl.PropagateError,
			attributes: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("source.address", "1.2.3.999")
				return m
			}(),
			expectErr: true,
		},
		{
			name:      "propagate domain name",
			errorMode: ottl.PropagateError,
			attributes: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("source.address", "example.com")
				return m
			}(),
			expectErr: true,
		},
		{
			name:      "propagate unspecified IP",
			errorMode: ottl.PropagateError,
			attributes: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("source.address", "0.0.0.0")
				return m
			}(),
			expectErr: true,
			errCheck: func(err error) bool {
				return errors.Is(err, errUnspecifiedIP)
			},
		},
		// ignore error mode
		{
			name:            "ignore missing IP attribute",
			errorMode:       ottl.IgnoreError,
			attributes:      pcommon.NewMap(),
			expectErr:       false,
			expectLogsDebug: 1,
			expectedLogMsg:  "failed to find IP address, ignoring",
		},
		{
			name:      "ignore invalid IP",
			errorMode: ottl.IgnoreError,
			attributes: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("source.address", "1.2.3.999")
				return m
			}(),
			expectErr:      false,
			expectLogsWarn: 1,
			expectedLogMsg: "failed to parse IP address, ignoring",
		},
		{
			name:      "ignore domain name",
			errorMode: ottl.IgnoreError,
			attributes: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("source.address", "example.com")
				return m
			}(),
			expectErr:      false,
			expectLogsWarn: 1,
			expectedLogMsg: "failed to parse IP address, ignoring",
		},
		{
			name:      "ignore unspecified IP",
			errorMode: ottl.IgnoreError,
			attributes: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("source.address", "0.0.0.0")
				return m
			}(),
			expectErr:      false,
			expectLogsWarn: 1,
			expectedLogMsg: "failed to parse IP address, ignoring",
		},
		// silent error mode
		{
			name:       "silent missing IP attribute",
			errorMode:  ottl.SilentError,
			attributes: pcommon.NewMap(),
			expectErr:  false,
		},
		{
			name:      "silent invalid IP",
			errorMode: ottl.SilentError,
			attributes: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("source.address", "1.2.3.999")
				return m
			}(),
			expectErr: false,
		},
		{
			name:      "silent domain name",
			errorMode: ottl.SilentError,
			attributes: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("source.address", "example.com")
				return m
			}(),
			expectErr: false,
		},
		{
			name:      "silent unspecified IP",
			errorMode: ottl.SilentError,
			attributes: func() pcommon.Map {
				m := pcommon.NewMap()
				m.PutStr("source.address", "0.0.0.0")
				return m
			}(),
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core, recorded := observer.New(zap.DebugLevel)
			logger := zap.New(core)
			proc := &geoIPProcessor{
				cfg: &Config{
					Attributes: defaultAttributes,
					ErrorMode:  tt.errorMode,
				},
				logger: logger,
			}

			err := proc.processAttributes(t.Context(), tt.attributes)
			if tt.expectErr {
				require.Error(t, err)
				if tt.errCheck != nil {
					assert.True(t, tt.errCheck(err))
				}
			} else {
				require.NoError(t, err)
			}

			if tt.expectLogsDebug > 0 {
				debugLogs := recorded.FilterLevelExact(zap.DebugLevel)
				require.Equal(t, tt.expectLogsDebug, debugLogs.Len())
				assert.Equal(t, tt.expectedLogMsg, debugLogs.All()[0].Message)
			}
			if tt.expectLogsWarn > 0 {
				warnLogs := recorded.FilterLevelExact(zap.WarnLevel)
				require.Equal(t, tt.expectLogsWarn, warnLogs.Len())
				assert.Equal(t, tt.expectedLogMsg, warnLogs.All()[0].Message)
			}
			if tt.errorMode == ottl.SilentError {
				assert.Equal(t, 0, recorded.Len())
			}
		})
	}
}

func TestProcessor_Signals_ErrorMode(t *testing.T) {
	for _, errorMode := range []ottl.ErrorMode{ottl.PropagateError, ottl.IgnoreError, ottl.SilentError} {
		t.Run(string(errorMode), func(t *testing.T) {
			factory := NewFactory()
			cfg := &Config{
				Context:    resource,
				Providers:  map[string]provider.Config{"mock": &providerConfigMock{}},
				Attributes: defaultAttributes,
				ErrorMode:  errorMode,
				providerFactories: map[string]provider.GeoIPProviderFactory{
					"mock": &baseMockFactory,
				},
			}

			core, recorded := observer.New(zap.DebugLevel)
			settings := processortest.NewNopSettings(metadata.Type)
			settings.Logger = zap.New(core)

			// Test Logs
			logsSink := new(consumertest.LogsSink)
			logsProc, err := factory.CreateLogs(t.Context(), settings, cfg, logsSink)
			require.NoError(t, err)

			ld := plog.NewLogs()
			rl := ld.ResourceLogs().AppendEmpty()
			rl.Resource().Attributes().PutStr("source.address", "invalid-ip-or-domain")

			err = logsProc.ConsumeLogs(t.Context(), ld)
			if errorMode == ottl.PropagateError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				if errorMode == ottl.IgnoreError {
					assert.Equal(t, 1, recorded.FilterLevelExact(zap.WarnLevel).Len())
				} else {
					assert.Equal(t, 0, recorded.Len())
				}
			}
		})
	}
}
