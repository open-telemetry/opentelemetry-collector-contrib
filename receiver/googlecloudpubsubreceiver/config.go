// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudpubsubreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver"

import (
	"fmt"
	"regexp"
	"time"

	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver/internal"
)

var subscriptionMatcher = regexp.MustCompile(`projects/[a-z][a-z0-9\-]*(:[a-z0-9\-]+)?/subscriptions/`)

// Values for the on_decode_error and on_pipeline_error policies.
const (
	onErrorPropagate = "propagate"
	onErrorIgnore    = "ignore"
	onErrorAck       = "ack"
	onErrorNack      = "nack"
)

type Config struct {
	// Google Cloud Project ID where the Pubsub client will connect to
	ProjectID string `mapstructure:"project"`
	// User agent that will be used by the Pubsub client to connect to the service
	UserAgent string `mapstructure:"user_agent"`
	// Override of the Pubsub Endpoint, leave empty for the default endpoint
	Endpoint string `mapstructure:"endpoint"`
	// Only has effect if Endpoint is not ""
	Insecure bool `mapstructure:"insecure"`
	// UniverseDomain is the universe domain for the Pubsub service.
	// Defaults to "googleapis.com". Set to support Sovereign Cloud regions.
	// See https://pkg.go.dev/google.golang.org/api/option#WithUniverseDomain
	UniverseDomain string `mapstructure:"universe_domain"`
	// Timeout for all API calls. If not set, defaults to 12 seconds.
	TimeoutSettings exporterhelper.TimeoutConfig `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.

	// The fully qualified resource name of the Pubsub subscription
	Subscription string `mapstructure:"subscription"`
	// Lock down the encoding of the payload, leave empty for attribute based detection
	Encoding string `mapstructure:"encoding"`
	// Lock down the compression of the payload, leave empty for attribute based detection
	Compression string `mapstructure:"compression"`

	// Ignore errors when the configured encoder fails to decoding a PubSub messages.
	// This asks for the behavior of on_decode_error set to "ignore", which is also the
	// default, so setting it changes nothing on its own; setting it together with a
	// conflicting on_decode_error fails validation.
	//
	// Deprecated: use on_decode_error set to "ignore" instead.
	IgnoreEncodingError bool `mapstructure:"ignore_encoding_error"`

	// on_decode_error sets how a message that the configured encoding fails to decode is
	// handled. "ignore" (the default) acknowledges and drops the message, counting it in
	// the encoding error metric. "propagate" leaves the message unacknowledged, so it is
	// redelivered after the ack deadline expires; this was the previous default and it
	// makes an undecodable message redeliver forever. "nack" negatively acknowledges the
	// message (a modify ack deadline of 0), so the subscription retry policy or dead
	// letter policy applies.
	OnDecodeError string `mapstructure:"on_decode_error"`

	// on_pipeline_error sets how a message that the downstream pipeline rejects with a
	// permanent error is handled. "ack" (the default) acknowledges and drops the message.
	// "nack" negatively acknowledges the message, so the subscription retry policy or
	// dead letter policy applies. Transient rejections (a full sending queue, memory
	// limiter refusal) are never acknowledged nor negatively acknowledged, so the message
	// is redelivered after the ack deadline expires regardless of this setting.
	OnPipelineError string `mapstructure:"on_pipeline_error"`

	// The client id that will be used by Pubsub to make load balancing decisions
	ClientID string `mapstructure:"client_id"`

	FlowControlConfig FlowControlConfig `mapstructure:"flow_control"`
}

// FlowControlConfig defines the flow control configuration for the receiver. This is used to
// tune the internal flow control implementation, along with the Pub/Sub flow control settings
// documented at https://cloud.google.com/pubsub/docs/flow-control and
// https://cloud.google.com/pubsub/docs/reference/rpc/google.pubsub.v1#streamingpullrequest
type FlowControlConfig struct {
	// The maximum duration the acknowledgement loop waits before sending the acknowledgements.
	TriggerAckBatchDuration time.Duration `mapstructure:"trigger_ack_batch_duration"`

	// The number of pending acknowledgements (acks and nacks combined) that triggers an
	// immediate flush, without waiting for trigger_ack_batch_duration. 0 (the default)
	// disables the size trigger.
	TriggerAckBatchSize int `mapstructure:"trigger_ack_batch_size"`

	// The ack deadline to use for the Pub/Sub stream.
	StreamAckDeadline time.Duration `mapstructure:"stream_ack_deadline"`
	// Pub/Sub flow control settings for the maximum number of outstanding messages.
	MaxOutstandingMessages int64 `mapstructure:"max_outstanding_messages"`
	// Pub/Sub flow control settings for the maximum number of outstanding bytes.
	MaxOutstandingBytes int64 `mapstructure:"max_outstanding_bytes"`
}

func (fcc *FlowControlConfig) getInternalConfig() *internal.FlowControlConfig {
	return &internal.FlowControlConfig{
		TriggerAckBatchDuration: fcc.TriggerAckBatchDuration,
		TriggerAckBatchSize:     fcc.TriggerAckBatchSize,
		StreamAckDeadline:       fcc.StreamAckDeadline,
		MaxOutstandingMessages:  fcc.MaxOutstandingMessages,
		MaxOutstandingBytes:     fcc.MaxOutstandingBytes,
	}
}

// decodeErrorPolicy resolves the effective on_decode_error policy. The
// deprecated ignore_encoding_error flag asks for "ignore", which is also the
// default, so an unset on_decode_error resolves to "ignore" either way; setting
// the flag together with a conflicting on_decode_error fails validation.
func (config *Config) decodeErrorPolicy() string {
	if config.OnDecodeError != "" {
		return config.OnDecodeError
	}
	return onErrorIgnore
}

func (config *Config) validate() error {
	if !subscriptionMatcher.MatchString(config.Subscription) {
		return fmt.Errorf("subscription '%s' is not a valid format, use 'projects/<project_id>/subscriptions/<name>'", config.Subscription)
	}
	switch config.Compression {
	case "":
	case "gzip":
	default:
		return fmt.Errorf("compression %v is not supported.  supported compression formats include [gzip]", config.Compression)
	}
	switch config.OnDecodeError {
	case "", onErrorPropagate, onErrorIgnore, onErrorNack:
	default:
		return fmt.Errorf("on_decode_error %q is not supported. supported values: [propagate, ignore, nack]", config.OnDecodeError)
	}
	if config.IgnoreEncodingError && config.OnDecodeError != "" && config.OnDecodeError != onErrorIgnore {
		return fmt.Errorf("ignore_encoding_error conflicts with on_decode_error %q. remove the deprecated ignore_encoding_error, or set on_decode_error to \"ignore\"", config.OnDecodeError)
	}
	switch config.OnPipelineError {
	case "", onErrorAck, onErrorNack:
	default:
		return fmt.Errorf("on_pipeline_error %q is not supported. supported values: [ack, nack]", config.OnPipelineError)
	}
	if config.FlowControlConfig.TriggerAckBatchSize < 0 {
		return fmt.Errorf("trigger_ack_batch_size %d is not supported. use a positive value, or 0 to disable the size trigger", config.FlowControlConfig.TriggerAckBatchSize)
	}
	return nil
}
