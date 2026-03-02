// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package textencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/textencodingextension"

import (
	"context"
	"io"
	"regexp"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/textutils"
)

var (
	_ encoding.LogsMarshalerExtension   = (*textExtension)(nil)
	_ encoding.LogsUnmarshalerExtension = (*textExtension)(nil)
	_ encoding.LogsDecoderExtension     = (*textExtension)(nil)
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

func (e *textExtension) NewLogsDecoder(reader io.Reader, options ...encoding.DecoderOption) (encoding.LogsDecoder, error) {
	return e.textEncoder.NewLogsDecoder(reader, options...)
}

func (e *textExtension) Start(_ context.Context, _ component.Host) error {
	enc, err := textutils.LookupEncoding(e.config.Encoding)
	if err != nil {
		return err
	}

	var unmarshallingSeparator *regexp.Regexp

	if e.config.UnmarshalingSeparator != "" {
		unmarshallingSeparator, err = regexp.Compile(e.config.UnmarshalingSeparator)
		if err != nil {
			return err
		}
	} else {
		unmarshallingSeparator = nil
	}

	e.textEncoder = &textLogCodec{
		decoder:               enc.NewDecoder(),
		marshalingSeparator:   e.config.MarshalingSeparator,
		unmarshalingSeparator: unmarshallingSeparator,
	}

	return err
}

func (*textExtension) Shutdown(context.Context) error {
	return nil
}
