// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package k8swindows // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/k8swindows"

import (
	"context"
	"errors"
	"os"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	cExtractor "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor/extractors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/host"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/k8swindows/extractors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/k8swindows/hcsshim"
	kubeletsummaryprovider "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/k8swindows/kubelet"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"

	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type K8sWindows struct {
	cancel                 context.CancelFunc
	logger                 *zap.Logger
	nodeName               string `toml:"node_name"`
	k8sDecorator           stores.K8sDecorator
	kubeletSummaryProvider *kubeletsummaryprovider.SummaryProvider
	hcsshimStatsProvider   *hcsshim.HCSStatsProvider
	hostInfo               host.Info
	version                string
}

func New(logger *zap.Logger, decorator *stores.K8sDecorator, hostInfo host.Info) (*K8sWindows, error) {
	nodeName := os.Getenv("HOST_NAME")
	if nodeName == "" {
		return nil, errors.New("missing environment variable HOST_NAME. Please check your deployment YAML config")
	}

	metricsExtractors := []extractors.MetricExtractor{}
	metricsExtractors = append(metricsExtractors, extractors.NewCPUMetricExtractor(logger))
	metricsExtractors = append(metricsExtractors, extractors.NewMemMetricExtractor(logger))
	metricsExtractors = append(metricsExtractors, extractors.NewFileSystemMetricExtractor(logger))
	metricsExtractors = append(metricsExtractors, extractors.NewNetMetricExtractor(logger))

	ksp, err := kubeletsummaryprovider.New(logger, &hostInfo, metricsExtractors)
	if err != nil {
		logger.Error("failed to initialize kubelet SummaryProvider, ", zap.Error(err))
		return nil, err
	}

	hnsMetricsExtractors := append([]extractors.MetricExtractor{}, extractors.NewNetMetricExtractor(logger))
	hsp, err := hcsshim.NewHnSProvider(logger, &hostInfo, hnsMetricsExtractors)
	if err != nil {
		logger.Error("failed to initialize HCSShim SummaryProvider, ", zap.Error(err))
		return nil, err
	}

	return &K8sWindows{
		logger:                 logger,
		nodeName:               nodeName,
		k8sDecorator:           *decorator,
		kubeletSummaryProvider: ksp,
		hcsshimStatsProvider:   hsp,
		hostInfo:               hostInfo,
		version:                "0",
	}, nil
}

func (k *K8sWindows) GetMetrics() []pmetric.Metrics {
	k.logger.Debug("D! called K8sWindows GetMetrics")
	var result []pmetric.Metrics

	metrics, err := k.kubeletSummaryProvider.GetMetrics()
	if err != nil {
		k.logger.Error("failed to get metrics from kubelet SummaryProvider, ", zap.Error(err))
		return result
	}

	hcsmetrics, err := k.hcsshimStatsProvider.GetMetrics()
	if err != nil {
		k.logger.Error("failed to get metrics from HCSShim StatsProvider, ", zap.Error(err))
		return result
	}

	metrics = append(metrics, hcsmetrics...)
	metrics = cExtractor.MergeMetrics(metrics)
	metrics = k.decorateMetrics(metrics)
	for _, ciMetric := range metrics {
		md := ci.ConvertToOTLPMetrics(ciMetric.GetFields(), ciMetric.GetTags(), k.logger)
		result = append(result, md)
	}

	return result
}

func (k *K8sWindows) decorateMetrics(windowsmetrics []*stores.CIMetricImpl) []*stores.CIMetricImpl {
	// ebsVolumeIdsUsedAsPV := c.hostInfo.ExtractEbsIDsUsedByKubernetes()
	var result []*stores.CIMetricImpl
	for _, m := range windowsmetrics {
		tags := m.GetTags()
		// c.addEbsVolumeInfo(tags, ebsVolumeIdsUsedAsPV)

		// add version
		tags[ci.Version] = k.version

		// add nodeName for node, pod and container
		metricType := tags[ci.MetricType]
		if k.nodeName != "" && (ci.IsNode(metricType) || ci.IsInstance(metricType) ||
			ci.IsPod(metricType) || ci.IsContainer(metricType)) {
			tags[ci.NodeNameKey] = k.nodeName
		}

		// add instance id and type
		if instanceID := k.hostInfo.GetInstanceID(); instanceID != "" {
			tags[ci.InstanceID] = instanceID
		}
		if instanceType := k.hostInfo.GetInstanceType(); instanceType != "" {
			tags[ci.InstanceType] = instanceType
		}

		// add scaling group name
		tags[ci.AutoScalingGroupNameKey] = k.hostInfo.GetAutoScalingGroupName()

		// add tags for EKS
		tags[ci.ClusterNameKey] = k.hostInfo.GetClusterName()

		// add tags for OS
		tags[ci.OperatingSystem] = ci.OperatingSystemWindows

		out := k.k8sDecorator.Decorate(m)
		if out != nil {
			result = append(result, out.(*stores.CIMetricImpl))
		}
	}
	return result
}

func (k *K8sWindows) Shutdown() error {
	k.logger.Debug("D! called K8sWindows Shutdown")
	return nil
}
