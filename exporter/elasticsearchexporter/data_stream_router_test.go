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
	otel      bool
	scopeName string
	want      elasticsearch.Index
}

func createRouteTests(dsType string) []routeTestCase {
	renderWantRoute := func(dsType, dsDataset string, otel bool) elasticsearch.Index {
		if otel {
			dsDataset += ".otel"
		}
		return elasticsearch.NewDataStreamIndex(dsType, dsDataset, defaultDataStreamNamespace)
	}

	return []routeTestCase{
		{
			name: "default",
			otel: false,
			want: renderWantRoute(dsType, defaultDataStreamDataset, false),
		},
		{
			name: "otel",
			otel: true,
			want: renderWantRoute(dsType, defaultDataStreamDataset, true),
		},
		{
			name:      "default with receiver scope name",
			otel:      false,
			scopeName: "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper",
			want:      renderWantRoute(dsType, "hostmetricsreceiver", false),
		},
		{
			name:      "otel with receiver scope name",
			otel:      true,
			scopeName: "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper",
			want:      renderWantRoute(dsType, "hostmetricsreceiver", true),
		},
		{
			name:      "default with non-receiver scope name",
			otel:      false,
			scopeName: "some_other_scope_name",
			want:      renderWantRoute(dsType, defaultDataStreamDataset, false),
		},
		{
			name:      "otel with non-receiver scope name",
			otel:      true,
			scopeName: "some_other_scope_name",
			want:      renderWantRoute(dsType, defaultDataStreamDataset, true),
		},
	}
}

func TestRouteLogRecord(t *testing.T) {
	tests := createRouteTests(defaultDataStreamTypeLogs)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			router := dynamicDocumentRouter{otel: tc.otel}
			scope := pcommon.NewInstrumentationScope()
			scope.SetName(tc.scopeName)

			ds, err := router.routeLogRecord(pcommon.NewResource(), scope, pcommon.NewMap())
			require.NoError(t, err)
			assert.Equal(t, tc.want, ds)
		})
	}
}

func TestRouteDataPoint(t *testing.T) {
	tests := createRouteTests(defaultDataStreamTypeMetrics)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			router := dynamicDocumentRouter{otel: tc.otel}
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
			router := dynamicDocumentRouter{otel: tc.otel}
			scope := pcommon.NewInstrumentationScope()
			scope.SetName(tc.scopeName)

			ds, err := router.routeSpan(pcommon.NewResource(), scope, pcommon.NewMap())
			require.NoError(t, err)
			assert.Equal(t, tc.want, ds)
		})
	}
}
