// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/memorylimiterprocessor"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/attraction"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/remotetapprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanprocessor"
)

func TestDefaultProcessors(t *testing.T) {
	t.Parallel()

	allFactories, err := components()
	require.NoError(t, err)

	procFactories := allFactories.Processors

	tests := []struct {
		getConfigFn   getProcessorConfigFn
		processor     component.Type
		skipLifecycle bool
	}{
		{
			processor: "attributes",
			getConfigFn: func() component.Config {
				cfg := procFactories["attributes"].CreateDefaultConfig().(*attributesprocessor.Config)
				cfg.Actions = []attraction.ActionKeyValue{
					{Key: "attribute1", Action: attraction.INSERT, Value: 123},
				}
				return cfg
			},
		},
		{
			processor: "batch",
		},
		{
			processor:     "datadog",
			skipLifecycle: true, // requires external exporters to be configured to route data
		},
		{
			processor: "deltatorate",
		},
		{
			processor: "filter",
		},
		{
			processor: "groupbyattrs",
		},
		{
			processor: "groupbytrace",
		},
		{
			processor:     "k8sattributes",
			skipLifecycle: true, // Requires a k8s API to communicate with
		},
		{
			processor: "memory_limiter",
			getConfigFn: func() component.Config {
				cfg := procFactories["memory_limiter"].CreateDefaultConfig().(*memorylimiterprocessor.Config)
				cfg.CheckInterval = 100 * time.Millisecond
				cfg.MemoryLimitMiB = 1024 * 1024
				return cfg
			},
		},
		{
			processor: "metricstransform",
		},
		{
			processor: "experimental_metricsgeneration",
		},
		{
			processor: "probabilistic_sampler",
		},
		{
			processor: "resourcedetection",
		},
		{
			processor: "resource",
			getConfigFn: func() component.Config {
				cfg := procFactories["resource"].CreateDefaultConfig().(*resourceprocessor.Config)
				cfg.AttributesActions = []attraction.ActionKeyValue{
					{Key: "attribute1", Action: attraction.INSERT, Value: 123},
				}
				return cfg
			},
		},
		{
			processor:     "routing",
			skipLifecycle: true, // Requires external exporters to be configured to route data
		},
		{
			processor: "span",
			getConfigFn: func() component.Config {
				cfg := procFactories["span"].CreateDefaultConfig().(*spanprocessor.Config)
				cfg.Rename.FromAttributes = []string{"test-key"}
				return cfg
			},
		},
		{
			processor:     "servicegraph",
			skipLifecycle: true,
		},
		{
			processor:     "spanmetrics",
			skipLifecycle: true, // Requires a running exporter to convert data to/from
		},
		{
			processor: "cumulativetodelta",
		},
		{
			processor: "tail_sampling",
		},
		{
			processor: "transform",
		},
		{
			processor: "redaction",
		},
		{
			processor: "remotetap",
			getConfigFn: func() component.Config {
				cfg := procFactories["remotetap"].CreateDefaultConfig().(*remotetapprocessor.Config)
				cfg.Endpoint = "localhost:0"
				return cfg
			},
		},
		{
			processor: "sumologic",
		},
	}

	assert.Equal(t, len(procFactories), len(tests), "All processors must be added to lifecycle tests")
	for _, tt := range tests {
		t.Run(string(tt.processor), func(t *testing.T) {
			factory := procFactories[tt.processor]
			assert.Equal(t, tt.processor, factory.Type())

			t.Run("shutdown", func(t *testing.T) {
				verifyProcessorShutdown(t, factory, tt.getConfigFn)
			})
			t.Run("lifecycle", func(t *testing.T) {
				if tt.skipLifecycle {
					t.SkipNow()
				}
				verifyProcessorLifecycle(t, factory, tt.getConfigFn)
			})
		})
	}
}

// getProcessorConfigFn is used customize the configuration passed to the verification.
// This is used to change ports or provide values required but not provided by the
// default configuration.
type getProcessorConfigFn func() component.Config

// verifyProcessorLifecycle is used to test if a processor type can handle the typical
// lifecycle of a component. The getConfigFn parameter only need to be specified if
// the test can't be done with the default configuration for the component.
func verifyProcessorLifecycle(t *testing.T, factory processor.Factory, getConfigFn getProcessorConfigFn) {
	ctx := context.Background()
	host := newAssertNoErrorHost(t)
	processorCreationSet := processortest.NewNopCreateSettings()

	if getConfigFn == nil {
		getConfigFn = factory.CreateDefaultConfig
	}

	createFns := map[component.DataType]createProcessorFn{
		component.DataTypeLogs:    wrapCreateLogsProc(factory),
		component.DataTypeTraces:  wrapCreateTracesProc(factory),
		component.DataTypeMetrics: wrapCreateMetricsProc(factory),
	}

	for i := 0; i < 2; i++ {
		procs := make(map[component.DataType]component.Component)
		for dataType, createFn := range createFns {
			proc, err := createFn(ctx, processorCreationSet, getConfigFn())
			if errors.Is(err, component.ErrDataTypeIsNotSupported) {
				continue
			}
			require.NoError(t, err)
			procs[dataType] = proc
			require.NoError(t, proc.Start(ctx, host))
		}
		for dataType, proc := range procs {
			assert.NotPanics(t, func() {
				switch dataType {
				case component.DataTypeLogs:
					logsProc := proc.(processor.Logs)
					logs := testdata.GenerateLogsManyLogRecordsSameResource(2)
					if !logsProc.Capabilities().MutatesData {
						logs.MarkReadOnly()
					}
					assert.NoError(t, logsProc.ConsumeLogs(ctx, logs))
				case component.DataTypeMetrics:
					metricsProc := proc.(processor.Metrics)
					metrics := testdata.GenerateMetricsTwoMetrics()
					if !metricsProc.Capabilities().MutatesData {
						metrics.MarkReadOnly()
					}
					assert.NoError(t, metricsProc.ConsumeMetrics(ctx, metrics))
				case component.DataTypeTraces:
					tracesProc := proc.(processor.Traces)
					traces := testdata.GenerateTracesTwoSpansSameResource()
					if !tracesProc.Capabilities().MutatesData {
						traces.MarkReadOnly()
					}
					assert.NoError(t, tracesProc.ConsumeTraces(ctx, traces))
				}
			})
			require.NoError(t, proc.Shutdown(ctx))
		}
	}
}

// verifyProcessorShutdown is used to test if a processor type can be shutdown without being started first.
// We disregard errors being returned by shutdown, we're just making sure the processors don't panic.
func verifyProcessorShutdown(tb testing.TB, factory processor.Factory, getConfigFn getProcessorConfigFn) {
	ctx := context.Background()
	processorCreationSet := processortest.NewNopCreateSettings()

	if getConfigFn == nil {
		getConfigFn = factory.CreateDefaultConfig
	}

	createFns := []createProcessorFn{
		wrapCreateLogsProc(factory),
		wrapCreateTracesProc(factory),
		wrapCreateMetricsProc(factory),
	}

	for _, createFn := range createFns {
		p, err := createFn(ctx, processorCreationSet, getConfigFn())
		if errors.Is(err, component.ErrDataTypeIsNotSupported) {
			continue
		}
		if p == nil {
			continue
		}
		assert.NotPanics(tb, func() {
			_ = p.Shutdown(ctx)
		})
	}
}

type createProcessorFn func(
	ctx context.Context,
	set processor.CreateSettings,
	cfg component.Config,
) (component.Component, error)

func wrapCreateLogsProc(factory processor.Factory) createProcessorFn {
	return func(ctx context.Context, set processor.CreateSettings, cfg component.Config) (component.Component, error) {
		return factory.CreateLogsProcessor(ctx, set, cfg, consumertest.NewNop())
	}
}

func wrapCreateMetricsProc(factory processor.Factory) createProcessorFn {
	return func(ctx context.Context, set processor.CreateSettings, cfg component.Config) (component.Component, error) {
		return factory.CreateMetricsProcessor(ctx, set, cfg, consumertest.NewNop())
	}
}

func wrapCreateTracesProc(factory processor.Factory) createProcessorFn {
	return func(ctx context.Context, set processor.CreateSettings, cfg component.Config) (component.Component, error) {
		return factory.CreateTracesProcessor(ctx, set, cfg, consumertest.NewNop())
	}
}
