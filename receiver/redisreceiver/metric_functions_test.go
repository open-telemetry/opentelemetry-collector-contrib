// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redisreceiver

import (
	"reflect"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver/internal/metadata"
)

func TestDataPointRecorders(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	settings := receivertest.NewNopCreateSettings()
	settings.Logger = logger
	rs := &redisScraper{
		redisSvc: newRedisSvc(newFakeClient()),
		settings: settings.TelemetrySettings,
		mb:       metadata.NewMetricsBuilder(Config{}.MetricsBuilderConfig, settings),
	}
	metricByRecorder := map[string]string{}
	for metric, recorder := range rs.dataPointRecorders() {
		switch recorder.(type) {
		case func(pcommon.Timestamp, int64), func(pcommon.Timestamp, float64):
			recorderName := runtime.FuncForPC(reflect.ValueOf(recorder).Pointer()).Name()
			require.NotContains(t, metricByRecorder, recorderName, "share the same recorder")
			metricByRecorder[recorderName] = metric
		default:
			assert.Failf(t, "invalid-recorder", "Metric %q has invalid recorder type", metric)
		}
	}
}
