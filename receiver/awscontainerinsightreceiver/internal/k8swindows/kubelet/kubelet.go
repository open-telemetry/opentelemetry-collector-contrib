// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package kubelet // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/k8swindows/kubelet"

import (
	"fmt"
	"os"
	"strconv"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	cExtractor "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor/extractors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/k8swindows/extractors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"

	"go.uber.org/zap"
	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"
)

// SummaryProvider represents receiver to get container metric from Kubelet.
type SummaryProvider struct {
	logger           *zap.Logger
	kubeletProvider  KubeletProvider
	hostInfo         cExtractor.CPUMemInfoProvider
	metricExtractors []extractors.MetricExtractor
}

func CreateDefaultKubeletProvider(logger *zap.Logger) KubeletProvider {
	return &kubeletProvider{logger: logger, hostIP: os.Getenv("HOST_IP"), hostPort: ci.KubeSecurePort}
}

// Options decorates SummaryProvider struct.
type Options func(provider *SummaryProvider)

func New(logger *zap.Logger, info cExtractor.CPUMemInfoProvider, mextractor []extractors.MetricExtractor, opts ...Options) (*SummaryProvider, error) {
	sp := &SummaryProvider{
		logger:           logger,
		hostInfo:         info,
		kubeletProvider:  CreateDefaultKubeletProvider(logger),
		metricExtractors: mextractor,
	}

	for _, opt := range opts {
		opt(sp)
	}

	return sp, nil
}

func (sp *SummaryProvider) GetMetrics() ([]*stores.CIMetricImpl, error) {
	var metrics []*stores.CIMetricImpl

	summary, err := sp.kubeletProvider.GetSummary()
	if err != nil {
		sp.logger.Error("failed to get summary from kubeletProvider, ", zap.Error(err))
		return nil, err
	}
	outMetrics, err := sp.getPodMetrics(summary)
	if err != nil {
		sp.logger.Error("failed to get pod metrics using kubelet summary, ", zap.Error(err))
		return metrics, err
	}
	metrics = append(metrics, outMetrics...)

	nodeMetrics, err := sp.getNodeMetrics(summary)
	if err != nil {
		sp.logger.Error("failed to get node metrics using kubelet summary, ", zap.Error(err))
		return nodeMetrics, err
	}
	metrics = append(metrics, nodeMetrics...)

	return metrics, nil
}

// getContainerMetrics returns container level metrics from kubelet summary.
func (sp *SummaryProvider) getContainerMetrics(pod stats.PodStats) ([]*stores.CIMetricImpl, error) {
	var metrics []*stores.CIMetricImpl

	for _, container := range pod.Containers {
		tags := map[string]string{}

		tags[ci.PodIDKey] = pod.PodRef.UID
		tags[ci.PodNameKey] = pod.PodRef.Name
		tags[ci.K8sNamespace] = pod.PodRef.Namespace
		tags[ci.ContainerNamekey] = container.Name
		containerID := fmt.Sprintf("%s-%s", pod.PodRef.UID, container.Name)
		tags[ci.ContainerIDkey] = containerID

		rawMetric := extractors.ConvertContainerToRaw(container, pod)
		tags[ci.Timestamp] = strconv.FormatInt(rawMetric.Time.UnixNano(), 10)

		for _, extractor := range sp.metricExtractors {
			if extractor.HasValue(rawMetric) {
				metrics = append(metrics, extractor.GetValue(rawMetric, sp.hostInfo, ci.TypeContainer)...)
			}
		}
		for _, metric := range metrics {
			metric.AddTags(tags)
		}
	}

	return metrics, nil
}

// getPodMetrics returns pod and container level metrics from kubelet summary.
func (sp *SummaryProvider) getPodMetrics(summary *stats.Summary) ([]*stores.CIMetricImpl, error) {
	var metrics []*stores.CIMetricImpl

	if summary == nil {
		return metrics, nil
	}

	for _, pod := range summary.Pods {
		var metricsPerPod []*stores.CIMetricImpl

		tags := map[string]string{}

		tags[ci.PodIDKey] = pod.PodRef.UID
		tags[ci.PodNameKey] = pod.PodRef.Name
		tags[ci.K8sNamespace] = pod.PodRef.Namespace

		rawMetric := extractors.ConvertPodToRaw(pod)
		tags[ci.Timestamp] = strconv.FormatInt(rawMetric.Time.UnixNano(), 10)

		for _, extractor := range sp.metricExtractors {
			if extractor.HasValue(rawMetric) {
				metricsPerPod = append(metricsPerPod, extractor.GetValue(rawMetric, sp.hostInfo, ci.TypePod)...)
			}
		}
		for _, metric := range metricsPerPod {
			metric.AddTags(tags)
		}
		metrics = append(metrics, metricsPerPod...)

		containerMetrics, err := sp.getContainerMetrics(pod)
		if err != nil {
			sp.logger.Error("failed to get container metrics, ", zap.Error(err))
			return containerMetrics, err
		}
		metrics = append(metrics, containerMetrics...)
	}
	return metrics, nil
}

// getNodeMetrics returns Node level metrics from kubelet summary.
func (sp *SummaryProvider) getNodeMetrics(summary *stats.Summary) ([]*stores.CIMetricImpl, error) {
	var metrics []*stores.CIMetricImpl

	if summary == nil {
		return metrics, nil
	}

	rawMetric := extractors.ConvertNodeToRaw(summary.Node)
	for _, extractor := range sp.metricExtractors {
		if extractor.HasValue(rawMetric) {
			metrics = append(metrics, extractor.GetValue(rawMetric, sp.hostInfo, ci.TypeNode)...)
		}
	}
	return metrics, nil
}
