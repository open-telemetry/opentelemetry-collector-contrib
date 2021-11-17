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

	"github.com/prometheus/prometheus/pkg/labels"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/model/pdata"
)

const targetExternalLabels = `
# HELP go_threads Number of OS threads created
# TYPE go_threads gauge
go_threads 19`

func TestExternalLabels(t *testing.T) {
	ctx := context.Background()
	targets := []*testData{
		{
			name: "target1",
			pages: []mockPrometheusResponse{
				{code: 200, data: targetExternalLabels},
			},
			validateFunc: verifyExternalLabels,
		},
	}

	mp, cfg, err := setupMockPrometheus(false, targets...)
	cfg.GlobalConfig.ExternalLabels = labels.FromStrings("key", "value")
	require.Nilf(t, err, "Failed to create Prometheus config: %v", err)
	defer mp.Close()

	cms := new(consumertest.MetricsSink)
	receiver := newPrometheusReceiver(componenttest.NewNopReceiverCreateSettings(), &Config{
		ReceiverSettings: config.NewReceiverSettings(config.NewComponentID(typeStr)),
		PrometheusConfig: cfg}, cms)

	require.NoError(t, receiver.Start(ctx, componenttest.NewNopHost()), "Failed to invoke Start: %v", err)
	t.Cleanup(func() { require.NoError(t, receiver.Shutdown(ctx)) })

	mp.wg.Wait()
	metrics := cms.AllMetrics()

	// split and store results by target name
	pResults := make(map[string][]*pdata.ResourceMetrics)
	for _, md := range metrics {
		rms := md.ResourceMetrics()
		for i := 0; i < rms.Len(); i++ {
			name, _ := rms.At(i).Resource().Attributes().Get("service.name")
			pResult, ok := pResults[name.AsString()]
			if !ok {
				pResult = make([]*pdata.ResourceMetrics, 0)
			}
			rm := rms.At(i)
			pResults[name.AsString()] = append(pResult, &rm)
		}
	}
	lres, lep := len(pResults), len(mp.endpoints)
	assert.Equalf(t, lep, lres, "want %d targets, but got %v\n", lep, lres)

	// loop to validate outputs for each targets
	for _, target := range targets {
		validScrapes := getValidScrapes(t, pResults[target.name])
		target.validateFunc(t, target, validScrapes)
	}
}

func verifyExternalLabels(t *testing.T, td *testData, rms []*pdata.ResourceMetrics) {
	verifyNumScrapeResults(t, td, rms)
	if len(rms) < 1 {
		t.Fatal("At least one metric request should be present")
	}
	wantAttributes := td.attributes
	metrics1 := rms[0].InstrumentationLibraryMetrics().At(0).Metrics()
	ts1 := metrics1.At(0).Gauge().DataPoints().At(0).Timestamp()
	doCompare(t, "scrape-externalLabels", wantAttributes, rms[0], []testExpectation{
		assertMetricPresent("go_threads",
			compareMetricType(pdata.MetricDataTypeGauge),
			[]dataPointExpectation{
				{
					numberPointComparator: []numberPointComparator{
						compareTimestamp(ts1),
						compareDoubleValue(19),
						compareAttributes(map[string]string{"key": "value"}),
					},
				},
			}),
	})
}
