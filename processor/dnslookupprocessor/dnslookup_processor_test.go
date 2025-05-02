// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dnslookupprocessor

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/dnslookupprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/dnslookupprocessor/internal/testutil"
)

func TestProcessor(t *testing.T) {
	testCases := []struct {
		name      string
		goldenDir string
		resolve   LookupConfig
		reverse   LookupConfig
	}{
		{
			name:      "resolve source.address and reverse custom.ip",
			goldenDir: "normal",
			resolve:   defaultResolve(),
			reverse:   customReverse(),
		},
		{
			name:      "attributes not found",
			goldenDir: "attr_not_found",
			resolve:   defaultResolve(),
			reverse:   customReverse(),
		},
		{
			name:      "attributes are empty",
			goldenDir: "attr_empty",
			resolve:   defaultResolve(),
			reverse:   customReverse(),
		},
		{
			name:      "take the first valid attribute",
			goldenDir: "multiple_attrs",
			resolve: LookupConfig{
				Enabled:           true,
				Context:           resource,
				Attributes:        []string{"bad.address", "good.address"},
				ResolvedAttribute: "resolved.ip",
			},
			reverse: LookupConfig{
				Enabled:           true,
				Context:           resource,
				Attributes:        []string{"bad.ip", "good.ip"},
				ResolvedAttribute: "resolved.address",
			},
		},
		{
			name:      "attributes has no resolution",
			goldenDir: "no_resolution",
			resolve:   defaultResolve(),
			reverse:   customReverse(),
		},
		{
			name:      "custom resolve record attributes",
			goldenDir: "custom_resolve_attr",
			resolve: LookupConfig{
				Enabled:           true,
				Context:           record,
				Attributes:        []string{"custom.address", "custom.another.address"},
				ResolvedAttribute: "custom.ip",
			},
			reverse: defaultReverse(),
		},
		{
			name:      "custom reverse record attributes",
			goldenDir: "custom_reverse_attr",
			resolve:   defaultResolve(),
			reverse: LookupConfig{
				Enabled:           true,
				Context:           record,
				Attributes:        []string{"custom.ip", "custom.another.ip"},
				ResolvedAttribute: "custom.address",
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createNonExpiryHostsConfig(t, tt.resolve, tt.reverse)
			compareAllSignals(cfg, tt.goldenDir)(t)
		})
	}
}

func TestProcessorError(t *testing.T) {
	testCases := []struct {
		name      string
		goldenDir string
		resolve   LookupConfig
		reverse   LookupConfig
	}{
		{
			name:      "resolver returns error",
			goldenDir: "resolver_error",
			resolve:   defaultResolve(),
			reverse:   defaultReverse(),
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createNonExpiryNSConfig(tt.resolve, tt.reverse)

			dir := filepath.Join("testdata", tt.goldenDir)
			factory := NewFactory()

			// compare metrics
			metricsSink := new(consumertest.MetricsSink)
			metricsProcessor, err := factory.CreateMetrics(context.Background(), processortest.NewNopSettings(metadata.Type), cfg, metricsSink)
			require.NoError(t, err)

			inputMetrics, err := golden.ReadMetrics(filepath.Join(dir, "input-metrics.yaml"))
			require.NoError(t, err)
			expectedMetrics, err := golden.ReadMetrics(filepath.Join(dir, "output-metrics.yaml"))
			require.NoError(t, err)

			// non existent nameserver gives no such host error
			err = metricsProcessor.ConsumeMetrics(context.Background(), inputMetrics)
			require.Error(t, err)
			// consume again gives no error as the lookup result stored in miss cache
			err = metricsProcessor.ConsumeMetrics(context.Background(), inputMetrics)
			require.NoError(t, err)
			require.NoError(t, metricsProcessor.Shutdown(context.Background()))

			actualMetrics := metricsSink.AllMetrics()
			require.Len(t, actualMetrics, 1)
			// golden.WriteMetrics(t, filepath.Join(dir, "output-metrics.yaml"), actualMetrics[0])
			require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics[0]))

			// compare traces
			tracesSink := new(consumertest.TracesSink)
			tracesProcessor, err := factory.CreateTraces(context.Background(), processortest.NewNopSettings(metadata.Type), cfg, tracesSink)
			require.NoError(t, err)

			inputTraces, err := golden.ReadTraces(filepath.Join(dir, "input-traces.yaml"))
			require.NoError(t, err)
			expectedTraces, err := golden.ReadTraces(filepath.Join(dir, "output-traces.yaml"))
			require.NoError(t, err)

			// non existent nameserver gives no such host error
			err = tracesProcessor.ConsumeTraces(context.Background(), inputTraces)
			require.Error(t, err)
			// consume again gives no error as the lookup result stored in miss cache
			err = tracesProcessor.ConsumeTraces(context.Background(), inputTraces)
			require.NoError(t, err)
			require.NoError(t, tracesProcessor.Shutdown(context.Background()))

			actualTraces := tracesSink.AllTraces()
			require.Len(t, actualTraces, 1)
			// golden.WriteTraces(t, filepath.Join(dir, "output-traces.yaml"), actualTraces[0])
			require.NoError(t, ptracetest.CompareTraces(expectedTraces, actualTraces[0]))

			// compare logs
			logsSink := new(consumertest.LogsSink)
			logsProcessor, err := factory.CreateLogs(context.Background(), processortest.NewNopSettings(metadata.Type), cfg, logsSink)
			require.NoError(t, err)

			inputLogs, err := golden.ReadLogs(filepath.Join(dir, "input-logs.yaml"))
			require.NoError(t, err)
			expectedLogs, err := golden.ReadLogs(filepath.Join(dir, "output-logs.yaml"))
			require.NoError(t, err)

			// non existent nameserver gives no such host error
			err = logsProcessor.ConsumeLogs(context.Background(), inputLogs)
			require.Error(t, err)
			// consume again gives no error as the lookup result stored in miss cache
			err = logsProcessor.ConsumeLogs(context.Background(), inputLogs)
			require.NoError(t, err)

			actualLogs := logsSink.AllLogs()
			require.Len(t, actualLogs, 1)
			// golden.WriteLogs(t, filepath.Join(dir, "output-logs.yaml"), actualLogs[0])
			require.NoError(t, plogtest.CompareLogs(expectedLogs, actualLogs[0]))
			require.NoError(t, logsProcessor.Shutdown(context.Background()))
		})
	}
}

func TestProcessorShutdownError(t *testing.T) {
	errorMock1 := new(testutil.MockResolver)
	errorMock1.On("Close").Return(errors.New("error 1")).Once()

	processor := &dnsLookupProcessor{
		resolver: errorMock1,
	}

	assert.EqualError(t, processor.shutdown(context.Background()), "error 1")
}

func TestCreateResolverChain(t *testing.T) {
	logger := zaptest.NewLogger(t)

	tests := []struct {
		name           string
		config         *Config
		expectError    bool
		errorSubstring string
	}{
		{
			name: "one resolver configured",
			config: &Config{
				EnableSystemResolver: true,
				HitCacheSize:         0,
				MissCacheSize:        0,
			},
			expectError: false,
		},
		{
			name: "No resolvers configured",
			config: &Config{
				EnableSystemResolver: false,
				HitCacheSize:         0,
				MissCacheSize:        0,
			},
			expectError:    true,
			errorSubstring: "no DNS resolver configuration available",
		},
		{
			name: "Hostfile error",
			config: &Config{
				EnableSystemResolver: false,
				Hostfiles:            []string{"/nonexistent/path"},
				HitCacheSize:         0,
				MissCacheSize:        0,
			},
			expectError:    true,
			errorSubstring: "failed to create hostfile resolver",
		},
		{
			name: "Nameserver error",
			config: &Config{
				EnableSystemResolver: false,
				Nameservers:          []string{"invalid:nameserver"},
				HitCacheSize:         0,
				MissCacheSize:        0,
			},
			expectError:    true,
			errorSubstring: "failed to create nameserver resolver",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := createResolverChain(tt.config, logger)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, r)
				if tt.errorSubstring != "" {
					assert.Contains(t, err.Error(), tt.errorSubstring)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func defaultResolve() LookupConfig {
	return LookupConfig{
		Enabled:           true,
		Context:           resource,
		Attributes:        []string{"source.address"},
		ResolvedAttribute: "source.ip",
	}
}

func defaultReverse() LookupConfig {
	return LookupConfig{
		Enabled:           false,
		Context:           resource,
		Attributes:        []string{"source.ip"},
		ResolvedAttribute: "source.address",
	}
}

func customReverse() LookupConfig {
	return LookupConfig{
		Enabled:           true,
		Context:           resource,
		Attributes:        []string{"custom.ip", "custom.another.ip"},
		ResolvedAttribute: "custom.address",
	}
}

func createNonExpiryHostsConfig(t *testing.T, resolve LookupConfig, reverse LookupConfig) component.Config {
	const hostsContent = `
192.168.1.20 example.com
192.168.1.30 another.example.com
`
	hostFilePath := testutil.CreateTempHostFile(t, hostsContent)

	return &Config{
		Resolve:              resolve,
		Reverse:              reverse,
		HitCacheSize:         10000,
		HitCacheTTL:          0,
		MissCacheSize:        1000,
		MissCacheTTL:         0,
		MaxRetries:           1,
		Timeout:              0.5,
		Hostfiles:            []string{hostFilePath},
		EnableSystemResolver: false,
	}
}

func createNonExpiryNSConfig(resolve LookupConfig, reverse LookupConfig) component.Config {
	return &Config{
		Resolve:              resolve,
		Reverse:              reverse,
		HitCacheSize:         100,
		HitCacheTTL:          0,
		MissCacheSize:        100,
		MissCacheTTL:         0,
		MaxRetries:           1,
		Timeout:              0.5,
		Nameservers:          []string{"non-exist-nameserver.net"},
		EnableSystemResolver: false,
	}
}

func compareAllSignals(cfg component.Config, goldenDir string) func(t *testing.T) {
	return func(t *testing.T) {
		dir := filepath.Join("testdata", goldenDir)
		factory := NewFactory()

		// compare metrics
		metricsSink := new(consumertest.MetricsSink)
		metricsProcessor, err := factory.CreateMetrics(context.Background(), processortest.NewNopSettings(metadata.Type), cfg, metricsSink)
		require.NoError(t, err)

		inputMetrics, err := golden.ReadMetrics(filepath.Join(dir, "input-metrics.yaml"))
		require.NoError(t, err)
		expectedMetrics, err := golden.ReadMetrics(filepath.Join(dir, "output-metrics.yaml"))
		require.NoError(t, err)

		err = metricsProcessor.ConsumeMetrics(context.Background(), inputMetrics)
		require.NoError(t, err)
		require.NoError(t, metricsProcessor.Shutdown(context.Background()))

		actualMetrics := metricsSink.AllMetrics()
		require.Len(t, actualMetrics, 1)
		// golden.WriteMetrics(t, filepath.Join(dir, "output-metrics.yaml"), actualMetrics[0])
		require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics[0]))

		// compare traces
		tracesSink := new(consumertest.TracesSink)
		tracesProcessor, err := factory.CreateTraces(context.Background(), processortest.NewNopSettings(metadata.Type), cfg, tracesSink)
		require.NoError(t, err)

		inputTraces, err := golden.ReadTraces(filepath.Join(dir, "input-traces.yaml"))
		require.NoError(t, err)
		expectedTraces, err := golden.ReadTraces(filepath.Join(dir, "output-traces.yaml"))
		require.NoError(t, err)

		err = tracesProcessor.ConsumeTraces(context.Background(), inputTraces)
		require.NoError(t, err)
		require.NoError(t, tracesProcessor.Shutdown(context.Background()))

		actualTraces := tracesSink.AllTraces()
		require.Len(t, actualTraces, 1)
		// golden.WriteTraces(t, filepath.Join(dir, "output-traces.yaml"), actualTraces[0])
		require.NoError(t, ptracetest.CompareTraces(expectedTraces, actualTraces[0]))

		// compare logs
		logsSink := new(consumertest.LogsSink)
		logsProcessor, err := factory.CreateLogs(context.Background(), processortest.NewNopSettings(metadata.Type), cfg, logsSink)
		require.NoError(t, err)

		inputLogs, err := golden.ReadLogs(filepath.Join(dir, "input-logs.yaml"))
		require.NoError(t, err)
		expectedLogs, err := golden.ReadLogs(filepath.Join(dir, "output-logs.yaml"))
		require.NoError(t, err)

		err = logsProcessor.ConsumeLogs(context.Background(), inputLogs)
		require.NoError(t, err)

		actualLogs := logsSink.AllLogs()
		require.Len(t, actualLogs, 1)
		// golden.WriteLogs(t, filepath.Join(dir, "output-logs.yaml"), actualLogs[0])
		require.NoError(t, plogtest.CompareLogs(expectedLogs, actualLogs[0]))
		require.NoError(t, logsProcessor.Shutdown(context.Background()))
	}
}
