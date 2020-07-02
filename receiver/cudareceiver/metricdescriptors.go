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

package cudareceiver

import (
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
)

// CUDA (GPU) metric constants.

// https://docs.nvidia.com/deploy/nvml-api/group__nvmlDeviceQueries.html#group__nvmlDeviceQueries_1g92d1c5182a14dd4be7090e3c1480b121
var metricTemperature = &metricspb.MetricDescriptor{
	Name:        "gpu/temperature",
	Description: "",
	Unit:        "C",
	Type:        metricspb.MetricDescriptor_GAUGE_INT64,
	LabelKeys:   nil,
}

// https://docs.nvidia.com/deploy/nvml-api/group__nvmlDeviceQueries.html#group__nvmlDeviceQueries_1g7ef7dff0ff14238d08a19ad7fb23fc87
var metricPower = &metricspb.MetricDescriptor{
	Name:        "gpu/power",
	Description: "",
	Unit:        "milliwatts",
	Type:        metricspb.MetricDescriptor_GAUGE_INT64,
	LabelKeys:   nil,
}

// https://docs.nvidia.com/deploy/nvml-api/group__nvmlDeviceQueries.html#group__nvmlDeviceQueries_1gd86f1c74f81b5ddfaa6cb81b51030c72
var metricPCIeThroughput = &metricspb.MetricDescriptor{
	Name:        "gpu/pcie_throughput",
	Description: "",
	Unit:        "KB/s",
	Type:        metricspb.MetricDescriptor_GAUGE_INT64,
	LabelKeys:   nil,
}

var cudaMetricDescriptors = []*metricspb.MetricDescriptor{
	metricTemperature,
	metricPower,
	metricPCIeThroughput,
}
