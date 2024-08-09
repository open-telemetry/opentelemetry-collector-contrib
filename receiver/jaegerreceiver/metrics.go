// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jaegerreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver"

import (
	"github.com/jaegertracing/jaeger/pkg/metrics"
)

const (
	DroppedPacketsStat   = "thrift.udp.server.packets.dropped"
	ProcessedPacketsStat = "thrift.udp.server.packets.processed"
)

type counterHandler func(int64)

type OTelMetricsCounter struct {
	handler counterHandler
}

func (jc *OTelMetricsCounter) Inc(val int64) {
	jc.handler(val)
}

type OTelMetrics struct {
	namespace string
	counters  map[string]counterHandler
}

func (jm *OTelMetrics) Counter(options metrics.Options) metrics.Counter {

	handler := jm.counters[options.Name]
	if handler == nil {
		return metrics.NullCounter
	}
	return &OTelMetricsCounter{handler: handler}
}

func (jm *OTelMetrics) Timer(_ metrics.TimerOptions) metrics.Timer {
	return metrics.NullTimer
}

func (jm *OTelMetrics) Gauge(_ metrics.Options) metrics.Gauge {
	return metrics.NullGauge
}

func (jm *OTelMetrics) Histogram(_ metrics.HistogramOptions) metrics.Histogram {
	return metrics.NullHistogram
}

func (jm *OTelMetrics) Namespace(scope metrics.NSOptions) metrics.Factory {
	return &OTelMetrics{
		namespace: jm.namespace + "." + scope.Name,
	}
}
