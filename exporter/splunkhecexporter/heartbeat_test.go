// Copyright The OpenTelemetry Authors
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

package splunkhecexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter"

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
)

const (
	metricLabelKey = "customKey"
	metricLabelVal = "customVal"
)

func createTestConfig(metricsOverrides map[string]string, enableMetrics bool) *Config {
	config := NewFactory().CreateDefaultConfig().(*Config)
	config.Heartbeat = HecHeartbeat{
		Interval: 10 * time.Millisecond,
	}
	config.Telemetry = HecTelemetry{
		Enabled:              enableMetrics,
		OverrideMetricsNames: metricsOverrides,
		ExtraAttributes: map[string]string{
			metricLabelKey: metricLabelVal,
		},
	}
	return config
}

func initHeartbeater(t *testing.T, metricsOverrides map[string]string, enableMetrics bool, consumeFn func(ctx context.Context, ld plog.Logs) error) {
	config := createTestConfig(metricsOverrides, enableMetrics)
	hbter := newHeartbeater(config, component.NewDefaultBuildInfo(), consumeFn)
	t.Cleanup(func() {
		hbter.shutdown()
	})
}

func assertHeartbeatInfoLog(t *testing.T, l plog.Logs) {
	assert.Equal(t, 1, l.ResourceLogs().Len())
	assert.Contains(t, l.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().AsString(), "HeartbeatInfo")
}

func getMetricValue(metricName string) []float64 {
	viewData, _ := view.RetrieveData(metricName)
	var ret []float64
	if len(viewData) > 0 {
		for _, data := range viewData {
			ret = append(ret, data.Data.(*view.SumData).Value)
		}
	}
	return ret
}

func getTags(metricName string) [][]tag.Tag {
	viewData, _ := view.RetrieveData(metricName)
	var ret [][]tag.Tag
	if len(viewData) > 0 {
		for _, data := range viewData {
			ret = append(ret, data.Tags)
		}
	}
	return ret
}

func resetMetrics(metricsNames ...string) {
	for _, metricsName := range metricsNames {
		if v := view.Find(metricsName); v != nil {
			view.Unregister(v)
		}
	}
}

func Test_newHeartbeater_disabled(t *testing.T) {
	config := createTestConfig(map[string]string{}, false)
	config.Heartbeat.Interval = 0
	hb := newHeartbeater(config, component.NewDefaultBuildInfo(), func(ctx context.Context, ld plog.Logs) error {
		return nil
	})
	assert.Nil(t, hb)
}

func Test_Heartbeat_success(t *testing.T) {
	tests := []struct {
		metricsOverrides map[string]string
		enableMetrics    bool
	}{
		{
			metricsOverrides: map[string]string{},
			enableMetrics:    false,
		},
		{
			metricsOverrides: map[string]string{
				defaultHBSentMetricsName: "app_heartbeat_success_total",
			},
			enableMetrics: true,
		},
		{
			metricsOverrides: map[string]string{},
			enableMetrics:    false,
		},
	}

	for _, tt := range tests {
		consumeLogsChan := make(chan plog.Logs, 10)
		consumeFn := func(ctx context.Context, ld plog.Logs) error {
			consumeLogsChan <- ld
			return nil
		}
		initHeartbeater(t, tt.metricsOverrides, true, consumeFn)

		assert.Eventually(t, func() bool {
			return len(consumeLogsChan) != 0
		}, time.Second, 10*time.Millisecond)

		logs := <-consumeLogsChan
		assertHeartbeatInfoLog(t, logs)

		if tt.enableMetrics {
			sentMetricsName := getMetricsName(tt.metricsOverrides, defaultHBSentMetricsName)
			failedMetricsName := getMetricsName(tt.metricsOverrides, defaultHBFailedMetricsName)

			assert.Eventually(t, func() bool {
				return len(getMetricValue(sentMetricsName)) != 0
			}, time.Second, 10*time.Millisecond)
			assert.Greater(t, getMetricValue(sentMetricsName)[0], float64(0), "there should be at least one success metric datapoint")
			metricLabelKeyTag, _ := tag.NewKey(metricLabelKey)
			assert.Equal(t, []tag.Tag{{Key: metricLabelKeyTag, Value: metricLabelVal}}, getTags(sentMetricsName)[0])

			resetMetrics(sentMetricsName, failedMetricsName)
		}
	}
}

func Test_Heartbeat_failure(t *testing.T) {
	resetMetrics()
	consumeFn := func(ctx context.Context, ld plog.Logs) error {
		return errors.New("always error")
	}
	initHeartbeater(t, map[string]string{}, true, consumeFn)

	assert.Eventually(t, func() bool {
		return len(getMetricValue(defaultHBFailedMetricsName)) != 0
	}, time.Second, 10*time.Millisecond)
	assert.Greater(t, getMetricValue(defaultHBFailedMetricsName)[0], float64(0), "there should be at least one failure metric datapoint")
	metricLabelKeyTag, _ := tag.NewKey(metricLabelKey)
	assert.Equal(t, []tag.Tag{{Key: metricLabelKeyTag, Value: metricLabelVal}}, getTags(defaultHBFailedMetricsName)[0])
}
