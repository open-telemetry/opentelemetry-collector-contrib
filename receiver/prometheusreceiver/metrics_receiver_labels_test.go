// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prometheusreceiver

import (
	"context"
	"testing"

	agentmetricspb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/metrics/v1"
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	promcfg "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer/consumertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus"
)

const targetExternalLabels = `
# HELP go_threads Number of OS threads created
# TYPE go_threads gauge
go_threads 19`

func TestExternalLabels(t *testing.T) {
	targets := []*testData{
		{
			name: "target1",
			pages: []mockPrometheusResponse{
				{code: 200, data: targetExternalLabels},
			},
			validateFunc: verifyExternalLabels,
		},
	}

	mp, cfg, err := setupMockPrometheus(targets...)
	cfg.GlobalConfig.ExternalLabels = labels.FromStrings("key", "value")
	require.Nilf(t, err, "Failed to create Prometheus config: %v", err)

	testHelper(t, targets, mp, cfg)
}

func verifyExternalLabels(t *testing.T, td *testData, mds []*agentmetricspb.ExportMetricsServiceRequest) {
	verifyNumScrapeResults(t, td, mds)

	want := &agentmetricspb.ExportMetricsServiceRequest{
		Node:     td.node,
		Resource: td.resource,
	}
	doCompare("scrape-externalLabels", t, want, mds[0], []testExpectation{
		assertMetricPresent("go_threads",
			[]descriptorComparator{
				compareMetricType(metricspb.MetricDescriptor_GAUGE_DOUBLE),
				compareMetricLabelKeys([]string{"key"}),
			},
			[]seriesExpectation{
				{
					series: []seriesComparator{
						compareSeriesLabelValues([]string{"value"}),
					},
					points: []pointComparator{
						comparePointTimestamp(mds[0].Metrics[0].Timeseries[0].Points[0].Timestamp),
						compareDoubleVal(19),
					},
				},
			}),
	})
}

const targetLabelLimit1 = `
# HELP test_gauge0 This is my gauge
# TYPE test_gauge0 gauge
test_gauge0{label1="value1",label2="value2"} 10
`

func verifyLabelLimitTarget1(t *testing.T, td *testData, result []*agentmetricspb.ExportMetricsServiceRequest) {
	//each sample in the scraped metrics is within the configured label_limit, scrape should be successful
	assertUp(t, 1, result[0])

	want := &agentmetricspb.ExportMetricsServiceRequest{
		Node:     td.node,
		Resource: td.resource,
	}
	doCompare("scrape-labelLimit", t, want, result[0], []testExpectation{
		assertMetricPresent("test_gauge0",
			[]descriptorComparator{
				compareMetricType(metricspb.MetricDescriptor_GAUGE_DOUBLE),
				compareMetricLabelKeys([]string{"label1", "label2"}),
			},
			[]seriesExpectation{
				{
					series: []seriesComparator{
						compareSeriesLabelValues([]string{"value1", "value2"}),
					},
					points: []pointComparator{
						comparePointTimestamp(result[0].Metrics[0].Timeseries[0].Points[0].Timestamp),
						compareDoubleVal(10),
					},
				},
			}),
	})
}

const targetLabelLimit2 = `
# HELP test_gauge0 This is my gauge
# TYPE test_gauge0 gauge
test_gauge0{label1="value1",label2="value2",label3="value3"} 10
`

func verifyLabelLimitTarget2(t *testing.T, _ *testData, result []*agentmetricspb.ExportMetricsServiceRequest) {
	//The number of labels in targetLabelLimit2 exceeds the configured label_limit, scrape should be unsuccessful
	assertUp(t, 0, result[0])
}

func TestLabelLimitConfig(t *testing.T) {
	targets := []*testData{
		{
			name: "target1",
			pages: []mockPrometheusResponse{
				{code: 200, data: targetLabelLimit1},
			},
			validateFunc: verifyLabelLimitTarget1,
		},
		{
			name: "target2",
			pages: []mockPrometheusResponse{
				{code: 200, data: targetLabelLimit2},
			},
			validateFunc: verifyLabelLimitTarget2,
		},
	}

	mp, cfg, err := setupMockPrometheus(targets...)
	require.Nilf(t, err, "Failed to create Prometheus config: %v", err)

	// set label limit in scrape_config
	for _, scrapeCfg := range cfg.ScrapeConfigs {
		scrapeCfg.LabelLimit = 5
	}

	testHelper(t, targets, mp, cfg)
}

// starts prometheus receiver with custom config, retrieves metrics from MetricsSink
func testHelper(t *testing.T, targets []*testData, mp *mockPrometheus, cfg *promcfg.Config) {
	ctx := context.Background()
	defer mp.Close()

	cms := new(consumertest.MetricsSink)
	receiver := newPrometheusReceiver(componenttest.NewNopReceiverCreateSettings(), &Config{
		ReceiverSettings: config.NewReceiverSettings(config.NewComponentID(typeStr)),
		PrometheusConfig: cfg}, cms)

	require.NoError(t, receiver.Start(ctx, componenttest.NewNopHost()))

	// verify state after shutdown is called
	t.Cleanup(func() { require.NoError(t, receiver.Shutdown(ctx)) })

	// wait for all provided data to be scraped
	mp.wg.Wait()
	metrics := cms.AllMetrics()

	// split and store results by target name
	results := make(map[string][]*agentmetricspb.ExportMetricsServiceRequest)
	for _, md := range metrics {
		rms := md.ResourceMetrics()
		for i := 0; i < rms.Len(); i++ {
			ocmd := &agentmetricspb.ExportMetricsServiceRequest{}
			ocmd.Node, ocmd.Resource, ocmd.Metrics = opencensus.ResourceMetricsToOC(rms.At(i))
			result, ok := results[ocmd.Node.ServiceInfo.Name]
			if !ok {
				result = make([]*agentmetricspb.ExportMetricsServiceRequest, 0)
			}
			results[ocmd.Node.ServiceInfo.Name] = append(result, ocmd)
		}
	}

	// loop to validate outputs for each targets
	for _, target := range targets {
		t.Run(target.name, func(t *testing.T) {
			target.validateFunc(t, target, results[target.name])
		})
	}
}
