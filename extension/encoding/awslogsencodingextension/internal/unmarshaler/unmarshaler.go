// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package unmarshaler // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension/internal/unmarshaler"

import (
	"io"

	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
)

// AWSUnmarshaler is the base interface for all AWS log format unmarshalers.
type AWSUnmarshaler interface {
	UnmarshalAWSLogs(reader io.Reader) (plog.Logs, error)
}

// StreamingLogsUnmarshaler is optionally implemented by unmarshalers that support streaming decoding.
type StreamingLogsUnmarshaler interface {
	AWSUnmarshaler
	NewLogsDecoder(reader io.Reader, options ...encoding.DecoderOption) (encoding.LogsDecoder, error)
}
