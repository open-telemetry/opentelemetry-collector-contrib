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

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal"

import (
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/textparse"
	"github.com/prometheus/prometheus/scrape"
)

const (
	startTs                 = int64(1555366610000)
	interval                = int64(15 * 1000)
	defaultBuilderStartTime = float64(1.0)
)

var testMetadata = map[string]scrape.MetricMetadata{
	"counter_test":    {Metric: "counter_test", Type: textparse.MetricTypeCounter, Help: "", Unit: ""},
	"counter_test2":   {Metric: "counter_test2", Type: textparse.MetricTypeCounter, Help: "", Unit: ""},
	"gauge_test":      {Metric: "gauge_test", Type: textparse.MetricTypeGauge, Help: "", Unit: ""},
	"gauge_test2":     {Metric: "gauge_test2", Type: textparse.MetricTypeGauge, Help: "", Unit: ""},
	"hist_test":       {Metric: "hist_test", Type: textparse.MetricTypeHistogram, Help: "", Unit: ""},
	"hist_test2":      {Metric: "hist_test2", Type: textparse.MetricTypeHistogram, Help: "", Unit: ""},
	"ghist_test":      {Metric: "ghist_test", Type: textparse.MetricTypeGaugeHistogram, Help: "", Unit: ""},
	"summary_test":    {Metric: "summary_test", Type: textparse.MetricTypeSummary, Help: "", Unit: ""},
	"summary_test2":   {Metric: "summary_test2", Type: textparse.MetricTypeSummary, Help: "", Unit: ""},
	"unknown_test":    {Metric: "unknown_test", Type: textparse.MetricTypeUnknown, Help: "", Unit: ""},
	"poor_name":       {Metric: "poor_name", Type: textparse.MetricTypeGauge, Help: "", Unit: ""},
	"poor_name_count": {Metric: "poor_name_count", Type: textparse.MetricTypeCounter, Help: "", Unit: ""},
	"scrape_foo":      {Metric: "scrape_foo", Type: textparse.MetricTypeCounter, Help: "", Unit: ""},
	"example_process_start_time_seconds": {Metric: "example_process_start_time_seconds",
		Type: textparse.MetricTypeGauge, Help: "", Unit: ""},
	"process_start_time_seconds": {Metric: "process_start_time_seconds",
		Type: textparse.MetricTypeGauge, Help: "", Unit: ""},
	"badprocess_start_time_seconds": {Metric: "badprocess_start_time_seconds",
		Type: textparse.MetricTypeGauge, Help: "", Unit: ""},
}

type testDataPoint struct {
	lb labels.Labels
	t  int64
	v  float64
}

type testScrapedPage struct {
	pts []*testDataPoint
}

func createLabels(mFamily string, tagPairs ...string) labels.Labels {
	lm := make(map[string]string)
	lm[model.MetricNameLabel] = mFamily
	if len(tagPairs)%2 != 0 {
		panic("tag pairs is not even")
	}

	for i := 0; i < len(tagPairs); i += 2 {
		lm[tagPairs[i]] = tagPairs[i+1]
	}

	return labels.FromMap(lm)
}

func createDataPoint(mname string, value float64, tagPairs ...string) *testDataPoint {
	return &testDataPoint{
		lb: createLabels(mname, tagPairs...),
		v:  value,
	}
}
