// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package scraperinttest // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/scraperinttest"

import (
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func EqualsLatestMetrics(expected pmetric.Metrics, sink *consumertest.MetricsSink, compareOpts []pmetrictest.CompareMetricsOption) func() bool {
	return func() bool {
		allMetrics := sink.AllMetrics()
		return len(allMetrics) > 0 && nil == pmetrictest.CompareMetrics(expected, allMetrics[len(allMetrics)-1], compareOpts...)
	}
}
