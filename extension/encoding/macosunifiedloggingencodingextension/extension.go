// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package macosunifiedloggingencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/macosunifiedloggingencodingextension"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
)

var (
	_ encoding.LogsUnmarshalerExtension = (*macosUnifiedLoggingExtension)(nil)
)

type macosUnifiedLoggingExtension struct {
	config  *Config
	decoder *macosUnifiedLoggingDecoder
}

func (e *macosUnifiedLoggingExtension) UnmarshalLogs(buf []byte) (plog.Logs, error) {
	return e.decoder.UnmarshalLogs(buf)
}

func (e *macosUnifiedLoggingExtension) Start(_ context.Context, _ component.Host) error {
	e.decoder = &macosUnifiedLoggingDecoder{
		config: e.config,
	}
	return nil
}

func (*macosUnifiedLoggingExtension) Shutdown(context.Context) error {
	return nil
}
