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

type Config struct {
	// Google Cloud Project ID where the Pubsub client will connect to
	ProjectID string `mapstructure:"project"`
	// User agent that will be used by the Pubsub client to connect to the service
	UserAgent string `mapstructure:"user_agent"`
	// Override of the Pubsub Endpoint, leave empty for the default endpoint
	Endpoint string `mapstructure:"endpoint"`
	// Only has effect if Endpoint is not ""
	Insecure bool `mapstructure:"insecure"`
	// Timeout for all API calls. If not set, defaults to 12 seconds.
	TimeoutSettings exporterhelper.TimeoutConfig `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.

	// The fully qualified resource name of the Pubsub subscription
	Subscription string `mapstructure:"subscription"`
	// Lock down the encoding of the payload, leave empty for attribute based detection
	Encoding string `mapstructure:"encoding"`
	// Lock down the compression of the payload, leave empty for attribute based detection
	Compression string `mapstructure:"compression"`

	// Ignore errors when the configured encoder fails to decoding a PubSub messages
	IgnoreEncodingError bool `mapstructure:"ignore_encoding_error"`

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
		StreamAckDeadline:       fcc.StreamAckDeadline,
		MaxOutstandingMessages:  fcc.MaxOutstandingMessages,
		MaxOutstandingBytes:     fcc.MaxOutstandingBytes,
	}
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
	return nil
}
