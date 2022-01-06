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

package redisreceiver

import (
	"reflect"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver/internal/metadata"
)

func TestDataPointRecorders(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	settings := componenttest.NewNopReceiverCreateSettings()
	settings.Logger = logger
	rs := &redisScraper{
		redisSvc: newRedisSvc(newFakeClient()),
		settings: settings,
		mb:       metadata.NewMetricsBuilder(Config{}.Metrics),
	}
	metricByRecorder := map[string]string{}
	for metric, recorder := range rs.dataPointRecorders() {
		switch recorder.(type) {
		case func(pdata.Timestamp, int64), func(pdata.Timestamp, float64):
			recorderName := runtime.FuncForPC(reflect.ValueOf(recorder).Pointer()).Name()
			if m, ok := metricByRecorder[recorderName]; ok {
				assert.Failf(t, "shared-recorder", "Metrics %q and %q share the same recorder", metric, m)
			}
			metricByRecorder[recorderName] = metric
		default:
			assert.Failf(t, "invalid-recorder", "Metric %q has invalid recorder type", metric)
		}
	}
}
