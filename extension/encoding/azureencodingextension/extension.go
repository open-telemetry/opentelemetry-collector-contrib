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

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
)

var (
	_ encoding.TracesUnmarshalerExtension  = (*azureExtension)(nil)
	_ encoding.LogsUnmarshalerExtension    = (*azureExtension)(nil)
	_ encoding.MetricsUnmarshalerExtension = (*azureExtension)(nil)
)

type azureExtension struct {
	config *Config
}

func (*azureExtension) UnmarshalTraces(_ []byte) (ptrace.Traces, error) {
	return ptrace.Traces{}, errors.New("not implemented yet")
}

func (*azureExtension) UnmarshalLogs(_ []byte) (plog.Logs, error) {
	return plog.Logs{}, errors.New("not implemented yet")
}

func (*azureExtension) UnmarshalMetrics(_ []byte) (pmetric.Metrics, error) {
	return pmetric.Metrics{}, errors.New("not implemented yet")
}

func (*azureExtension) Start(context.Context, component.Host) error {
	return nil
}

func (*azureExtension) Shutdown(context.Context) error {
	return nil
}
