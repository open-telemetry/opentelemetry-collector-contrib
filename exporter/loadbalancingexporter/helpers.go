// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// mergeTraces concatenates two ptrace.Traces into a single ptrace.Traces.
func mergeTraces(t1 ptrace.Traces, t2 ptrace.Traces) ptrace.Traces {
	t2.ResourceSpans().MoveAndAppendTo(t1.ResourceSpans())
	return t1
}

// mergeMetrics concatenates two pmetric.Metrics into a single pmetric.Metrics.
func mergeMetrics(m1 pmetric.Metrics, m2 pmetric.Metrics) pmetric.Metrics {
	m2.ResourceMetrics().MoveAndAppendTo(m1.ResourceMetrics())
	return m1
}
