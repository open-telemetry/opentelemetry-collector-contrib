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

package awsecscontainermetrics

import (
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/golang/protobuf/ptypes/timestamp"
)

func convertToOCMetrics(prefix string, m ECSMetrics, labelKeys []*metricspb.LabelKey, labelValues []*metricspb.LabelValue, timestamp *timestamp.Timestamp) []*metricspb.Metric {

	return applyTimestamp([]*metricspb.Metric{
		intGauge(prefix+AttributeMemoryUsage, UnitBytes, &m.MemoryUsage, labelKeys, labelValues),
		intGauge(prefix+AttributeMemoryMaxUsage, UnitBytes, &m.MemoryMaxUsage, labelKeys, labelValues),
		intGauge(prefix+AttributeMemoryLimit, UnitBytes, &m.MemoryLimit, labelKeys, labelValues),
		intGauge(prefix+AttributeMemoryUtilized, UnitMegaBytes, &m.MemoryUtilized, labelKeys, labelValues),
		intGauge(prefix+AttributeMemoryReserved, UnitMegaBytes, &m.MemoryReserved, labelKeys, labelValues),

		intCumulative(prefix+AttributeCPUTotalUsage, UnitNanoSecond, &m.CPUTotalUsage, labelKeys, labelValues),
		intCumulative(prefix+AttributeCPUKernelModeUsage, UnitNanoSecond, &m.CPUUsageInKernelmode, labelKeys, labelValues),
		intCumulative(prefix+AttributeCPUUserModeUsage, UnitNanoSecond, &m.CPUUsageInUserMode, labelKeys, labelValues),
		intGauge(prefix+AttributeCPUCores, UnitCount, &m.NumOfCPUCores, labelKeys, labelValues),
		intGauge(prefix+AttributeCPUOnlines, UnitCount, &m.CPUOnlineCpus, labelKeys, labelValues),
		intCumulative(prefix+AttributeCPUSystemUsage, UnitNanoSecond, &m.SystemCPUUsage, labelKeys, labelValues),
		doubleGauge(prefix+AttributeCPUUtilized, UnitPercent, &m.CPUUtilized, labelKeys, labelValues),
		doubleGauge(prefix+AttributeCPUReserved, UnitVCpu, &m.CPUReserved, labelKeys, labelValues),
		doubleGauge(prefix+AttributeCPUUsageInVCPU, UnitVCpu, &m.CPUUsageInVCPU, labelKeys, labelValues),

		doubleGauge(prefix+AttributeNetworkRateRx, UnitBytesPerSec, &m.NetworkRateRxBytesPerSecond, labelKeys, labelValues),
		doubleGauge(prefix+AttributeNetworkRateTx, UnitBytesPerSec, &m.NetworkRateTxBytesPerSecond, labelKeys, labelValues),

		intCumulative(prefix+AttributeNetworkRxBytes, UnitBytes, &m.NetworkRxBytes, labelKeys, labelValues),
		intCumulative(prefix+AttributeNetworkRxPackets, UnitCount, &m.NetworkRxPackets, labelKeys, labelValues),
		intCumulative(prefix+AttributeNetworkRxErrors, UnitCount, &m.NetworkRxErrors, labelKeys, labelValues),
		intCumulative(prefix+AttributeNetworkRxDropped, UnitCount, &m.NetworkRxDropped, labelKeys, labelValues),
		intCumulative(prefix+AttributeNetworkTxBytes, UnitBytes, &m.NetworkTxBytes, labelKeys, labelValues),
		intCumulative(prefix+AttributeNetworkTxPackets, UnitCount, &m.NetworkTxPackets, labelKeys, labelValues),
		intCumulative(prefix+AttributeNetworkTxErrors, UnitCount, &m.NetworkTxErrors, labelKeys, labelValues),
		intCumulative(prefix+AttributeNetworkTxDropped, UnitCount, &m.NetworkTxDropped, labelKeys, labelValues),

		intCumulative(prefix+AttributeStorageRead, UnitBytes, &m.StorageReadBytes, labelKeys, labelValues),
		intCumulative(prefix+AttributeStorageWrite, UnitBytes, &m.StorageWriteBytes, labelKeys, labelValues),
	}, timestamp)
}

func intGauge(metricName string, unit string, value *uint64, labelKeys []*metricspb.LabelKey, labelValues []*metricspb.LabelValue) *metricspb.Metric {
	if value == nil {
		return nil
	}
	return &metricspb.Metric{
		MetricDescriptor: &metricspb.MetricDescriptor{
			Name:      metricName,
			Unit:      unit,
			Type:      metricspb.MetricDescriptor_GAUGE_INT64,
			LabelKeys: labelKeys,
		},
		Timeseries: []*metricspb.TimeSeries{{
			LabelValues: labelValues,
			Points: []*metricspb.Point{{
				Value: &metricspb.Point_Int64Value{
					Int64Value: int64(*value),
				},
			}},
		}},
	}
}

func doubleGauge(metricName string, unit string, value *float64, labelKeys []*metricspb.LabelKey, labelValues []*metricspb.LabelValue) *metricspb.Metric {
	if value == nil {
		return nil
	}
	return &metricspb.Metric{
		MetricDescriptor: &metricspb.MetricDescriptor{
			Name:      metricName,
			Unit:      unit,
			Type:      metricspb.MetricDescriptor_GAUGE_DOUBLE,
			LabelKeys: labelKeys,
		},
		Timeseries: []*metricspb.TimeSeries{{
			LabelValues: labelValues,
			Points: []*metricspb.Point{{
				Value: &metricspb.Point_DoubleValue{
					DoubleValue: *value,
				},
			}},
		}},
	}
}

func intCumulative(metricName string, unit string, value *uint64, labelKeys []*metricspb.LabelKey, labelValues []*metricspb.LabelValue) *metricspb.Metric {
	if value == nil {
		return nil
	}
	return &metricspb.Metric{
		MetricDescriptor: &metricspb.MetricDescriptor{
			Name:      metricName,
			Unit:      unit,
			Type:      metricspb.MetricDescriptor_CUMULATIVE_INT64,
			LabelKeys: labelKeys,
		},
		Timeseries: []*metricspb.TimeSeries{{
			LabelValues: labelValues,
			Points: []*metricspb.Point{{
				Value: &metricspb.Point_Int64Value{
					Int64Value: int64(*value),
				},
			}},
		}},
	}
}
