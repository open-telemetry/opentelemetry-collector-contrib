// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package neuron // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/neuron"

import (
	"context"
	"strconv"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"
)

const (
	neuronHardwareInfoKey         = "neuron_hardware"
	neuronCorePerDeviceKey        = "neuroncore_per_device_count"
	neuronCoreAttributeKey        = "NeuronCore"
	neuronDeviceCountAttributeKey = "neuron_device_count"
	neuronDeviceAttributeKey      = "NeuronDevice"
	neuronCoreResourceName        = "aws.amazon.com/neuroncore"
	neuronDeviceResourceName      = "aws.amazon.com/neurondevice"
	neuronDeviceResourceNameAlt   = "aws.amazon.com/neuron"
)

type PodResourcesStoreInterface interface {
	GetContainerInfo(string, string) *stores.ContainerInfo
}

type PodAttributesDecoratorConsumer struct {
	NextConsumer      consumer.Metrics
	PodResourcesStore PodResourcesStoreInterface
	Logger            *zap.Logger
}

func (pdc *PodAttributesDecoratorConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{
		MutatesData: true,
	}
}

func (pdc *PodAttributesDecoratorConsumer) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	pdc.neuronMetricsProcess(md)
	return pdc.NextConsumer.ConsumeMetrics(ctx, md)
}

func (pdc *PodAttributesDecoratorConsumer) neuronMetricsProcess(md pmetric.Metrics) {
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rs := rms.At(i)
		ilms := rs.ScopeMetrics()
		for j := 0; j < ilms.Len(); j++ {
			ils := ilms.At(j)
			metrics := ils.Metrics()

			neuronHardwareInfo, neuronHardwareInfoFound := findNeuronHardwareInfo(metrics)
			if neuronHardwareInfoFound {
				neuronCoresPerDevice, extracted := getNeuronCoresPerDevice(neuronHardwareInfo)
				if extracted {
					for k := 0; k < metrics.Len(); k++ {
						m := metrics.At(k)
						pdc.addPodCorrelationAttributes(getMetricDatapoints(m), neuronCoresPerDevice)
					}
				}
			}
		}
	}
}

func findNeuronHardwareInfo(metrics pmetric.MetricSlice) (pmetric.Metric, bool) {
	var neuronHardwareInfo pmetric.Metric
	neuronHardwareInfoFound := false
	for k := 0; k < metrics.Len(); k++ {
		m := metrics.At(k)
		if m.Name() == neuronHardwareInfoKey {
			neuronHardwareInfo = m
			neuronHardwareInfoFound = true
			break
		}
	}
	return neuronHardwareInfo, neuronHardwareInfoFound
}

func (pdc *PodAttributesDecoratorConsumer) addPodCorrelationAttributes(metricDatapoints pmetric.NumberDataPointSlice, neuronCoresPerDevice int) {
	for i := 0; i < metricDatapoints.Len(); i++ {
		attributes := metricDatapoints.At(i).Attributes()
		var containerInfo *stores.ContainerInfo

		if neuronDeviceIndex, neuronDeviceIndexPresent := attributes.Get(neuronDeviceAttributeKey); neuronDeviceIndexPresent {
			// get container info from neuronDeviceIndex
			neuronDeviceIndex := neuronDeviceIndex.AsString()
			containerInfo = pdc.getContainerInfoForNeuronDeviceIndex(neuronDeviceIndex)
		} else if neuronCoreIndex, neuronCoreIndexPresent := attributes.Get(neuronCoreAttributeKey); neuronCoreIndexPresent {
			// get container info from neuronCore
			containerInfo = pdc.PodResourcesStore.GetContainerInfo(neuronCoreIndex.AsString(), neuronCoreResourceName)
			neuronDeviceIndex := getNeuronDeviceIndexFromCoreAttribute(neuronCoreIndex, neuronCoresPerDevice)
			if containerInfo == nil {
				// else get container info from calculated neuronDeviceIndex
				containerInfo = pdc.getContainerInfoForNeuronDeviceIndex(neuronDeviceIndex)
			}
			attributes.PutStr(neuronDeviceAttributeKey, neuronDeviceIndex)
		}
		populateAttributes(&attributes, containerInfo)
	}
}

func (pdc *PodAttributesDecoratorConsumer) getContainerInfoForNeuronDeviceIndex(neuronDeviceIndex string) *stores.ContainerInfo {
	containerInfo := pdc.PodResourcesStore.GetContainerInfo(neuronDeviceIndex, neuronDeviceResourceName)
	if containerInfo == nil {
		// Alt resource name is to support backward compatibility in neuron monitor : https://awsdocs-neuron.readthedocs-hosted.com/en/latest/containers/tutorials/k8s-setup.html
		containerInfo = pdc.PodResourcesStore.GetContainerInfo(neuronDeviceIndex, neuronDeviceResourceNameAlt)
	}
	return containerInfo
}

func populateAttributes(attributes *pcommon.Map, containerInfo *stores.ContainerInfo) {
	if containerInfo != nil {
		attributes.PutStr(ci.ContainerNamekey, containerInfo.ContainerName)
		attributes.PutStr(ci.PodNameKey, containerInfo.PodName)
		attributes.PutStr(ci.K8sNamespace, containerInfo.Namespace)
	}
}

func getMetricDatapoints(m pmetric.Metric) pmetric.NumberDataPointSlice {
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		return m.Gauge().DataPoints()
	case pmetric.MetricTypeSum:
		return m.Sum().DataPoints()
	default:
		return pmetric.NewNumberDataPointSlice()
	}
}

// We extract the attribute named `neuroncore_per_device_count` from the metric to get the value
// https://awsdocs-neuron.readthedocs-hosted.com/en/latest/tools/neuron-sys-tools/neuron-monitor-user-guide.html
func getNeuronCoresPerDevice(neuronHardwareInfo pmetric.Metric) (int, bool) {
	neuronCoreHardwareInfoDatapoints := neuronHardwareInfo.Sum().DataPoints()
	if neuronCoreHardwareInfoDatapoints.Len() > 0 {
		neuronCoresPerDeviceValue, found := neuronCoreHardwareInfoDatapoints.At(0).Attributes().Get(neuronCorePerDeviceKey)
		if found {
			neuronCoresPerDevice, _ := strconv.Atoi(neuronCoresPerDeviceValue.AsString())
			return neuronCoresPerDevice, true
		}
	}
	return -1, false
}

// To get the device index from core index we divide the index by cores in a single device
// https://awsdocs-neuron.readthedocs-hosted.com/en/latest/tools/neuron-sys-tools/neuron-monitor-user-guide.html
func getNeuronDeviceIndexFromCoreAttribute(neuronCoreIndex pcommon.Value, neuronCoresPerDevice int) string {
	neuronCoreIndexIntVal, _ := strconv.Atoi(neuronCoreIndex.AsString())
	return strconv.Itoa(neuronCoreIndexIntVal / neuronCoresPerDevice)
}
