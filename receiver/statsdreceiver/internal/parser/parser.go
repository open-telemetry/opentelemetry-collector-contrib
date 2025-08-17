// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package parser // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/internal/parser"

import (
	"net"

	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/protocol"
)

// Parser is something that can map input StatsD strings to OTLP Metric representations.
type Parser interface {
	Initialize(enableMetricType, enableSimpleTags, isMonotonicCounter, enableIPOnlyAggregation bool, sendTimerHistogram []protocol.TimerHistogramMapping) error
	GetMetrics() []BatchMetrics
	Aggregate(line string, addr net.Addr) error
}

type BatchMetrics struct {
	Info    client.Info
	Metrics pmetric.Metrics
}
