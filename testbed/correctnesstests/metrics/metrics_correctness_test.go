// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"fmt"
	"log"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/goldendataset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/metricstestutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/correctnesstests"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

// tests with the prefix "TestHarness_" get run in the "correctnesstests-metrics" ci job
func TestHarness_MetricsGoldenData(t *testing.T) {
	tests, err := correctnesstests.LoadPictOutputPipelineDefs(
		"testdata/generated_pict_pairs_metrics_pipeline.txt",
	)
	require.NoError(t, err)

	res := results{}
	res.Init("results")
	for _, test := range tests {
		test.TestName = fmt.Sprintf("%s-%s", test.Receiver, test.Exporter)
		test.DataSender = correctnesstests.ConstructMetricsSender(t, test.Receiver)
		test.DataReceiver = correctnesstests.ConstructReceiver(t, test.Exporter)
		t.Run(test.TestName, func(t *testing.T) {
			r := testWithMetricsGoldenDataset(
				t,
				test.DataSender.(testbed.MetricDataSender),
				test.DataReceiver,
			)
			res.Add("", r)
		})
	}
	res.Save()
}

func testWithMetricsGoldenDataset(
	t *testing.T,
	sender testbed.MetricDataSender,
	receiver testbed.DataReceiver,
) result {
	mds := getTestMetrics(t)
	accumulator := newDiffAccumulator()
	h := newTestHarness(
		t,
		newMetricSupplier(mds),
		newMetricsReceivedIndex(mds),
		sender,
		accumulator,
	)
	tc := newCorrectnessTestCase(t, sender, receiver, h)

	tc.startTestbedReceiver()
	tc.startCollector()
	tc.startTestbedSender()

	tc.sendFirstMetric()
	tc.waitForAllMetrics()

	tc.stopTestbedReceiver()
	tc.stopCollector()

	r := result{
		testName:   t.Name(),
		testResult: "PASS",
		numDiffs:   accumulator.numDiffs,
	}
	if accumulator.numDiffs > 0 {
		r.testResult = "FAIL"
		t.Fail()
	}
	return r
}

func getTestMetrics(t *testing.T) []pmetric.Metrics {
	const file = "../../../internal/coreinternal/goldendataset/testdata/generated_pict_pairs_metrics.txt"
	mds, err := goldendataset.GenerateMetrics(file)
	require.NoError(t, err)
	return mds
}

type diffAccumulator struct {
	numDiffs int
}

var _ diffConsumer = (*diffAccumulator)(nil)

func newDiffAccumulator() *diffAccumulator {
	return &diffAccumulator{}
}

func (d *diffAccumulator) accept(metricName string, diffs []*metricstestutil.MetricDiff) {
	if len(diffs) > 0 {
		d.numDiffs++
		log.Printf("Found diffs for [%v]\n%v", metricName, diffs)
	}
}
