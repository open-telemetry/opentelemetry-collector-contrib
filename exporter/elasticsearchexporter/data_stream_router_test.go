// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type routeTestInfo struct {
	name string
	otel bool
	want string
}

func createRouteTests(dsType string) []routeTestInfo {
	renderWantRoute := func(dsType string, otel bool) string {
		if otel {
			return fmt.Sprintf("%s-%s.otel-%s", dsType, defaultDataStreamDataset, defaultDataStreamNamespace)
		}
		return fmt.Sprintf("%s-%s-%s", dsType, defaultDataStreamDataset, defaultDataStreamNamespace)
	}

	return []routeTestInfo{
		{
			name: "default",
			otel: false,
			want: renderWantRoute(dsType, false),
		},
		{
			name: "otel",
			otel: true,
			want: renderWantRoute(dsType, true),
		},
	}
}

func TestRouteLogRecord(t *testing.T) {

	tests := createRouteTests(defaultDataStreamTypeLogs)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ds := routeLogRecord(plog.NewLogRecord(), plog.NewScopeLogs().Scope(), plog.NewResourceLogs().Resource(), "", tc.otel)
			assert.Equal(t, tc.want, ds)
		})
	}
}

func TestRouteDataPoint(t *testing.T) {

	tests := createRouteTests(defaultDataStreamTypeMetrics)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ds := routeDataPoint(pmetric.NewNumberDataPoint(), plog.NewScopeLogs().Scope(), plog.NewResourceLogs().Resource(), "", tc.otel)
			assert.Equal(t, tc.want, ds)
		})
	}
}

func TestRouteSpan(t *testing.T) {

	tests := createRouteTests(defaultDataStreamTypeTraces)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ds := routeSpan(ptrace.NewSpan(), plog.NewScopeLogs().Scope(), plog.NewResourceLogs().Resource(), "", tc.otel)
			assert.Equal(t, tc.want, ds)
		})
	}
}
