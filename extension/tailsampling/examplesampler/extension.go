// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package examplesampler // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/tailsampling/examplesampler"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/tailsampling/examplesampler/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"
)

type exampleExtension struct {
	component.Component

	telemetry *metadata.TelemetryBuilder
	logger    *zap.Logger
}

var (
	_ extension.Extension      = (*exampleExtension)(nil)
	_ samplingpolicy.Extension = (*exampleExtension)(nil)
)

func NewExtension(settings extension.Settings, _ *Config) (extension.Extension, error) {
	telemetry, err := metadata.NewTelemetryBuilder(settings.TelemetrySettings)
	if err != nil {
		return nil, err
	}

	return &exampleExtension{
		telemetry: telemetry,
		logger:    settings.Logger,
	}, nil
}

func (e *exampleExtension) Start(_ context.Context, _ component.Host) error {
	e.logger.Debug("Starting examplesampler extension")
	return nil
}

func (e *exampleExtension) Shutdown(_ context.Context) error {
	e.logger.Debug("Shutting down examplesampler extension")
	e.telemetry.Shutdown()
	return nil
}

// NewEvaluator implements samplingpolicy.Extension.
func (e *exampleExtension) NewEvaluator(policyName string, cfg map[string]any) (samplingpolicy.Evaluator, error) {
	e.logger.Debug("Creating new evaluator", zap.String("policy_name", policyName), zap.Any("cfg", cfg))

	return &evaluator{
		policy:    policyName,
		telemetry: e.telemetry,
	}, nil
}

type evaluator struct {
	policy    string
	telemetry *metadata.TelemetryBuilder
}

// Evaluate implements samplingpolicy.Evaluator.
func (e *evaluator) Evaluate(ctx context.Context, _ pcommon.TraceID, _ *samplingpolicy.TraceData) (samplingpolicy.Decision, error) {
	e.telemetry.ExtensionTailSamplingEvaluationsCount.Add(ctx, 1,
		metric.WithAttributes(attribute.String("policy", e.policy)),
	)
	return samplingpolicy.NotSampled, nil
}
