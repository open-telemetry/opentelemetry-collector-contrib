// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package geoipprocessor

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/ptracetest"
	conventions "github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor/internal/convention"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor/internal/provider"
	maxmind "github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor/internal/provider/maxmindprovider"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor/internal/provider/maxmindprovider/testdata"
)

func TestProcessorWithMaxMind(t *testing.T) {
	t.Parallel()

	tmpDBfiles := testdata.GenerateLocalDB(t, "./internal/provider/maxmindprovider/testdata/")
	defer os.RemoveAll(tmpDBfiles)

	factory := NewFactory()
	maxmindConfig := maxmind.Config{}
	maxmindConfig.DatabasePath = tmpDBfiles + "/" + "GeoLite2-City-Test.mmdb"
	cfg := &Config{Providers: map[string]provider.Config{"maxmind": &maxmindConfig}}

	actualAttributes := withAttributes([]attribute.KeyValue{semconv.SourceAddress("1.2.3.4")})
	expectedAttributes := withAttributes([]attribute.KeyValue{
		semconv.SourceAddress("1.2.3.4"),
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
	})

	// verify metrics
	nextMetrics := new(consumertest.MetricsSink)
	metricsProcessor, err := factory.CreateMetricsProcessor(context.Background(), processortest.NewNopSettings(), cfg, nextMetrics)
	require.NoError(t, err)
	err = metricsProcessor.ConsumeMetrics(context.Background(), generateMetrics(actualAttributes))
	require.NoError(t, err)

	actualMetrics := nextMetrics.AllMetrics()
	require.Equal(t, 1, len(actualMetrics))
	require.NoError(t, pmetrictest.CompareMetrics(generateMetrics(expectedAttributes), actualMetrics[0]))

	// the testing database does not contain metadata for IP 40.0.0.0, see ./internal/provider/maxmindprovider/testdata/GeoLite2-City-Test.json
	err = metricsProcessor.ConsumeMetrics(context.Background(), generateMetrics(withAttributes([]attribute.KeyValue{
		semconv.SourceAddress("40.0.0.0"),
	})))
	require.Contains(t, err.Error(), "no geo IP metadata found")

	// verify logs
	nextLogs := new(consumertest.LogsSink)
	logsProcessor, err := factory.CreateLogsProcessor(context.Background(), processortest.NewNopSettings(), cfg, nextLogs)
	require.NoError(t, err)
	err = logsProcessor.ConsumeLogs(context.Background(), generateLogs(actualAttributes))
	require.NoError(t, err)

	actualLogs := nextLogs.AllLogs()
	require.Equal(t, 1, len(actualLogs))
	require.NoError(t, plogtest.CompareLogs(generateLogs(expectedAttributes), actualLogs[0]))

	// verify traces
	nextTraces := new(consumertest.TracesSink)
	tracesProcessor, err := factory.CreateTracesProcessor(context.Background(), processortest.NewNopSettings(), cfg, nextTraces)
	require.NoError(t, err)
	err = tracesProcessor.ConsumeTraces(context.Background(), generateTraces(actualAttributes))
	require.NoError(t, err)

	actualTraces := nextTraces.AllTraces()
	require.Equal(t, 1, len(actualTraces))
	require.NoError(t, ptracetest.CompareTraces(generateTraces(expectedAttributes), actualTraces[0]))
}
