// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsdevicepodcorrelationprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/awsdevicepodcorrelationprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/awsdevicepodcorrelationprocessor/internal/kubelet"
)

const (
	k8sPodNameKey    = "k8s.pod.name"
	k8sNamespaceKey  = "k8s.namespace.name"
	containerNameKey = "k8s.container.name"
)

// deviceLookup is the interface used to look up device-to-pod mappings.
type deviceLookup interface {
	GetContainerInfo(deviceID, resourceName string) *kubelet.ContainerInfo
}

type devicePodCorrelationProcessor struct {
	config *Config
	logger *zap.Logger
	client *kubelet.Client
	lookup deviceLookup
}

func newProcessor(cfg *Config, logger *zap.Logger) *devicePodCorrelationProcessor {
	return &devicePodCorrelationProcessor{
		config: cfg,
		logger: logger,
	}
}

// Start creates the Kubelet Pod Resources API client and registers
// all configured resource names.
func (p *devicePodCorrelationProcessor) Start(ctx context.Context, _ component.Host) error {
	p.client = kubelet.NewClient(p.logger, kubelet.WithSocketPath(p.config.KubeletSocketPath))
	p.lookup = p.client

	seen := make(map[string]struct{})
	for _, dt := range p.config.DeviceTypes {
		for _, rn := range dt.ResourceNames {
			if _, ok := seen[rn]; !ok {
				seen[rn] = struct{}{}
				p.client.AddResourceName(rn)
			}
		}
	}

	return p.client.Start(ctx)
}

// Shutdown stops the kubelet client and releases resources.
func (p *devicePodCorrelationProcessor) Shutdown(_ context.Context) error {
	if p.client != nil {
		p.client.Stop()
	}
	return nil
}

// processMetrics iterates all datapoints in the metric batch and enriches them
// with pod/namespace/container attributes when a device ID matches a configured
// device type and the kubelet client has correlation data.
func (p *devicePodCorrelationProcessor) processMetrics(_ context.Context, md pmetric.Metrics) (pmetric.Metrics, error) {
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		resourceAttrs := rm.Resource().Attributes()
		ilms := rm.ScopeMetrics()
		for j := 0; j < ilms.Len(); j++ {
			metrics := ilms.At(j).Metrics()
			for k := 0; k < metrics.Len(); k++ {
				m := metrics.At(k)
				switch m.Type() {
				case pmetric.MetricTypeGauge:
					processDatapoints(m.Gauge().DataPoints(), resourceAttrs, p.config.DeviceTypes, p.lookup, p.logger)
				case pmetric.MetricTypeSum:
					processDatapoints(m.Sum().DataPoints(), resourceAttrs, p.config.DeviceTypes, p.lookup, p.logger)
				case pmetric.MetricTypeHistogram:
					processDatapoints(m.Histogram().DataPoints(), resourceAttrs, p.config.DeviceTypes, p.lookup, p.logger)
				case pmetric.MetricTypeExponentialHistogram:
					processDatapoints(m.ExponentialHistogram().DataPoints(), resourceAttrs, p.config.DeviceTypes, p.lookup, p.logger)
				case pmetric.MetricTypeSummary:
					processDatapoints(m.Summary().DataPoints(), resourceAttrs, p.config.DeviceTypes, p.lookup, p.logger)
				default:
				}
			}
		}
	}
	return md, nil
}

// processDatapoints enriches datapoints with pod correlation attributes.
func processDatapoints[DP interface{ Attributes() pcommon.Map }](
	datapoints interface {
		Len() int
		At(int) DP
	},
	resourceAttrs pcommon.Map,
	deviceTypes []DeviceTypeConfig,
	lookup deviceLookup,
	logger *zap.Logger,
) {
	for i := 0; i < datapoints.Len(); i++ {
		dpAttrs := datapoints.At(i).Attributes()

		if _, exists := dpAttrs.Get(k8sPodNameKey); exists {
			logger.Debug("Skipping datapoint, pod attributes already present")
			continue
		}

		for _, dt := range deviceTypes {
			var sourceAttrs pcommon.Map
			if dt.DeviceIDSource == DeviceIDSourceResource {
				sourceAttrs = resourceAttrs
			} else {
				sourceAttrs = dpAttrs
			}

			deviceIDVal, found := sourceAttrs.Get(dt.DeviceIDAttribute)
			if !found {
				continue
			}

			deviceID := deviceIDVal.AsString()
			var containerInfo *kubelet.ContainerInfo
			for _, rn := range dt.ResourceNames {
				containerInfo = lookup.GetContainerInfo(deviceID, rn)
				if containerInfo != nil {
					break
				}
			}

			if containerInfo != nil {
				logger.Debug("Correlated device to pod",
					zap.String("device_type", dt.Name),
					zap.String("device_id", deviceID),
					zap.String("pod", containerInfo.PodName),
					zap.String("namespace", containerInfo.Namespace),
					zap.String("container", containerInfo.ContainerName),
				)
				dpAttrs.PutStr(k8sPodNameKey, containerInfo.PodName)
				dpAttrs.PutStr(k8sNamespaceKey, containerInfo.Namespace)
				dpAttrs.PutStr(containerNameKey, containerInfo.ContainerName)
				break
			}
		}
	}
}
