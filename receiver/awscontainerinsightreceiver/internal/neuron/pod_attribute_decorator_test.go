// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package neuron

import (
	"context"
	"testing"

	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/prometheusscraper/decoratorconsumer"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"
)

var (
	dummyPodName               = "pod-name"
	dummyPodNameForAltResource = "pod-name-alt"
	dummyContainerName         = "container-name"
	dummyNamespace             = "namespace"
)

type mockPodResourcesStore struct{}

func (m mockPodResourcesStore) GetContainerInfo(_ string, _ string) *stores.ContainerInfo {
	return &stores.ContainerInfo{
		PodName:       dummyPodName,
		ContainerName: dummyContainerName,
		Namespace:     dummyNamespace,
	}
}

type mockPodResourcesStoreWithAltResourceName struct{}

func (m mockPodResourcesStoreWithAltResourceName) GetContainerInfo(_ string, resourceName string) *stores.ContainerInfo {
	if resourceName == neuronDeviceResourceNameAlt {
		return &stores.ContainerInfo{
			PodName:       dummyPodNameForAltResource,
			ContainerName: dummyContainerName,
			Namespace:     dummyNamespace,
		}
	}
	return nil
}

func TestConsumeMetricsForPodAttributeDecorator(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	dc := &PodAttributesDecoratorConsumer{
		NextConsumer:      consumertest.NewNop(),
		PodResourcesStore: mockPodResourcesStore{},
		Logger:            logger,
	}
	ctx := context.Background()

	testcases1 := map[string]decoratorconsumer.TestCase{
		"empty": {
			Metrics:     pmetric.NewMetrics(),
			Want:        pmetric.NewMetrics(),
			ShouldError: false,
		},
		"neuron_hardware_info_not_found": {
			Metrics: decoratorconsumer.GenerateMetrics(map[decoratorconsumer.MetricIdentifier][]map[string]string{
				{Name: "test", MetricType: pmetric.MetricTypeGauge}: {
					{
						"device": "test0",
					},
				},
			}),

			Want: decoratorconsumer.GenerateMetrics(map[decoratorconsumer.MetricIdentifier][]map[string]string{
				{Name: "test", MetricType: pmetric.MetricTypeGauge}: {
					{
						"device": "test0",
					},
				},
			}),
			ShouldError: false,
		},
		"correlation_via_neuron_device_index": {
			Metrics: decoratorconsumer.GenerateMetrics(map[decoratorconsumer.MetricIdentifier][]map[string]string{
				{Name: neuronHardwareInfoKey, MetricType: pmetric.MetricTypeSum}: {
					{
						neuronCorePerDeviceKey: "2",
					},
				},
				{Name: "test", MetricType: pmetric.MetricTypeGauge}: {
					{
						"device":                 "test0",
						neuronDeviceAttributeKey: "1",
					},
				},
			}),
			Want: decoratorconsumer.GenerateMetrics(map[decoratorconsumer.MetricIdentifier][]map[string]string{
				{Name: neuronHardwareInfoKey, MetricType: pmetric.MetricTypeSum}: {
					{
						neuronCorePerDeviceKey: "2",
					},
				},
				{Name: "test", MetricType: pmetric.MetricTypeGauge}: {
					{
						"device":                 "test0",
						neuronDeviceAttributeKey: "1",
						ci.ContainerNamekey:      dummyContainerName,
						ci.PodNameKey:            dummyPodName,
						ci.K8sNamespace:          dummyNamespace,
					},
				},
			}),
			ShouldError: false,
		},
		"correlation_via_neuron_core": {
			Metrics: decoratorconsumer.GenerateMetrics(map[decoratorconsumer.MetricIdentifier][]map[string]string{
				{Name: neuronHardwareInfoKey, MetricType: pmetric.MetricTypeSum}: {
					{
						neuronCorePerDeviceKey: "2",
					},
				},
				{Name: "test", MetricType: pmetric.MetricTypeGauge}: {
					{
						"device":               "test0",
						neuronCoreAttributeKey: "10",
					},
				},
			}),
			Want: decoratorconsumer.GenerateMetrics(map[decoratorconsumer.MetricIdentifier][]map[string]string{
				{Name: neuronHardwareInfoKey, MetricType: pmetric.MetricTypeSum}: {
					{
						neuronCorePerDeviceKey: "2",
					},
				},
				{Name: "test", MetricType: pmetric.MetricTypeGauge}: {
					{
						"device":                 "test0",
						neuronCoreAttributeKey:   "10",
						neuronDeviceAttributeKey: "5",
						ci.ContainerNamekey:      dummyContainerName,
						ci.PodNameKey:            dummyPodName,
						ci.K8sNamespace:          dummyNamespace,
					},
				},
			}),
			ShouldError: false,
		},
		"correlation_when_both_present": {
			Metrics: decoratorconsumer.GenerateMetrics(map[decoratorconsumer.MetricIdentifier][]map[string]string{
				{Name: neuronHardwareInfoKey, MetricType: pmetric.MetricTypeSum}: {
					{
						neuronCorePerDeviceKey: "2",
					},
				},
				{Name: "test", MetricType: pmetric.MetricTypeGauge}: {
					{
						"device":                 "test0",
						neuronDeviceAttributeKey: "5",
						neuronCoreAttributeKey:   "10",
					},
				},
			}),
			Want: decoratorconsumer.GenerateMetrics(map[decoratorconsumer.MetricIdentifier][]map[string]string{
				{Name: neuronHardwareInfoKey, MetricType: pmetric.MetricTypeSum}: {
					{
						neuronCorePerDeviceKey: "2",
					},
				},
				{Name: "test", MetricType: pmetric.MetricTypeGauge}: {
					{
						"device":                 "test0",
						neuronCoreAttributeKey:   "10",
						neuronDeviceAttributeKey: "5",
						ci.ContainerNamekey:      dummyContainerName,
						ci.PodNameKey:            dummyPodName,
						ci.K8sNamespace:          dummyNamespace,
					},
				},
			}),
			ShouldError: false,
		},
	}

	decoratorconsumer.RunDecoratorTestScenarios(ctx, t, dc, testcases1)

	dc = &PodAttributesDecoratorConsumer{
		NextConsumer:      consumertest.NewNop(),
		PodResourcesStore: mockPodResourcesStoreWithAltResourceName{},
		Logger:            logger,
	}

	testcases2 := map[string]decoratorconsumer.TestCase{
		"correlation_via_neuron_device_index_alt_name": {
			Metrics: decoratorconsumer.GenerateMetrics(map[decoratorconsumer.MetricIdentifier][]map[string]string{
				{Name: neuronHardwareInfoKey, MetricType: pmetric.MetricTypeSum}: {
					{
						neuronCorePerDeviceKey: "2",
					},
				},
				{Name: "test", MetricType: pmetric.MetricTypeGauge}: {
					{
						"device":                 "test0",
						neuronDeviceAttributeKey: "1",
					},
				},
			}),
			Want: decoratorconsumer.GenerateMetrics(map[decoratorconsumer.MetricIdentifier][]map[string]string{
				{Name: neuronHardwareInfoKey, MetricType: pmetric.MetricTypeSum}: {
					{
						neuronCorePerDeviceKey: "2",
					},
				},
				{Name: "test", MetricType: pmetric.MetricTypeGauge}: {
					{
						"device":                 "test0",
						neuronDeviceAttributeKey: "1",
						ci.ContainerNamekey:      dummyContainerName,
						ci.PodNameKey:            dummyPodNameForAltResource,
						ci.K8sNamespace:          dummyNamespace,
					},
				},
			}),
			ShouldError: false,
		},
	}

	decoratorconsumer.RunDecoratorTestScenarios(ctx, t, dc, testcases2)
}
