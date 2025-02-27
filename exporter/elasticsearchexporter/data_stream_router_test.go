// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
)

type routeTestCase struct {
	name      string
	mode      MappingMode
	scopeName string
	want      elasticsearch.Index
}

func createRouteTests(dsType string) []routeTestCase {
	renderWantRoute := func(dsType, dsDataset string, mode MappingMode) elasticsearch.Index {
		if mode == MappingOTel {
			dsDataset += ".otel"
		}
		return elasticsearch.NewDataStreamIndex(dsType, dsDataset, defaultDataStreamNamespace)
	}

	return []routeTestCase{
		{
			name: "default",
			mode: MappingNone,
			want: renderWantRoute(dsType, defaultDataStreamDataset, MappingNone),
		},
		{
			name: "otel",
			mode: MappingOTel,
			want: renderWantRoute(dsType, defaultDataStreamDataset, MappingOTel),
		},
		{
			name:      "default with receiver scope name",
			mode:      MappingNone,
			scopeName: "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper",
			want:      renderWantRoute(dsType, "hostmetricsreceiver", MappingNone),
		},
		{
			name:      "otel with receiver scope name",
			mode:      MappingOTel,
			scopeName: "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper",
			want:      renderWantRoute(dsType, "hostmetricsreceiver", MappingOTel),
		},
		{
			name:      "default with non-receiver scope name",
			mode:      MappingNone,
			scopeName: "some_other_scope_name",
			want:      renderWantRoute(dsType, defaultDataStreamDataset, MappingNone),
		},
		{
			name:      "otel with non-receiver scope name",
			mode:      MappingOTel,
			scopeName: "some_other_scope_name",
			want:      renderWantRoute(dsType, defaultDataStreamDataset, MappingOTel),
		},
	}
}

func TestRouteLogRecord(t *testing.T) {
	tests := createRouteTests(defaultDataStreamTypeLogs)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			router := dynamicDocumentRouter{mode: tc.mode}
			scope := pcommon.NewInstrumentationScope()
			scope.SetName(tc.scopeName)

			ds, err := router.routeLogRecord(pcommon.NewResource(), scope, pcommon.NewMap())
			require.NoError(t, err)
			assert.Equal(t, tc.want, ds)
		})
	}

	t.Run("test data_stream.type for bodymap mode", func(t *testing.T) {
		dsType := "metrics"
		router := dynamicDocumentRouter{mode: MappingBodyMap}
		attrs := pcommon.NewMap()
		attrs.PutStr("data_stream.type", dsType)
		ds, err := router.routeLogRecord(pcommon.NewResource(), pcommon.NewInstrumentationScope(), attrs)
		require.NoError(t, err)
		assert.Equal(t, dsType, ds.Type)
	})
	t.Run("test data_stream.type is not honored for other modes (except bodymap)", func(t *testing.T) {
		dsType := "metrics"
		router := dynamicDocumentRouter{mode: MappingOTel}
		attrs := pcommon.NewMap()
		attrs.PutStr("data_stream.type", dsType)
		ds, err := router.routeLogRecord(pcommon.NewResource(), pcommon.NewInstrumentationScope(), attrs)
		require.NoError(t, err)
		assert.Equal(t, "logs", ds.Type) // should equal to logs
	})

	t.Run("test data_stream.type does not accept values other than logs/metrics", func(t *testing.T) {
		dsType := "random"
		router := dynamicDocumentRouter{mode: MappingBodyMap}
		attrs := pcommon.NewMap()
		attrs.PutStr("data_stream.type", dsType)
		_, err := router.routeLogRecord(pcommon.NewResource(), pcommon.NewInstrumentationScope(), attrs)
		require.Error(t, err, "data_stream.type cannot be other than logs or metrics")
	})
}

func TestRouteDataPoint(t *testing.T) {
	tests := createRouteTests(defaultDataStreamTypeMetrics)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			router := dynamicDocumentRouter{mode: tc.mode}
			scope := pcommon.NewInstrumentationScope()
			scope.SetName(tc.scopeName)

			ds, err := router.routeDataPoint(pcommon.NewResource(), scope, pcommon.NewMap())
			require.NoError(t, err)
			assert.Equal(t, tc.want, ds)
		})
	}
}

func TestRouteSpan(t *testing.T) {
	tests := createRouteTests(defaultDataStreamTypeTraces)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			router := dynamicDocumentRouter{mode: tc.mode}
			scope := pcommon.NewInstrumentationScope()
			scope.SetName(tc.scopeName)

			ds, err := router.routeSpan(pcommon.NewResource(), scope, pcommon.NewMap())
			require.NoError(t, err)
			assert.Equal(t, tc.want, ds)
		})
	}
}
