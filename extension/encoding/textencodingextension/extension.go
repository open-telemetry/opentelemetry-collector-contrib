// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package textencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/textencodingextension"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/textutils"
)

var (
	_ encoding.LogsMarshalerExtension   = (*textExtension)(nil)
	_ encoding.LogsUnmarshalerExtension = (*textExtension)(nil)
)

type textExtension struct {
	config      *Config
	textEncoder *textLogCodec
}

func (e *textExtension) UnmarshalLogs(buf []byte) (plog.Logs, error) {
	return e.textEncoder.UnmarshalLogs(buf)
}

func (e *textExtension) MarshalLogs(ld plog.Logs) ([]byte, error) {
	return e.textEncoder.MarshalLogs(ld)
}

func (e *textExtension) Start(_ context.Context, _ component.Host) error {
	encCfg := textutils.NewEncodingConfig()
	encCfg.Encoding = e.config.Encoding
	enc, err := encCfg.Build()
	if err != nil {
		return err
	}
	e.textEncoder = &textLogCodec{
		enc: &enc,
	}

	return err
}

func (e *textExtension) Shutdown(_ context.Context) error {
	return nil
}
