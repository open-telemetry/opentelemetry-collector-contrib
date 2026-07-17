// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension/internal/metadata"
)

var (
	_ encoding.TracesUnmarshalerExtension  = (*azureExtension)(nil)
	_ encoding.LogsUnmarshalerExtension    = (*azureExtension)(nil)
	_ encoding.MetricsUnmarshalerExtension = (*azureExtension)(nil)
)

type azureExtension struct {
	config            *Config
	logger            *zap.Logger
	logUnmarshaler    plog.Unmarshaler
	traceUnmarshaler  ptrace.Unmarshaler
	metricUnmarshaler pmetric.Unmarshaler
}

func (ex *azureExtension) UnmarshalTraces(buf []byte) (ptrace.Traces, error) {
	return ex.traceUnmarshaler.UnmarshalTraces(buf)
}

func (ex *azureExtension) UnmarshalLogs(buf []byte) (plog.Logs, error) {
	return ex.logUnmarshaler.UnmarshalLogs(buf)
}

func (ex *azureExtension) UnmarshalMetrics(buf []byte) (pmetric.Metrics, error) {
	return ex.metricUnmarshaler.UnmarshalMetrics(buf)
}

func (ex *azureExtension) Start(_ context.Context, _ component.Host) error {
	if metadata.ExtensionAzureencodingDontEmitV0LogConventionsFeatureGate.IsEnabled() &&
		!metadata.ExtensionAzureencodingEmitV1LogConventionsFeatureGate.IsEnabled() {
		err := errors.New("extension.azureencoding.DontEmitV0LogConventions cannot be enabled without enabling extension.azureencoding.EmitV1LogConventions")
		ex.logger.Error("Invalid feature gate combination", zap.Error(err))
		return err
	}

	if !metadata.ExtensionAzureencodingDontEmitV0LogConventionsFeatureGate.IsEnabled() {
		ex.logger.Warn(
			"[WARNING] Azure encoding logs currently emit legacy semconv attributes. " +
				"To opt in to v1 semconv attributes, enable extension.azureencoding.EmitV1LogConventions and " +
				"extension.azureencoding.DontEmitV0LogConventions feature gates.",
		)
	}

	return nil
}

func (*azureExtension) Shutdown(context.Context) error {
	return nil
}
