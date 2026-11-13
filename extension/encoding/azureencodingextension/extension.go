// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
)

var (
	_ encoding.TracesUnmarshalerExtension  = (*azureExtension)(nil)
	_ encoding.LogsUnmarshalerExtension    = (*azureExtension)(nil)
	_ encoding.MetricsUnmarshalerExtension = (*azureExtension)(nil)
)

type azureExtension struct {
	config            *Config
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

func (*azureExtension) Start(context.Context, component.Host) error {
	return nil
}

func (*azureExtension) Shutdown(context.Context) error {
	return nil
}
