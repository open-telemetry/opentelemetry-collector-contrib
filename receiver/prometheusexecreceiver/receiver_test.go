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

package prometheusexecreceiver

import (
	"context"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/translator/internaldata"
	"go.uber.org/zap"
)

// loadConfigAssertNoError loads the test config and asserts there are no errors, and returns the receiver wanted
func loadConfigAssertNoError(t *testing.T, receiverConfigName string) configmodels.Receiver {
	factories, err := componenttest.ExampleComponents()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[factory.Type()] = factory

	config, err := configtest.LoadConfigFile(t, path.Join(".", "testdata", "config.yaml"), factories)

	assert.NoError(t, err)
	assert.NotNil(t, config)

	return config.Receivers[receiverConfigName]
}

// TestExecKeyMissing loads config and asserts there is an error with that config
func TestExecKeyMissing(t *testing.T) {
	receiverConfig := loadConfigAssertNoError(t, "prometheus_exec")

	assertErrorWhenExecKeyMissing(t, receiverConfig)
}

// assertErrorWhenExecKeyMissing makes sure the config passed throws an error, since it's missing the exec key
func assertErrorWhenExecKeyMissing(t *testing.T, errorReceiverConfig configmodels.Receiver) {
	_, err := new(component.ReceiverCreateParams{Logger: zap.NewNop()}, errorReceiverConfig.(*Config), nil)
	assert.Error(t, err, "new() didn't return an error")
}

// TestEndToEnd loads the test config and completes an 2e2 test where Prometheus metrics are scrapped twice from `test_prometheus_exporter.go`
func TestEndToEnd(t *testing.T) {
	receiverConfig := loadConfigAssertNoError(t, "prometheus_exec/end_to_end_test/2")

	// e2e test with port undefined by user
	endToEndScrapeTest(t, receiverConfig, "end-to-end port not defined")
}

// endToEndScrapeTest creates a receiver that invokes `go run test_prometheus_exporter.go` and waits until it has scraped the /metrics endpoint twice - the application will crash between each scrape
func endToEndScrapeTest(t *testing.T, receiverConfig configmodels.Receiver, testName string) {
	sink := &exportertest.SinkMetricsExporter{}
	wrapper, err := new(component.ReceiverCreateParams{Logger: zap.NewNop()}, receiverConfig.(*Config), sink)
	assert.NoError(t, err, "new() returned an error")

	ctx := context.Background()
	err = wrapper.Start(ctx, componenttest.NewNopHost())
	assert.NoError(t, err, "Start() returned an error")
	defer func() { assert.NoError(t, wrapper.Shutdown(ctx)) }()

	var metrics []pdata.Metrics

	// Make sure two scrapes have been completed (this implies the process was started, scraped, restarted and finally scraped a second time)
	const waitFor = 20 * time.Second
	const tick = 100 * time.Millisecond
	require.Eventuallyf(t, func() bool {
		got := sink.AllMetrics()
		if len(got) < 2 {
			return false
		}
		metrics = got
		return true
	}, waitFor, tick, "Two scrapes not completed after %v (%v)", waitFor, testName)

	assertTwoUniqueValuesScraped(t, metrics)
}

// assertTwoUniqueValuesScraped iterates over the found metrics and returns true if it finds at least 2 unique metrics, meaning the endpoint
// was successfully scraped twice AND the subprocess being handled was stopped and restarted
func assertTwoUniqueValuesScraped(t *testing.T, metricsSlice []pdata.Metrics) {
	var value float64
	for i, val := range metricsSlice {
		temp := internaldata.MetricsToOC(val)[0].Metrics[0].Timeseries[0].Points[0].GetDoubleValue()
		if i != 0 && temp != value {
			return
		}
		if temp != value {
			value = temp
		}
	}

	assert.Fail(t, "All %v scraped values were non-unique", len(metricsSlice))
}
