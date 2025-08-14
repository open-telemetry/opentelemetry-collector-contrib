// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package neuron // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/neuron"

import (
	"context"
	"strconv"
	"strings"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

const (
	statusType                 = "status_type"
	errorType                  = "error_type"
	memoryLocation             = "memory_location"
	percentile                 = "percentile"
	RuntimeTagOverride         = "DEFAULT"
	DefaultNeuronCorePerDevice = 2
)

var attributeConfig = map[string][]string{
	NeuronExecutionStatus:                       {statusType},
	NeuronExecutionErrors:                       {errorType},
	NeuronRuntimeMemoryUsage:                    {memoryLocation},
	NeuronExecutionLatency:                      {percentile},
	NeuronCoreUtilization:                       {neuronCoreAttributeKey, neuronDeviceAttributeKey},
	NeuronCoreMemoryUtilizationConstants:        {neuronCoreAttributeKey, neuronDeviceAttributeKey},
	NeuronCoreMemoryUtilizationModelCode:        {neuronCoreAttributeKey, neuronDeviceAttributeKey},
	NeuronCoreMemoryUtilizationSharedScratchpad: {neuronCoreAttributeKey, neuronDeviceAttributeKey},
	NeuronCoreMemoryUtilizationRuntimeMemory:    {neuronCoreAttributeKey, neuronDeviceAttributeKey},
	NeuronCoreMemoryUtilizationTensors:          {neuronCoreAttributeKey, neuronDeviceAttributeKey},
}

var nonCoreAttributeValues = map[string]string{
	statusType:     "completed",
	errorType:      "generic",
	memoryLocation: "neuron_device",
	percentile:     "p50",
}

// The decorator is used to add metric with zero dataPoint values, if not present
// This allows non-sparse metrics in cases when neuron monitor is not running
type EmptyMetricDecorator struct {
	NextConsumer consumer.Metrics
	Logger       *zap.Logger
}

func (ed *EmptyMetricDecorator) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{
		MutatesData: true,
	}
}

func (ed *EmptyMetricDecorator) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rs := rms.At(i)
		ilms := rs.ScopeMetrics()
		for j := 0; j < ilms.Len(); j++ {
			ils := ilms.At(j)
			metrics := ils.Metrics()

			neuronHardwareInfo, neuronHardwareInfoFound := findNeuronHardwareInfo(metrics)
			if neuronHardwareInfoFound {
				ed.addEmptyMetrics(neuronHardwareInfo, metrics)
				neuronCoresPerDevice, foundCoresPerDevice := getNeuronCoresPerDevice(neuronHardwareInfo)
				if foundCoresPerDevice {
					ed.addNeuronCorePerDeviceAttribute(metrics, neuronCoresPerDevice)
				} else {
					// Always add the Default if the above is not found, should never happen
					ed.addNeuronCorePerDeviceAttribute(metrics, DefaultNeuronCorePerDevice)
				}
			}
		}
	}
	return ed.NextConsumer.ConsumeMetrics(ctx, md)
}

func (ed *EmptyMetricDecorator) addEmptyMetrics(hardwareInfo pmetric.Metric, metrics pmetric.MetricSlice) {
	metricFoundMap := make(map[string]bool)
	for k := range attributeConfig {
		metricFoundMap[k] = false
	}

	for i := 0; i < metrics.Len(); i++ {
		m := metrics.At(i)
		if _, ok := metricFoundMap[m.Name()]; ok {
			metricFoundMap[m.Name()] = true
		}
	}

	for k, v := range metricFoundMap {
		if v {
			continue
		}
		if strings.Contains(k, "core") {
			populateCoreMetrics(metrics, k, hardwareInfo)
		} else {
			populateNonCoreMetrics(metrics, k, attributeConfig[k], hardwareInfo)
		}
	}
}

// method populates per non-core metrics, thus empty metrics are added only once per node
func populateNonCoreMetrics(metrics pmetric.MetricSlice, metricName string, attributesToAdd []string, hardwareInfo pmetric.Metric) {
	metricToAdd := createNewMetricFromHardwareInfo(hardwareInfo, metricName)
	metricBody := metricToAdd.Gauge().DataPoints().At(0)

	for _, attribute := range attributesToAdd {
		metricBody.Attributes().PutStr(attribute, nonCoreAttributeValues[attribute])
	}

	metricToAdd.CopyTo(metrics.AppendEmpty())
}

// method populates per core metrics, thus empty metrics are added per core
func populateCoreMetrics(metrics pmetric.MetricSlice, metricName string, hardwareInfo pmetric.Metric) {
	neuronCoresPerDevice, foundCoresPerDevice := getNeuronCoresPerDevice(hardwareInfo)
	neuronDeviceCount, foundDeviceCount := getNeuronDeviceCount(hardwareInfo)
	if !foundCoresPerDevice || !foundDeviceCount {
		return
	}

	metricToAdd := pmetric.NewMetric()
	metricToAdd.SetEmptyGauge()
	metricToAdd.SetName(metricName)
	emptyDatapoints := metricToAdd.Gauge().DataPoints()
	for coreIndex := 0; coreIndex < neuronCoresPerDevice*neuronDeviceCount; coreIndex++ {
		datapoint := emptyDatapoints.AppendEmpty()
		hardwareInfo.Sum().DataPoints().At(0).CopyTo(datapoint)
		datapoint.SetDoubleValue(0)
		datapoint.Attributes().PutStr(neuronCoreAttributeKey, strconv.Itoa(coreIndex))
		datapoint.Attributes().PutStr(neuronCoreOriginalAttributeKey, strconv.Itoa(coreIndex))
		datapoint.Attributes().PutStr(neuronDeviceAttributeKey, strconv.Itoa(coreIndex/neuronCoresPerDevice))
		datapoint.Attributes().PutStr("runtime_tag", RuntimeTagOverride)
	}

	metricToAdd.CopyTo(metrics.AppendEmpty())
}

// returns the device count for neuron from the hardwareInfo metric
// https://awsdocs-neuron.readthedocs-hosted.com/en/latest/tools/neuron-sys-tools/neuron-monitor-user-guide.html#neuron-hw-counters
func getNeuronDeviceCount(hardwareInfo pmetric.Metric) (int, bool) {
	neuronCoreHardwareInfoDatapoints := hardwareInfo.Sum().DataPoints()
	if neuronCoreHardwareInfoDatapoints.Len() > 0 {
		neuronDeviceCountValue, found := neuronCoreHardwareInfoDatapoints.At(0).Attributes().Get(neuronDeviceCountAttributeKey)
		if found {
			neuronDeviceCount, _ := strconv.Atoi(neuronDeviceCountValue.AsString())
			return neuronDeviceCount, true
		}
	}
	return -1, false
}

// returns a empty gauge metric with all attributes of hardwareInfo metric copied
func createNewMetricFromHardwareInfo(hardwareInfo pmetric.Metric, metricName string) pmetric.Metric {
	metricToAdd := pmetric.NewMetric()
	metricToAdd.SetEmptyGauge()
	hardwareInfo.Sum().DataPoints().CopyTo(metricToAdd.Gauge().DataPoints())

	metricToAdd.SetName(metricName)
	metricBody := metricToAdd.Gauge().DataPoints().At(0)
	metricBody.SetDoubleValue(0)
	metricBody.Attributes().PutStr("runtime_tag", RuntimeTagOverride)

	return metricToAdd
}

// method to add neuroncore_per_device_count attribute to all metrics
func (ed *EmptyMetricDecorator) addNeuronCorePerDeviceAttribute(metrics pmetric.MetricSlice, neuronCoresPerDevice int) {
	attributeValue := strconv.Itoa(neuronCoresPerDevice)
	for i := 0; i < metrics.Len(); i++ {
		m := metrics.At(i)
		switch m.Type() {
		case pmetric.MetricTypeGauge:
			ed.addAttributeToNumberDataPoints(m.Gauge().DataPoints(), attributeValue)
		case pmetric.MetricTypeSum:
			ed.addAttributeToNumberDataPoints(m.Sum().DataPoints(), attributeValue)
		case pmetric.MetricTypeHistogram:
			dataPoints := m.Histogram().DataPoints()
			for j := 0; j < dataPoints.Len(); j++ {
				dataPoints.At(j).Attributes().PutStr(neuronCorePerDeviceKey, attributeValue)
			}
		case pmetric.MetricTypeSummary:
			dataPoints := m.Summary().DataPoints()
			for j := 0; j < dataPoints.Len(); j++ {
				dataPoints.At(j).Attributes().PutStr(neuronCorePerDeviceKey, attributeValue)
			}
		default:
			ed.Logger.Warn("Metric type not supported", zap.String("metricType", m.Type().String()))
		}
	}
}

func (ed *EmptyMetricDecorator) addAttributeToNumberDataPoints(dataPoints pmetric.NumberDataPointSlice, attributeValue string) {
	for j := 0; j < dataPoints.Len(); j++ {
		dataPoints.At(j).Attributes().PutStr(neuronCorePerDeviceKey, attributeValue)
	}
}
