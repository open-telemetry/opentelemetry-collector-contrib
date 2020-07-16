// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package prometheusexec

import (
	"context"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer/consumerdata"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.uber.org/zap"
)

func TestEndToEnd(t *testing.T) {
	factories, err := config.ExampleComponents()
	assert.NoError(t, err)

	factory := &Factory{}
	factories.Receivers[factory.Type()] = factory

	config, err := config.LoadConfigFile(t, path.Join(".", "testdata", "config.yaml"), factories)

	assert.NoError(t, err)
	assert.NotNil(t, config)

	// Receiver without exec key, error expected
	errorReceiverConfig := config.Receivers["prometheus_exec"]
	wrapper := new(zap.NewNop(), errorReceiverConfig.(*Config), nil)

	err = wrapper.Start(context.Background(), nil)
	if err == nil {
		t.Errorf("end_to_end_test.go didn't get error, was expecting one since this config has no 'exec' key")
	}

	// Waitgroup to allow the goroutines to finish for the following end-to-end tests
	var waitGroup sync.WaitGroup
	waitGroup.Add(2)

	// Normal test with port defined, expose metrics from fake exporter and make sure they're scraped/received
	go scrapeTest(t, config.Receivers["prometheus_exec/end_to_end_test/1"], &waitGroup)

	// Normal test with port undefined by user, same as previous test
	go scrapeTest(t, config.Receivers["prometheus_exec/end_to_end_test/2"], &waitGroup)

	waitGroup.Wait()
}

// scrapeTest scrapes an endpoint twice, and between each scrape waits the correct amount of time for the subprocess exporter (./testdata/end_to_end_metrics_test/expoter.go)
// to fail and restart, meaning it verifies two things: the scrape is successful (twice), the process was restarted correctly when failed and the underlying
// Prometheus receiver was correctly stopped and then restarted. For extra testing the metrics are different every time the subprocess exporter is started
// And the uniqueness of the metric scraped is verified
func scrapeTest(t *testing.T, receiverConfig configmodels.Receiver, waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()

	// Create the wrapper
	sink := &exportertest.SinkMetricsExporterOld{}
	wrapper := new(zap.NewNop(), receiverConfig.(*Config), sink)

	// Initiate building the embedded configs and managing the subprocess with Start(), and keep track of current time
	err := wrapper.Start(context.Background(), componenttest.NewNopHost())
	if err != nil {
		t.Errorf("end_to_end_test.go got error = %v", err)
	}
	now := time.Now()
	defer wrapper.Shutdown(context.Background())

	var metrics []consumerdata.MetricsData

	// Make sure a first scrape works by checking for metrics in the test metrics exporter "sink", only return true when there are metrics
	const waitFor = 30 * time.Second
	const tick = 1 * time.Second
	require.Eventuallyf(t, func() bool {
		got := sink.AllMetrics()
		if len(got) == 0 {
			return false
		}

		metrics = got
		return true
	}, waitFor, tick, "No metrics were collected after %v for the first scrape", waitFor)

	//  Wait for the next time the process/fake exporter will fail and be restarted
	time.Sleep(time.Duration(now.Add(10*time.Second).UnixNano()-time.Now().UnixNano()) * time.Nanosecond)
	metrics = sink.AllMetrics()

	// Make sure the second scrape is successful, and validate that the metrics are different in the second scrape
	require.Eventuallyf(t, func() bool {
		got := sink.AllMetrics()
		if len(got) == 0 || len(got) == len(metrics) {
			return false
		}
		return validateMetrics(&got)
	}, waitFor, tick, "No metrics were collected after %v for the second scrape", waitFor)
}

// validateMetrics iterates over the found metrics and returns true if it finds at least 2 unique metrics
func validateMetrics(metricsSlice *[]consumerdata.MetricsData) bool {
	var value float64
	for i, val := range *metricsSlice {
		temp := val.Metrics[0].Timeseries[0].Points[0].GetDoubleValue()
		if i != 0 && temp != value {
			return true
		}
		if temp != value {
			value = temp
		}
	}
	return false
}
