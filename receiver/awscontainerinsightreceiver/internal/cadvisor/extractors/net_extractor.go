// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package extractors // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor/extractors"

import (
	"time"

	cinfo "github.com/google/cadvisor/info/v1"
	"go.uber.org/zap"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	awsmetrics "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/metrics"
)

type NetMetricExtractor struct {
	logger         *zap.Logger
	rateCalculator awsmetrics.MetricCalculator
}

func getInterfacesStats(stats *cinfo.ContainerStats) []cinfo.InterfaceStats {
	ifceStats := stats.Network.Interfaces
	if len(ifceStats) == 0 {
		ifceStats = []cinfo.InterfaceStats{stats.Network.InterfaceStats}
	}
	return ifceStats
}

func (n *NetMetricExtractor) HasValue(info *cinfo.ContainerInfo) bool {
	return info.Spec.HasNetwork
}

func (n *NetMetricExtractor) GetValue(info *cinfo.ContainerInfo, _ CPUMemInfoProvider, containerType string) []*CAdvisorMetric {

	// Just a protection here, there is no Container level Net metrics
	if containerType == ci.TypePod || containerType == ci.TypeContainer {
		return nil
	}

	// Rename type to pod so the metric name prefix is pod_
	if containerType == ci.TypeInfraContainer {
		containerType = ci.TypePod
	}

	curStats := GetStats(info)
	curIfceStats := getInterfacesStats(curStats)

	// used for aggregation
	netIfceMetrics := make([]map[string]any, len(curIfceStats))
	metrics := make([]*CAdvisorMetric, len(curIfceStats))

	for i, cur := range curIfceStats {
		mType := getNetMetricType(containerType, n.logger)
		netIfceMetric := make(map[string]any)

		infoName := info.Name + containerType + cur.Name // used to identify the network interface
		multiplier := float64(time.Second)
		assignRateValueToField(&n.rateCalculator, netIfceMetric, ci.NetRxBytes, infoName, float64(cur.RxBytes), curStats.Timestamp, multiplier)
		assignRateValueToField(&n.rateCalculator, netIfceMetric, ci.NetRxPackets, infoName, float64(cur.RxPackets), curStats.Timestamp, multiplier)
		assignRateValueToField(&n.rateCalculator, netIfceMetric, ci.NetRxDropped, infoName, float64(cur.RxDropped), curStats.Timestamp, multiplier)
		assignRateValueToField(&n.rateCalculator, netIfceMetric, ci.NetRxErrors, infoName, float64(cur.RxErrors), curStats.Timestamp, multiplier)
		assignRateValueToField(&n.rateCalculator, netIfceMetric, ci.NetTxBytes, infoName, float64(cur.TxBytes), curStats.Timestamp, multiplier)
		assignRateValueToField(&n.rateCalculator, netIfceMetric, ci.NetTxPackets, infoName, float64(cur.TxPackets), curStats.Timestamp, multiplier)
		assignRateValueToField(&n.rateCalculator, netIfceMetric, ci.NetTxDropped, infoName, float64(cur.TxDropped), curStats.Timestamp, multiplier)
		assignRateValueToField(&n.rateCalculator, netIfceMetric, ci.NetTxErrors, infoName, float64(cur.TxErrors), curStats.Timestamp, multiplier)

		if netIfceMetric[ci.NetRxBytes] != nil && netIfceMetric[ci.NetTxBytes] != nil {
			netIfceMetric[ci.NetTotalBytes] = netIfceMetric[ci.NetRxBytes].(float64) + netIfceMetric[ci.NetTxBytes].(float64)
		}

		netIfceMetrics[i] = netIfceMetric

		metric := newCadvisorMetric(mType, n.logger)
		metric.tags[ci.NetIfce] = cur.Name
		for k, v := range netIfceMetric {
			metric.fields[ci.MetricName(mType, k)] = v
		}

		metrics[i] = metric
	}

	aggregatedFields := ci.SumFields(netIfceMetrics)
	if len(aggregatedFields) > 0 {
		metric := newCadvisorMetric(containerType, n.logger)
		for k, v := range aggregatedFields {
			metric.fields[ci.MetricName(containerType, k)] = v
		}
		metrics = append(metrics, metric)
	}

	return metrics
}

func (n *NetMetricExtractor) Shutdown() error {
	return n.rateCalculator.Shutdown()
}

func NewNetMetricExtractor(logger *zap.Logger) *NetMetricExtractor {
	return &NetMetricExtractor{
		logger:         logger,
		rateCalculator: newFloat64RateCalculator(),
	}
}

func getNetMetricType(containerType string, logger *zap.Logger) string {
	metricType := ""
	switch containerType {
	case ci.TypeNode:
		metricType = ci.TypeNodeNet
	case ci.TypeInstance:
		metricType = ci.TypeInstanceNet
	case ci.TypePod:
		metricType = ci.TypePodNet
	default:
		logger.Warn("net_extractor: net metric extractor is parsing unexpected containerType", zap.String("containerType", containerType))
	}
	return metricType
}
