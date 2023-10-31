// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package batch // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/batch"

import (
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type unsupported struct{}

var (
	_ ptrace.Marshaler  = (*unsupported)(nil)
	_ pmetric.Marshaler = (*unsupported)(nil)
	_ plog.Marshaler    = (*unsupported)(nil)
)

func (unsupported) MarshalTraces(_ ptrace.Traces) ([]byte, error) {
	return nil, ErrUnsupportedEncoding
}

func (unsupported) MarshalMetrics(_ pmetric.Metrics) ([]byte, error) {
	return nil, ErrUnsupportedEncoding
}

func (unsupported) MarshalLogs(_ plog.Logs) ([]byte, error) {
	return nil, ErrUnsupportedEncoding
}
