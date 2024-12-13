// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package decoratorconsumer

import (
	"context"
	"testing"

	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"
)

var _ Decorator = (*MockK8sDecorator)(nil)

type MockK8sDecorator struct{}

func (m *MockK8sDecorator) Decorate(metric stores.CIMetric) stores.CIMetric {
	return metric
}

func (m *MockK8sDecorator) Shutdown() error {
	return nil
}

const (
	util      = "UTIL"
	memUtil   = "USED_PERCENT"
	memUsed   = "FB_USED"
	memTotal  = "FB_TOTAL"
	temp      = "TEMP"
	powerDraw = "POWER_USAGE"
)

var metricToUnit = map[string]string{
	util:      "Percent",
	memUtil:   "Percent",
	memUsed:   "Bytes",
	memTotal:  "Bytes",
	temp:      "None",
	powerDraw: "None",
}

func TestConsumeMetrics(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	dc := &DecorateConsumer{
		ContainerOrchestrator: "EKS",
		NextConsumer:          consumertest.NewNop(),
		K8sDecorator:          &MockK8sDecorator{},
		MetricToUnitMap:       metricToUnit,
		Logger:                logger,
	}
	ctx := context.Background()

	testcases := map[string]TestCase{
		"empty": {
			Metrics:     pmetric.NewMetrics(),
			Want:        pmetric.NewMetrics(),
			ShouldError: false,
		},
		"unit": {
			Metrics: GenerateMetrics(map[MetricIdentifier][]map[string]string{
				{util, pmetric.MetricTypeGauge, 0}: {{
					"device": "test0",
				}},
				{memUtil, pmetric.MetricTypeGauge, 0}: {{
					"device": "test0",
				}},
				{memTotal, pmetric.MetricTypeGauge, 0}: {{
					"device": "test0",
				}},
				{memUsed, pmetric.MetricTypeGauge, 0}: {{
					"device": "test0",
				}},
				{powerDraw, pmetric.MetricTypeGauge, 0}: {{
					"device": "test0",
				}},
				{temp, pmetric.MetricTypeGauge, 0}: {{
					"device": "test0",
				}},
			}),
			Want: GenerateMetrics(map[MetricIdentifier][]map[string]string{
				{util, pmetric.MetricTypeGauge, 0}: {{
					"device": "test0",
					"Unit":   "Percent",
				}},
				{memUtil, pmetric.MetricTypeGauge, 0}: {{
					"device": "test0",
					"Unit":   "Percent",
				}},
				{memTotal, pmetric.MetricTypeGauge, 0}: {{
					"device": "test0",
					"Unit":   "Bytes",
				}},
				{memUsed, pmetric.MetricTypeGauge, 0}: {{
					"device": "test0",
					"Unit":   "Bytes",
				}},
				{powerDraw, pmetric.MetricTypeGauge, 0}: {{
					"device": "test0",
					"Unit":   "None",
				}},
				{temp, pmetric.MetricTypeGauge, 0}: {{
					"device": "test0",
					"Unit":   "None",
				}},
			}),
			ShouldError: false,
		},
		"noUnit": {
			Metrics: GenerateMetrics(map[MetricIdentifier][]map[string]string{
				{"test", pmetric.MetricTypeGauge, 0}: {{
					"device": "test0",
				}},
			}),
			Want: GenerateMetrics(map[MetricIdentifier][]map[string]string{
				{"test", pmetric.MetricTypeGauge, 0}: {{
					"device": "test0",
				}},
			}),
			ShouldError: false,
		},
		"typeUnchanged": {
			Metrics: GenerateMetrics(map[MetricIdentifier][]map[string]string{
				{util, pmetric.MetricTypeGauge, 0}: {{
					"device": "test0",
					"Type":   "TestType",
				}},
			}),
			Want: GenerateMetrics(map[MetricIdentifier][]map[string]string{
				{util, pmetric.MetricTypeGauge, 0}: {{
					"device": "test0",
					"Type":   "TestType",
					"Unit":   "Percent",
				}},
			}),
			ShouldError: false,
		},
	}

	RunDecoratorTestScenarios(ctx, t, dc, testcases)
}
