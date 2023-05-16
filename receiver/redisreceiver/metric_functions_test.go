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
