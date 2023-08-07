// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package protocol // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/protocol"

import (
	"net"

	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// Parser is something that can map input StatsD strings to OTLP Metric representations.
type Parser interface {
	Initialize(enableMetricType bool, isMonotonicCounter bool, sendTimerHistogram []TimerHistogramMapping) error
	GetMetrics() []BatchMetrics
	Aggregate(line string, addr net.Addr) error
}

type BatchMetrics struct {
	Info    client.Info
	Metrics pmetric.Metrics
}
