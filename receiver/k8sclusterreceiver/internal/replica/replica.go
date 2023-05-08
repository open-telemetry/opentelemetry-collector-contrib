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

package replica // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/replica"

import (
	"fmt"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/utils"
)

func GetMetrics(resource string, desired, available int32) []*metricspb.Metric {
	return []*metricspb.Metric{
		{
			MetricDescriptor: &metricspb.MetricDescriptor{
				Name:        fmt.Sprintf("k8s.%s.desired", resource),
				Description: fmt.Sprintf("Number of desired pods in this %s", resource),
				Type:        metricspb.MetricDescriptor_GAUGE_INT64,
				Unit:        "1",
			},
			Timeseries: []*metricspb.TimeSeries{utils.GetInt64TimeSeries(int64(desired))},
		},
		{
			MetricDescriptor: &metricspb.MetricDescriptor{
				Name:        fmt.Sprintf("k8s.%s.available", resource),
				Description: fmt.Sprintf("Total number of available pods (ready for at least minReadySeconds) targeted by this %s", resource),
				Type:        metricspb.MetricDescriptor_GAUGE_INT64,
				Unit:        "1",
			},
			Timeseries: []*metricspb.TimeSeries{utils.GetInt64TimeSeries(int64(available))},
		},
	}
}
