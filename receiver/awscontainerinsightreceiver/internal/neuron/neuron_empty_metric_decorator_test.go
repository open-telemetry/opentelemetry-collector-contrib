// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package neuron

import (
	"context"
	"testing"

	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/prometheusscraper/decoratorconsumer"
)

func TestConsumeMetricsForNeuronEmptyMetricsDecorator(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	ned := &EmptyMetricDecorator{
		NextConsumer: consumertest.NewNop(),
		Logger:       logger,
	}
	ctx := context.Background()
	testcases := map[string]decoratorconsumer.TestCase{
		"empty": {
			Metrics:     pmetric.NewMetrics(),
			Want:        pmetric.NewMetrics(),
			ShouldError: false,
		},
		"neuron_hardware_info_not_found": {
			Metrics: decoratorconsumer.GenerateMetrics(map[decoratorconsumer.MetricIdentifier][]map[string]string{
				{Name: "test", MetricType: pmetric.MetricTypeGauge, DataValue: 1}: {
					{
						"device": "test0",
					},
				},
			}),

			Want: decoratorconsumer.GenerateMetrics(map[decoratorconsumer.MetricIdentifier][]map[string]string{
				{Name: "test", MetricType: pmetric.MetricTypeGauge, DataValue: 1}: {
					{
						"device": "test0",
					},
				},
			}),
			ShouldError: false,
		},
		"all_metrics_populated": {
			Metrics: decoratorconsumer.GenerateMetrics(map[decoratorconsumer.MetricIdentifier][]map[string]string{
				{Name: neuronHardwareInfoKey, MetricType: pmetric.MetricTypeSum, DataValue: 1}: {
					{
						neuronCorePerDeviceKey:        "2",
						neuronDeviceCountAttributeKey: "2",
					},
				},
			}),
			Want: decoratorconsumer.GenerateMetrics(map[decoratorconsumer.MetricIdentifier][]map[string]string{
				{Name: neuronHardwareInfoKey, MetricType: pmetric.MetricTypeSum, DataValue: 1}: {
					{
						neuronCorePerDeviceKey:        "2",
						neuronDeviceCountAttributeKey: "2",
					},
				},
				{Name: NeuronExecutionStatus, MetricType: pmetric.MetricTypeGauge, DataValue: 0}: {
					{
						neuronCorePerDeviceKey:        "2",
						neuronDeviceCountAttributeKey: "2",
						statusType:                    "completed",
						"runtime_tag":                 "DEFAULT",
					},
				},
				{Name: NeuronExecutionErrors, MetricType: pmetric.MetricTypeGauge, DataValue: 0}: {
					{
						neuronCorePerDeviceKey:        "2",
						neuronDeviceCountAttributeKey: "2",
						errorType:                     "generic",
						"runtime_tag":                 "DEFAULT",
					},
				},
				{Name: NeuronRuntimeMemoryUsage, MetricType: pmetric.MetricTypeGauge, DataValue: 0}: {
					{
						neuronCorePerDeviceKey:        "2",
						neuronDeviceCountAttributeKey: "2",
						memoryLocation:                "neuron_device",
						"runtime_tag":                 "DEFAULT",
					},
				},
				{Name: NeuronExecutionLatency, MetricType: pmetric.MetricTypeGauge, DataValue: 0}: {
					{
						neuronCorePerDeviceKey:        "2",
						neuronDeviceCountAttributeKey: "2",
						percentile:                    "p50",
						"runtime_tag":                 "DEFAULT",
					},
				},

				{Name: NeuronCoreUtilization, MetricType: pmetric.MetricTypeGauge, DataValue: 0}: {
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "0",
						neuronCoreOriginalAttributeKey: "0",
						neuronDeviceAttributeKey:       "0",
						"runtime_tag":                  "DEFAULT",
					},
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "1",
						neuronCoreOriginalAttributeKey: "1",
						neuronDeviceAttributeKey:       "0",
						"runtime_tag":                  "DEFAULT",
					},
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "2",
						neuronCoreOriginalAttributeKey: "2",
						neuronDeviceAttributeKey:       "1",
						"runtime_tag":                  "DEFAULT",
					},
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "3",
						neuronCoreOriginalAttributeKey: "3",
						neuronDeviceAttributeKey:       "1",
						"runtime_tag":                  "DEFAULT",
					},
				},

				{Name: NeuronCoreMemoryUtilizationConstants, MetricType: pmetric.MetricTypeGauge, DataValue: 0}: {
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "0",
						neuronCoreOriginalAttributeKey: "0",
						neuronDeviceAttributeKey:       "0",
						"runtime_tag":                  "DEFAULT",
					},
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "1",
						neuronCoreOriginalAttributeKey: "1",
						neuronDeviceAttributeKey:       "0",
						"runtime_tag":                  "DEFAULT",
					},
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "2",
						neuronCoreOriginalAttributeKey: "2",
						neuronDeviceAttributeKey:       "1",
						"runtime_tag":                  "DEFAULT",
					},
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "3",
						neuronCoreOriginalAttributeKey: "3",
						neuronDeviceAttributeKey:       "1",
						"runtime_tag":                  "DEFAULT",
					},
				},

				{Name: NeuronCoreMemoryUtilizationModelCode, MetricType: pmetric.MetricTypeGauge, DataValue: 0}: {
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "0",
						neuronCoreOriginalAttributeKey: "0",
						neuronDeviceAttributeKey:       "0",
						"runtime_tag":                  "DEFAULT",
					},
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "1",
						neuronCoreOriginalAttributeKey: "1",
						neuronDeviceAttributeKey:       "0",
						"runtime_tag":                  "DEFAULT",
					},
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "2",
						neuronCoreOriginalAttributeKey: "2",
						neuronDeviceAttributeKey:       "1",
						"runtime_tag":                  "DEFAULT",
					},
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "3",
						neuronCoreOriginalAttributeKey: "3",
						neuronDeviceAttributeKey:       "1",
						"runtime_tag":                  "DEFAULT",
					},
				},

				{Name: NeuronCoreMemoryUtilizationSharedScratchpad, MetricType: pmetric.MetricTypeGauge, DataValue: 0}: {
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "0",
						neuronCoreOriginalAttributeKey: "0",
						neuronDeviceAttributeKey:       "0",
						"runtime_tag":                  "DEFAULT",
					},
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "1",
						neuronCoreOriginalAttributeKey: "1",
						neuronDeviceAttributeKey:       "0",
						"runtime_tag":                  "DEFAULT",
					},
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "2",
						neuronCoreOriginalAttributeKey: "2",
						neuronDeviceAttributeKey:       "1",
						"runtime_tag":                  "DEFAULT",
					},
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "3",
						neuronCoreOriginalAttributeKey: "3",
						neuronDeviceAttributeKey:       "1",
						"runtime_tag":                  "DEFAULT",
					},
				},

				{Name: NeuronCoreMemoryUtilizationRuntimeMemory, MetricType: pmetric.MetricTypeGauge, DataValue: 0}: {
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "0",
						neuronCoreOriginalAttributeKey: "0",
						neuronDeviceAttributeKey:       "0",
						"runtime_tag":                  "DEFAULT",
					},
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "1",
						neuronCoreOriginalAttributeKey: "1",
						neuronDeviceAttributeKey:       "0",
						"runtime_tag":                  "DEFAULT",
					},
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "2",
						neuronCoreOriginalAttributeKey: "2",
						neuronDeviceAttributeKey:       "1",
						"runtime_tag":                  "DEFAULT",
					},
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "3",
						neuronCoreOriginalAttributeKey: "3",
						neuronDeviceAttributeKey:       "1",
						"runtime_tag":                  "DEFAULT",
					},
				},

				{Name: NeuronCoreMemoryUtilizationTensors, MetricType: pmetric.MetricTypeGauge, DataValue: 0}: {
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "0",
						neuronCoreOriginalAttributeKey: "0",
						neuronDeviceAttributeKey:       "0",
						"runtime_tag":                  "DEFAULT",
					},
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "1",
						neuronCoreOriginalAttributeKey: "1",
						neuronDeviceAttributeKey:       "0",
						"runtime_tag":                  "DEFAULT",
					},
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "2",
						neuronCoreOriginalAttributeKey: "2",
						neuronDeviceAttributeKey:       "1",
						"runtime_tag":                  "DEFAULT",
					},
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "3",
						neuronCoreOriginalAttributeKey: "3",
						neuronDeviceAttributeKey:       "1",
						"runtime_tag":                  "DEFAULT",
					},
				},
			}),
			ShouldError: false,
		},
		"some_metrics_populated": {
			Metrics: decoratorconsumer.GenerateMetrics(map[decoratorconsumer.MetricIdentifier][]map[string]string{
				{Name: neuronHardwareInfoKey, MetricType: pmetric.MetricTypeSum, DataValue: 1}: {
					{
						neuronCorePerDeviceKey:        "2",
						neuronDeviceCountAttributeKey: "2",
					},
				},
				{Name: NeuronExecutionStatus, MetricType: pmetric.MetricTypeGauge, DataValue: 1234}: {
					{
						statusType:    "completed",
						"runtime_tag": "123",
					},
				},
				{Name: NeuronExecutionErrors, MetricType: pmetric.MetricTypeGauge, DataValue: 1111}: {
					{
						errorType:     "generic",
						"runtime_tag": "123",
					},
				},
			}),
			Want: decoratorconsumer.GenerateMetrics(map[decoratorconsumer.MetricIdentifier][]map[string]string{
				{Name: neuronHardwareInfoKey, MetricType: pmetric.MetricTypeSum, DataValue: 1}: {
					{
						neuronCorePerDeviceKey:        "2",
						neuronDeviceCountAttributeKey: "2",
					},
				},
				{Name: NeuronExecutionStatus, MetricType: pmetric.MetricTypeGauge, DataValue: 1234}: {
					{
						statusType:             "completed",
						"runtime_tag":          "123",
						neuronCorePerDeviceKey: "2",
					},
				},
				{Name: NeuronExecutionErrors, MetricType: pmetric.MetricTypeGauge, DataValue: 1111}: {
					{
						errorType:              "generic",
						"runtime_tag":          "123",
						neuronCorePerDeviceKey: "2",
					},
				},
				{Name: NeuronRuntimeMemoryUsage, MetricType: pmetric.MetricTypeGauge, DataValue: 0}: {
					{
						neuronCorePerDeviceKey:        "2",
						neuronDeviceCountAttributeKey: "2",
						memoryLocation:                "neuron_device",
						"runtime_tag":                 "DEFAULT",
					},
				},
				{Name: NeuronExecutionLatency, MetricType: pmetric.MetricTypeGauge, DataValue: 0}: {
					{
						neuronCorePerDeviceKey:        "2",
						neuronDeviceCountAttributeKey: "2",
						percentile:                    "p50",
						"runtime_tag":                 "DEFAULT",
					},
				},

				{Name: NeuronCoreUtilization, MetricType: pmetric.MetricTypeGauge, DataValue: 0}: {
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "0",
						neuronCoreOriginalAttributeKey: "0",
						neuronDeviceAttributeKey:       "0",
						"runtime_tag":                  "DEFAULT",
					},
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "1",
						neuronCoreOriginalAttributeKey: "1",
						neuronDeviceAttributeKey:       "0",
						"runtime_tag":                  "DEFAULT",
					},
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "2",
						neuronCoreOriginalAttributeKey: "2",
						neuronDeviceAttributeKey:       "1",
						"runtime_tag":                  "DEFAULT",
					},
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "3",
						neuronCoreOriginalAttributeKey: "3",
						neuronDeviceAttributeKey:       "1",
						"runtime_tag":                  "DEFAULT",
					},
				},

				{Name: NeuronCoreMemoryUtilizationConstants, MetricType: pmetric.MetricTypeGauge, DataValue: 0}: {
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "0",
						neuronCoreOriginalAttributeKey: "0",
						neuronDeviceAttributeKey:       "0",
						"runtime_tag":                  "DEFAULT",
					},
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "1",
						neuronCoreOriginalAttributeKey: "1",
						neuronDeviceAttributeKey:       "0",
						"runtime_tag":                  "DEFAULT",
					},
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "2",
						neuronCoreOriginalAttributeKey: "2",
						neuronDeviceAttributeKey:       "1",
						"runtime_tag":                  "DEFAULT",
					},
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "3",
						neuronCoreOriginalAttributeKey: "3",
						neuronDeviceAttributeKey:       "1",
						"runtime_tag":                  "DEFAULT",
					},
				},

				{Name: NeuronCoreMemoryUtilizationModelCode, MetricType: pmetric.MetricTypeGauge, DataValue: 0}: {
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "0",
						neuronCoreOriginalAttributeKey: "0",
						neuronDeviceAttributeKey:       "0",
						"runtime_tag":                  "DEFAULT",
					},
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "1",
						neuronCoreOriginalAttributeKey: "1",
						neuronDeviceAttributeKey:       "0",
						"runtime_tag":                  "DEFAULT",
					},
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "2",
						neuronCoreOriginalAttributeKey: "2",
						neuronDeviceAttributeKey:       "1",
						"runtime_tag":                  "DEFAULT",
					},
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "3",
						neuronCoreOriginalAttributeKey: "3",
						neuronDeviceAttributeKey:       "1",
						"runtime_tag":                  "DEFAULT",
					},
				},

				{Name: NeuronCoreMemoryUtilizationSharedScratchpad, MetricType: pmetric.MetricTypeGauge, DataValue: 0}: {
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "0",
						neuronCoreOriginalAttributeKey: "0",
						neuronDeviceAttributeKey:       "0",
						"runtime_tag":                  "DEFAULT",
					},
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "1",
						neuronCoreOriginalAttributeKey: "1",
						neuronDeviceAttributeKey:       "0",
						"runtime_tag":                  "DEFAULT",
					},
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "2",
						neuronCoreOriginalAttributeKey: "2",
						neuronDeviceAttributeKey:       "1",
						"runtime_tag":                  "DEFAULT",
					},
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "3",
						neuronCoreOriginalAttributeKey: "3",
						neuronDeviceAttributeKey:       "1",
						"runtime_tag":                  "DEFAULT",
					},
				},

				{Name: NeuronCoreMemoryUtilizationRuntimeMemory, MetricType: pmetric.MetricTypeGauge, DataValue: 0}: {
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "0",
						neuronCoreOriginalAttributeKey: "0",
						neuronDeviceAttributeKey:       "0",
						"runtime_tag":                  "DEFAULT",
					},
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "1",
						neuronCoreOriginalAttributeKey: "1",
						neuronDeviceAttributeKey:       "0",
						"runtime_tag":                  "DEFAULT",
					},
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "2",
						neuronCoreOriginalAttributeKey: "2",
						neuronDeviceAttributeKey:       "1",
						"runtime_tag":                  "DEFAULT",
					},
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "3",
						neuronCoreOriginalAttributeKey: "3",
						neuronDeviceAttributeKey:       "1",
						"runtime_tag":                  "DEFAULT",
					},
				},

				{Name: NeuronCoreMemoryUtilizationTensors, MetricType: pmetric.MetricTypeGauge, DataValue: 0}: {
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "0",
						neuronCoreOriginalAttributeKey: "0",
						neuronDeviceAttributeKey:       "0",
						"runtime_tag":                  "DEFAULT",
					},
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "1",
						neuronCoreOriginalAttributeKey: "1",
						neuronDeviceAttributeKey:       "0",
						"runtime_tag":                  "DEFAULT",
					},
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "2",
						neuronCoreOriginalAttributeKey: "2",
						neuronDeviceAttributeKey:       "1",
						"runtime_tag":                  "DEFAULT",
					},
					{
						neuronCorePerDeviceKey:         "2",
						neuronDeviceCountAttributeKey:  "2",
						neuronCoreAttributeKey:         "3",
						neuronCoreOriginalAttributeKey: "3",
						neuronDeviceAttributeKey:       "1",
						"runtime_tag":                  "DEFAULT",
					},
				},
			}),
			ShouldError: false,
		},
	}

	decoratorconsumer.RunDecoratorTestScenarios(ctx, t, ned, testcases)
}
