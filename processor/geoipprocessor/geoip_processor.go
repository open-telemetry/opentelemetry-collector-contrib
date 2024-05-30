// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package geoipprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor/internal/provider"
)

// GeoProviders will be used by the Processor retrieve geographical metadata given an IP address.
type GeoProviders = []provider.GeoIPProvider

type geoIPProcessor struct {
	providers GeoProviders
}

func newGeoIPProcessor(providers GeoProviders) *geoIPProcessor {
	return &geoIPProcessor{
		providers,
	}
}

func (g *geoIPProcessor) processMetrics(_ context.Context, ms pmetric.Metrics) (pmetric.Metrics, error) {
	return ms, nil
}

func (g *geoIPProcessor) processTraces(_ context.Context, ts ptrace.Traces) (ptrace.Traces, error) {
	return ts, nil
}

func (g *geoIPProcessor) processLogs(_ context.Context, ls plog.Logs) (plog.Logs, error) {
	return ls, nil
}
