// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package loadbalancingexporter

import (
	"context"
	"errors"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.uber.org/zap"
)

func TestNewExporterNoResolver(t *testing.T) {
	// prepare
	config := &Config{}
	params := component.ExporterCreateParams{
		Logger: zap.NewNop(),
	}

	// test
	p, err := newExporter(params, config)

	// verify
	require.Nil(t, p)
	require.Equal(t, errNoResolver, err)
}

func TestNewExporterInvalidStaticResolver(t *testing.T) {
	// prepare
	config := &Config{
		Resolver: ResolverSettings{
			Static: &StaticResolver{Hostnames: []string{}},
		},
	}
	params := component.ExporterCreateParams{
		Logger: zap.NewNop(),
	}

	// test
	p, err := newExporter(params, config)

	// verify
	require.Nil(t, p)
	require.Equal(t, errNoEndpoints, err)
}

func TestNewExporterInvalidDNSResolver(t *testing.T) {
	// prepare
	config := &Config{
		Resolver: ResolverSettings{
			DNS: &DNSResolver{
				Hostname: "",
			},
		},
	}
	params := component.ExporterCreateParams{
		Logger: zap.NewNop(),
	}

	// test
	p, err := newExporter(params, config)

	// verify
	require.Nil(t, p)
	require.Equal(t, errNoHostname, err)
}

func TestExporterCapabilities(t *testing.T) {
	// prepare
	config := simpleConfig()
	params := component.ExporterCreateParams{
		Logger: zap.NewNop(),
	}
	p, err := newExporter(params, config)

	// test
	caps := p.GetCapabilities()

	// verify
	assert.NoError(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, false, caps.MutatesConsumedData)
}

func TestStart(t *testing.T) {
	// prepare
	config := simpleConfig()
	params := component.ExporterCreateParams{
		Logger: zap.NewNop(),
	}
	p, err := newExporter(params, config)
	require.NotNil(t, p)
	require.NoError(t, err)
	p.res = &mockResolver{}

	// test
	res := p.Start(context.Background(), componenttest.NewNopHost())
	defer p.Shutdown(context.Background())

	// verify
	assert.Nil(t, res)
}

func TestWithDNSResolver(t *testing.T) {
	params := component.ExporterCreateParams{
		Logger: zap.NewNop(),
	}
	p, err := newExporter(params, &Config{
		Resolver: ResolverSettings{
			DNS: &DNSResolver{
				Hostname: "service-1",
			},
		},
	})
	require.NotNil(t, p)
	require.NoError(t, err)

	// test
	res, ok := p.res.(*dnsResolver)

	// verify
	assert.NotNil(t, res)
	assert.True(t, ok)
}

func TestMultipleResolvers(t *testing.T) {
	params := component.ExporterCreateParams{
		Logger: zap.NewNop(),
	}

	// test
	p, err := newExporter(params, &Config{
		Resolver: ResolverSettings{
			Static: &StaticResolver{
				Hostnames: []string{"endpoint-1", "endpoint-2"},
			},
			DNS: &DNSResolver{
				Hostname: "service-1",
			},
		},
	})

	// verify
	assert.Nil(t, p)
	assert.Equal(t, errMultipleResolversProvided, err)
}

func TestStartFailureStaticResolver(t *testing.T) {
	// prepare
	config := simpleConfig()
	params := component.ExporterCreateParams{
		Logger: zap.NewNop(),
	}
	p, err := newExporter(params, config)
	require.NotNil(t, p)
	require.NoError(t, err)

	expectedErr := errors.New("some expected err")
	p.res = &mockResolver{
		onStart: func(context.Context) error {
			return expectedErr
		},
	}

	// test
	res := p.Start(context.Background(), componenttest.NewNopHost())

	// verify
	assert.Equal(t, expectedErr, res)
}

func TestShutdown(t *testing.T) {
	// prepare
	config := simpleConfig()
	params := component.ExporterCreateParams{
		Logger: zap.NewNop(),
	}
	p, err := newExporter(params, config)
	require.NotNil(t, p)
	require.NoError(t, err)

	// test
	res := p.Shutdown(context.Background())

	// verify
	assert.Nil(t, res)
}

func TestConsumeTraces(t *testing.T) {
	// prepare
	config := simpleConfig()
	params := component.ExporterCreateParams{
		Logger: zap.NewNop(),
	}
	p, err := newExporter(params, config)
	require.NotNil(t, p)
	require.NoError(t, err)

	// pre-load an exporter here, so that we don't use the actual OTLP exporter
	p.exporters["endpoint-1"] = &componenttest.ExampleExporterConsumer{}
	p.res = &mockResolver{
		triggerCallbacks: true,
		onResolve: func(ctx context.Context) ([]string, error) {
			return []string{"endpoint-1"}, nil
		},
	}

	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer p.Shutdown(context.Background())

	// test
	res := p.ConsumeTraces(context.Background(), simpleTraces())

	// verify
	assert.Nil(t, res)
}

func TestOnBackendChanges(t *testing.T) {
	// prepare
	config := simpleConfig()
	params := component.ExporterCreateParams{
		Logger: zap.NewNop(),
	}
	p, err := newExporter(params, config)
	require.NotNil(t, p)
	require.NoError(t, err)

	// test
	p.onBackendChanges([]string{"endpoint-1"})
	require.Len(t, p.ring.items, defaultWeight)

	// this should resolve to two endpoints
	endpoints := []string{"endpoint-1", "endpoint-2"}
	p.onBackendChanges(endpoints)

	// verify
	assert.Len(t, p.ring.items, 2*defaultWeight)
}

func TestRemoveExtraExporters(t *testing.T) {
	// prepare
	config := simpleConfig()
	params := component.ExporterCreateParams{
		Logger: zap.NewNop(),
	}
	p, err := newExporter(params, config)
	require.NotNil(t, p)
	require.NoError(t, err)

	p.exporters["endpoint-1"] = &componenttest.ExampleExporterConsumer{}
	p.exporters["endpoint-2"] = &componenttest.ExampleExporterConsumer{}
	resolved := []string{"endpoint-1"}

	// test
	p.removeExtraExporters(context.Background(), resolved)

	// verify
	assert.Len(t, p.exporters, 1)
	assert.NotContains(t, p.exporters, "endpoint-2")
}

func TestAddMissingExporters(t *testing.T) {
	// prepare
	config := simpleConfig()
	params := component.ExporterCreateParams{
		Logger: zap.NewNop(),
	}
	p, err := newExporter(params, config)
	require.NotNil(t, p)
	require.NoError(t, err)

	p.exporterFactory = exporterhelper.NewFactory("otlp", func() configmodels.Exporter {
		return &otlpexporter.Config{}
	}, exporterhelper.WithTraces(func(
		_ context.Context,
		_ component.ExporterCreateParams,
		_ configmodels.Exporter,
	) (component.TracesExporter, error) {
		return &componenttest.ExampleExporterConsumer{}, nil
	}))

	p.exporters["endpoint-1:55680"] = &componenttest.ExampleExporterConsumer{}
	resolved := []string{"endpoint-1", "endpoint-2"}

	// test
	p.addMissingExporters(context.Background(), resolved)

	// verify
	assert.Len(t, p.exporters, 2)
	assert.Contains(t, p.exporters, "endpoint-2:55680")
}

func TestFailedToAddMissingExporters(t *testing.T) {
	// prepare
	config := simpleConfig()
	params := component.ExporterCreateParams{
		Logger: zap.NewNop(),
	}
	p, err := newExporter(params, config)
	require.NotNil(t, p)
	require.NoError(t, err)

	expectedErr := errors.New("some expected error")
	p.exporterFactory = exporterhelper.NewFactory("otlp", func() configmodels.Exporter {
		return &otlpexporter.Config{}
	}, exporterhelper.WithTraces(func(
		_ context.Context,
		_ component.ExporterCreateParams,
		_ configmodels.Exporter,
	) (component.TracesExporter, error) {
		return nil, expectedErr
	}))

	p.exporters["endpoint-1"] = &componenttest.ExampleExporterConsumer{}
	resolved := []string{"endpoint-1", "endpoint-2"}

	// test
	p.addMissingExporters(context.Background(), resolved)

	// verify
	assert.Len(t, p.exporters, 1)
	assert.NotContains(t, p.exporters, "endpoint-2")
}

func TestEndpointFound(t *testing.T) {
	for _, tt := range []struct {
		endpoint  string
		endpoints []string
		expected  bool
	}{
		{
			"endpoint-1",
			[]string{"endpoint-1", "endpoint-2"},
			true,
		},
		{
			"endpoint-3",
			[]string{"endpoint-1", "endpoint-2"},
			false,
		},
	} {
		assert.Equal(t, tt.expected, endpointFound(tt.endpoint, tt.endpoints))
	}
}

func TestEndpointWithPort(t *testing.T) {
	for _, tt := range []struct {
		input, expected string
	}{
		{
			"endpoint-1",
			"endpoint-1:55680",
		},
		{
			"endpoint-1:55690",
			"endpoint-1:55690",
		},
	} {
		assert.Equal(t, tt.expected, endpointWithPort(tt.input))
	}
}

func TestBuildExporterConfig(t *testing.T) {
	// prepare
	factories, err := componenttest.ExampleComponents()
	require.NoError(t, err)

	factories.Exporters[typeStr] = NewFactory()

	cfg, err := configtest.LoadConfigFile(t, path.Join(".", "testdata", "test-build-exporter-config.yaml"), factories)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.NotNil(t, cfg.Exporters["loadbalancing"])

	params := component.ExporterCreateParams{
		Logger: zap.NewNop(),
	}
	e, err := newExporter(params, cfg.Exporters["loadbalancing"])
	require.NotNil(t, e)
	require.NoError(t, err)

	// test
	defaultCfg := e.exporterFactory.CreateDefaultConfig().(*otlpexporter.Config)
	exporterCfg := e.buildExporterConfig("the-endpoint")

	// verify
	grpcSettings := defaultCfg.GRPCClientSettings
	grpcSettings.Endpoint = "the-endpoint"
	assert.Equal(t, grpcSettings, exporterCfg.GRPCClientSettings)

	assert.Equal(t, defaultCfg.ExporterSettings, exporterCfg.ExporterSettings)
	assert.Equal(t, defaultCfg.TimeoutSettings, exporterCfg.TimeoutSettings)
	assert.Equal(t, defaultCfg.QueueSettings, exporterCfg.QueueSettings)
	assert.Equal(t, defaultCfg.RetrySettings, exporterCfg.RetrySettings)
}

func TestBatchWithTwoTraces(t *testing.T) {
	// prepare
	config := simpleConfig()
	params := component.ExporterCreateParams{
		Logger: zap.NewNop(),
	}
	p, err := newExporter(params, config)
	require.NotNil(t, p)
	require.NoError(t, err)

	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	sink := &componenttest.ExampleExporterConsumer{}
	p.exporters["endpoint-1"] = sink

	first := simpleTraces()
	second := simpleTraceWithID(pdata.NewTraceID([16]byte{2, 3, 4, 5}))
	batch := pdata.NewTraces()
	batch.ResourceSpans().Append(first.ResourceSpans().At(0))
	batch.ResourceSpans().Append(second.ResourceSpans().At(0))

	// test
	err = p.ConsumeTraces(context.Background(), batch)

	// verify
	assert.NoError(t, err)
	assert.Len(t, sink.Traces, 2)
}

func TestFailedExporterInRing(t *testing.T) {
	// this test is based on the discussion in the original PR for this exporter:
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/1542#discussion_r521268180
	// prepare
	config := &Config{
		Resolver: ResolverSettings{
			Static: &StaticResolver{Hostnames: []string{"endpoint-1", "endpoint-2"}},
		},
	}
	params := component.ExporterCreateParams{
		Logger: zap.NewNop(),
	}
	p, err := newExporter(params, config)
	require.NotNil(t, p)
	require.NoError(t, err)

	err = p.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	// simulate the case where one of the exporters failed to be created and do not exist in the internal map
	// this is a case that we are not even sure that might happen, so, this test case is here to document
	// this behavior. As the solution would require more locks/syncs/checks, we should probably wait to see
	// if this is really a problem in the real world
	delete(p.exporters, "endpoint-2")

	// sanity check
	require.Contains(t, p.res.(*staticResolver).endpoints, "endpoint-2")

	// test
	// this trace ID will reach the endpoint-2 -- see the consistent hashing tests for more info
	traceForMissingEndpoint := simpleTraceWithID(pdata.NewTraceID([16]byte{128, 128, 0, 0}))
	err = p.ConsumeTraces(context.Background(), traceForMissingEndpoint)

	// verify
	assert.Error(t, err)
}

func TestNoTracesInBatch(t *testing.T) {
	for _, tt := range []struct {
		desc  string
		batch pdata.Traces
	}{
		{
			"no resource spans",
			pdata.NewTraces(),
		},
		{
			"no instrumentation library spans",
			func() pdata.Traces {
				batch := pdata.NewTraces()
				rs := pdata.NewResourceSpans()
				rs.InitEmpty()
				batch.ResourceSpans().Append(rs)
				return batch
			}(),
		},
		{
			"no spans",
			func() pdata.Traces {
				batch := pdata.NewTraces()
				rs := pdata.NewResourceSpans()
				rs.InitEmpty()
				ils := pdata.NewInstrumentationLibrarySpans()
				ils.InitEmpty()
				rs.InstrumentationLibrarySpans().Append(ils)
				batch.ResourceSpans().Append(rs)
				return batch
			}(),
		},
	} {
		t.Run(tt.desc, func(t *testing.T) {
			res := traceIDFromTraces(tt.batch)
			assert.Equal(t, pdata.InvalidTraceID(), res)
		})
	}
}

func simpleTraces() pdata.Traces {
	return simpleTraceWithID(pdata.NewTraceID([16]byte{1, 2, 3, 4}))
}

func simpleTraceWithID(id pdata.TraceID) pdata.Traces {
	traces := pdata.NewTraces()
	rss := pdata.NewResourceSpans()
	rss.InitEmpty()

	ils := pdata.NewInstrumentationLibrarySpans()
	ils.InitEmpty()

	span := pdata.NewSpan()
	span.InitEmpty()
	span.SetTraceID(id)

	ils.Spans().Append(span)
	rss.InstrumentationLibrarySpans().Append(ils)
	traces.ResourceSpans().Append(rss)

	return traces
}

func simpleConfig() *Config {
	return &Config{
		Resolver: ResolverSettings{
			Static: &StaticResolver{Hostnames: []string{"endpoint-1"}},
		},
	}
}
