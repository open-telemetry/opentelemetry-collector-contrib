// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package encoding // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// LogsMessageDecoderExtension decodes a complete message into logs.
// Implementations must support concurrent calls and keep per-message state local to each call.
type LogsMessageDecoderExtension interface {
	extension.Extension
	DecodeLogs(ctx context.Context, payload []byte, opts ...LogsDecodeOption) (plog.Logs, error)
}

// MetricsMessageDecoderExtension decodes a complete message into metrics.
// Implementations must support concurrent calls and keep per-message state local to each call.
type MetricsMessageDecoderExtension interface {
	extension.Extension
	DecodeMetrics(ctx context.Context, payload []byte, opts ...MetricsDecodeOption) (pmetric.Metrics, error)
}

// TracesMessageDecoderExtension decodes a complete message into traces.
// Implementations must support concurrent calls and keep per-message state local to each call.
type TracesMessageDecoderExtension interface {
	extension.Extension
	DecodeTraces(ctx context.Context, payload []byte, opts ...TracesDecodeOption) (ptrace.Traces, error)
}

// ProfilesMessageDecoderExtension decodes a complete message into profiles.
// Implementations must support concurrent calls and keep per-message state local to each call.
type ProfilesMessageDecoderExtension interface {
	extension.Extension
	DecodeProfiles(ctx context.Context, payload []byte, opts ...ProfilesDecodeOption) (pprofile.Profiles, error)
}

// MessageDecodeOption configures message decoding.
// Only options defined by this package can implement this interface.
// Decoders use NewDecodeConfig to apply options.
type MessageDecodeOption interface {
	apply(*DecodeConfig) error
}

// LogsDecodeOption configures log message decoding.
type LogsDecodeOption = MessageDecodeOption

// MetricsDecodeOption configures metric message decoding.
type MetricsDecodeOption = MessageDecodeOption

// TracesDecodeOption configures trace message decoding.
type TracesDecodeOption = MessageDecodeOption

// ProfilesDecodeOption configures profile message decoding.
type ProfilesDecodeOption = MessageDecodeOption

// DecodeConfig holds the configuration for one message decoding call.
// Construct a separate configuration for each call with NewDecodeConfig.
type DecodeConfig struct {
	// MessageMetadata contains explicit metadata for this message.
	// It defaults to empty metadata and is not populated from context.
	MessageMetadata client.Metadata
}

// NewDecodeConfig applies options to a new message decoding configuration.
// It returns an error for a nil option or an option validation error.
func NewDecodeConfig(opts ...MessageDecodeOption) (DecodeConfig, error) {
	config := DecodeConfig{}
	for i, option := range opts {
		if option == nil {
			return DecodeConfig{}, fmt.Errorf("message decode option %d is nil", i)
		}
		if err := option.apply(&config); err != nil {
			return DecodeConfig{}, fmt.Errorf("message decode option %d: %w", i, err)
		}
	}
	return config, nil
}

// WithMessageMetadata supplies metadata for the message being decoded.
// Metadata is immutable and may be shared by concurrent calls.
// If supplied more than once, the last metadata option replaces earlier metadata.
func WithMessageMetadata(metadata client.Metadata) MessageDecodeOption {
	return messageMetadataOption{metadata: metadata}
}

type messageMetadataOption struct {
	metadata client.Metadata
}

func (o messageMetadataOption) apply(config *DecodeConfig) error {
	config.MessageMetadata = o.metadata
	return nil
}
