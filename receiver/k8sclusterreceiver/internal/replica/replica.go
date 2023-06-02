// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
