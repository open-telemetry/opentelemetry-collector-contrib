// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metricsgenerationprocessor

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/zap"
)

type metricsGenerationProcessor struct {
	generationRules []internalGenerationRule
	logger          *zap.Logger
}

var _ processorhelper.MProcessor = (*metricsGenerationProcessor)(nil)

type internalGenerationRule struct {
	NewMetricName  string
	Type           GenerationType
	Operand1Metric string
	Operand2Metric string
	Operation      OperationType
	ScaleBy        float64
}

func newMetricsGenerationProcessor(rules []internalGenerationRule, logger *zap.Logger) *metricsGenerationProcessor {
	return &metricsGenerationProcessor{
		generationRules: rules,
		logger:          logger,
	}
}

// Start is invoked during service startup.
func (mgp *metricsGenerationProcessor) Start(context.Context, component.Host) error {
	mgp.logger.Info("metricsGenerationProcessor started.")
	return nil
}

// ProcessMetrics implements the MProcessor interface.
func (mgp *metricsGenerationProcessor) ProcessMetrics(_ context.Context, md pdata.Metrics) (pdata.Metrics, error) {
	mgp.logger.Info("metricsGenerationProcessor processed metrics.")
	return md, nil
}

// Shutdown is invoked during service shutdown.
func (mgp *metricsGenerationProcessor) Shutdown(context.Context) error {
	mgp.logger.Info("metricsGenerationProcessor stopped.")
	return nil
}
