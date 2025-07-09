// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/correctnesstests/metrics"

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/metricstestutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

// testHarness listens for datapoints from the receiver to which it is attached
// and when it receives one, it compares it to the datapoint that was previously
// sent out. It then sends the next datapoint, if there is one.
type testHarness struct {
	t                  *testing.T
	metricSupplier     *metricSupplier
	metricIndex        *metricsReceivedIndex
	sender             testbed.MetricDataSender
	currPDM            pmetric.Metrics
	diffConsumer       diffConsumer
	outOfMetrics       bool
	allMetricsReceived chan struct{}
}

type diffConsumer interface {
	accept(string, []*metricstestutil.MetricDiff)
}

func newTestHarness(
	t *testing.T,
	s *metricSupplier,
	mi *metricsReceivedIndex,
	ds testbed.MetricDataSender,
	diffConsumer diffConsumer,
) *testHarness {
	return &testHarness{
		t:                  t,
		metricSupplier:     s,
		metricIndex:        mi,
		sender:             ds,
		diffConsumer:       diffConsumer,
		allMetricsReceived: make(chan struct{}),
	}
}

func (h *testHarness) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (h *testHarness) ConsumeMetrics(_ context.Context, pdm pmetric.Metrics) error {
	h.compare(pdm)
	if h.metricIndex.allReceived() {
		close(h.allMetricsReceived)
	}
	if !h.outOfMetrics {
		h.sendNextMetric()
	}
	return nil
}

func (h *testHarness) compare(pdm pmetric.Metrics) {
	pdms := pdm.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	var diffs []*metricstestutil.MetricDiff
	for i := 0; i < pdms.Len(); i++ {
		pdmRecd := pdms.At(i)
		metricName := pdmRecd.Name()
		metric, found := h.metricIndex.lookup(metricName)
		if !found {
			h.diffConsumer.accept(metricName, []*metricstestutil.MetricDiff{{
				ExpectedValue: metricName,
				Msg:           "Metric name not found in index",
			}})
		}
		if !metric.received {
			metric.received = true
			sent := metric.pdm
			pdmExpected := sent.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
			diffs = metricstestutil.DiffMetric(
				diffs,
				pdmExpected,
				pdmRecd,
			)
			h.diffConsumer.accept(metricName, diffs)
		}
	}
}

func (h *testHarness) sendNextMetric() {
	h.currPDM, h.outOfMetrics = h.metricSupplier.nextMetrics()
	if h.outOfMetrics {
		return
	}
	err := h.sender.ConsumeMetrics(context.Background(), h.currPDM)
	require.NoError(h.t, err)
}
