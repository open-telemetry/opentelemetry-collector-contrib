// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datastream

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

type routeTestCase struct {
	name      string
	otel      bool
	scopeName string
	want      string
}

func createRouteTests(dsType string) []routeTestCase {
	renderWantRoute := func(dsType, dsDataset string, otel bool) string {
		if otel {
			return fmt.Sprintf("%s-%s.otel-%s", dsType, dsDataset, DefaultDataStreamNamespace)
		}
		return fmt.Sprintf("%s-%s-%s", dsType, dsDataset, DefaultDataStreamNamespace)
	}

	return []routeTestCase{
		{
			name: "default",
			otel: false,
			want: renderWantRoute(dsType, DefaultDataStreamDataset, false),
		},
		{
			name: "otel",
			otel: true,
			want: renderWantRoute(dsType, DefaultDataStreamDataset, true),
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
			want:      renderWantRoute(dsType, DefaultDataStreamDataset, false),
		},
		{
			name:      "otel with non-receiver scope name",
			otel:      true,
			scopeName: "some_other_scope_name",
			want:      renderWantRoute(dsType, DefaultDataStreamDataset, true),
		},
	}
}

func TestRouteLogRecord(t *testing.T) {
	tests := createRouteTests(DefaultDataStreamTypeLogs)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ds := RouteLogRecord(pcommon.NewMap(), pcommon.NewMap(), pcommon.NewMap(), "", tc.otel, tc.scopeName)
			assert.Equal(t, tc.want, ds)
		})
	}
}

func TestRouteDataPoint(t *testing.T) {
	tests := createRouteTests(DefaultDataStreamTypeMetrics)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ds := RouteDataPoint(pcommon.NewMap(), pcommon.NewMap(), pcommon.NewMap(), "", tc.otel, tc.scopeName)
			assert.Equal(t, tc.want, ds)
		})
	}
}

func TestRouteSpan(t *testing.T) {
	tests := createRouteTests(DefaultDataStreamTypeTraces)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ds := RouteSpan(pcommon.NewMap(), pcommon.NewMap(), pcommon.NewMap(), "", tc.otel, tc.scopeName)
			assert.Equal(t, tc.want, ds)
		})
	}
}
