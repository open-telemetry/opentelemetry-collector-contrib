// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package geoipprocessor

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.25.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor/internal/provider"
)

type ProviderMock struct {
	LocationF func(context.Context, net.IP) (attribute.Set, error)
}

var _ provider.GeoIPProvider = (*ProviderMock)(nil)

var baseProviderMock = ProviderMock{
	LocationF: func(context.Context, net.IP) (attribute.Set, error) {
		return attribute.Set{}, nil
	},
}

func (pm *ProviderMock) Location(ctx context.Context, ip net.IP) (attribute.Set, error) {
	return pm.LocationF(ctx, ip)
}

type generateResourceFunc func(res pcommon.Resource)

func generateTraces(resourceFunc ...generateResourceFunc) ptrace.Traces {
	t := ptrace.NewTraces()
	rs := t.ResourceSpans().AppendEmpty()
	for _, resFun := range resourceFunc {
		res := rs.Resource()
		resFun(res)
	}
	span := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName("foobar")
	return t
}

func generateMetrics(resourceFunc ...generateResourceFunc) pmetric.Metrics {
	m := pmetric.NewMetrics()
	ms := m.ResourceMetrics().AppendEmpty()
	for _, resFun := range resourceFunc {
		res := ms.Resource()
		resFun(res)
	}
	metric := ms.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	metric.SetName("foobar")
	return m
}

func generateLogs(resourceFunc ...generateResourceFunc) plog.Logs {
	l := plog.NewLogs()
	ls := l.ResourceLogs().AppendEmpty()
	for _, resFun := range resourceFunc {
		res := ls.Resource()
		resFun(res)
	}
	ls.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	return l
}

func withAttributes(attributes []attribute.KeyValue) generateResourceFunc {
	return func(res pcommon.Resource) {
		for _, attribute := range attributes {
			res.Attributes().PutStr(string(attribute.Key), attribute.Value.AsString())
		}
	}
}

// TestProcessPdata asserts that the processor adds the corresponding geo location data into the resource attributes if an ip is found
func TestProcessPdata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                       string
		resourceAttributes         []attribute.Key
		initResourceAttributes     []generateResourceFunc
		geoLocationMock            func(context.Context, net.IP) (attribute.Set, error)
		expectedResourceAttributes []generateResourceFunc
	}{
		{
			name:               "default source.ip attribute, not found",
			resourceAttributes: defaultResourceAttributes,
			initResourceAttributes: []generateResourceFunc{
				withAttributes([]attribute.KeyValue{
					attribute.String("ip", "1.2.3.4"),
				}),
			},
			geoLocationMock: func(context.Context, net.IP) (attribute.Set, error) {
				return attribute.NewSet([]attribute.KeyValue{attribute.String("geo.city_name", "barcelona")}...), nil
			},
			expectedResourceAttributes: []generateResourceFunc{
				withAttributes([]attribute.KeyValue{
					attribute.String("ip", "1.2.3.4"),
				}),
			},
		},
		{
			name:               "default source.ip attribute",
			resourceAttributes: defaultResourceAttributes,
			initResourceAttributes: []generateResourceFunc{
				withAttributes([]attribute.KeyValue{
					attribute.String("ip", "1.2.3.4"),
					attribute.String(string(semconv.SourceAddressKey), "1.2.3.4"),
				}),
			},
			geoLocationMock: func(context.Context, net.IP) (attribute.Set, error) {
				return attribute.NewSet([]attribute.KeyValue{attribute.String("geo.city_name", "barcelona")}...), nil
			},
			expectedResourceAttributes: []generateResourceFunc{
				withAttributes([]attribute.KeyValue{
					attribute.String("ip", "1.2.3.4"),
					attribute.String("geo.city_name", "barcelona"),
					attribute.String(string(semconv.SourceAddressKey), "1.2.3.4"),
				}),
			},
		},
		{
			name:               "default source.ip attribute with an unspecified IP address should be skipped",
			resourceAttributes: defaultResourceAttributes,
			initResourceAttributes: []generateResourceFunc{
				withAttributes([]attribute.KeyValue{
					attribute.String(string(semconv.SourceAddressKey), "0.0.0.0"),
				}),
			},
			geoLocationMock: func(context.Context, net.IP) (attribute.Set, error) {
				return attribute.NewSet([]attribute.KeyValue{attribute.String("geo.city_name", "barcelona")}...), nil
			},
			expectedResourceAttributes: []generateResourceFunc{
				withAttributes([]attribute.KeyValue{
					attribute.String(string(semconv.SourceAddressKey), "0.0.0.0"),
				}),
			},
		},
		{
			name:               "custom resource attribute",
			resourceAttributes: []attribute.Key{"ip"},
			initResourceAttributes: []generateResourceFunc{
				withAttributes([]attribute.KeyValue{
					attribute.String("ip", "1.2.3.4"),
				}),
			},
			geoLocationMock: func(context.Context, net.IP) (attribute.Set, error) {
				// only one attribute should be added as we are using a set
				return attribute.NewSet([]attribute.KeyValue{attribute.String("geo.city_name", "barcelona"), attribute.String("geo.city_name", "barcelona")}...), nil
			},
			expectedResourceAttributes: []generateResourceFunc{
				withAttributes([]attribute.KeyValue{
					attribute.String("ip", "1.2.3.4"),
					attribute.String("geo.city_name", "barcelona"),
				}),
			},
		},
		{
			name:               "custom resource attributes, match second one",
			resourceAttributes: []attribute.Key{"ip", "host.ip"},
			initResourceAttributes: []generateResourceFunc{
				withAttributes([]attribute.KeyValue{
					attribute.String("host.ip", "1.2.3.4"),
				}),
			},
			geoLocationMock: func(_ context.Context, ip net.IP) (attribute.Set, error) {
				if ip.Equal(net.IP{1, 2, 3, 4}) {
					return attribute.NewSet(attribute.String("geo.city_name", "barcelona")), nil
				}
				return attribute.Set{}, nil
			},
			expectedResourceAttributes: []generateResourceFunc{
				withAttributes([]attribute.KeyValue{
					attribute.String("host.ip", "1.2.3.4"),
					attribute.String("geo.city_name", "barcelona"),
				}),
			},
		},
		{
			name:               "do not add resource attributes with an invalid ip",
			resourceAttributes: defaultResourceAttributes,
			initResourceAttributes: []generateResourceFunc{
				withAttributes([]attribute.KeyValue{
					attribute.String(string(semconv.SourceAddressKey), "%"),
				}),
			},
			geoLocationMock: func(context.Context, net.IP) (attribute.Set, error) {
				return attribute.NewSet([]attribute.KeyValue{attribute.String("geo.city_name", "barcelona")}...), nil
			},
			expectedResourceAttributes: []generateResourceFunc{
				withAttributes([]attribute.KeyValue{
					attribute.String(string(semconv.SourceAddressKey), "%"),
				}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// prepare processor
			baseProviderMock.LocationF = tt.geoLocationMock
			processor := newGeoIPProcessor(tt.resourceAttributes)
			processor.providers = []provider.GeoIPProvider{&baseProviderMock}

			// assert metrics
			actualMetrics, err := processor.processMetrics(context.Background(), generateMetrics(tt.initResourceAttributes...))
			require.NoError(t, err)
			require.NoError(t, pmetrictest.CompareMetrics(generateMetrics(tt.expectedResourceAttributes...), actualMetrics))

			// assert traces
			actualTraces, err := processor.processTraces(context.Background(), generateTraces(tt.initResourceAttributes...))
			require.NoError(t, err)
			require.NoError(t, ptracetest.CompareTraces(generateTraces(tt.expectedResourceAttributes...), actualTraces))

			// assert logs
			actualLogs, err := processor.processLogs(context.Background(), generateLogs(tt.initResourceAttributes...))
			require.NoError(t, err)
			require.NoError(t, plogtest.CompareLogs(generateLogs(tt.expectedResourceAttributes...), actualLogs))
		})
	}
}
