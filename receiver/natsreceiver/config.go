// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package natsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/natsreceiver"

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/natsclient"
)

const (
	// encodingOTLPProto decodes payloads as OTLP protobuf.
	encodingOTLPProto = "otlp_proto"
	// encodingOTLPJSON decodes payloads as OTLP JSON.
	encodingOTLPJSON = "otlp_json"
)

// SignalConfig defines the configuration for a single signal type (logs,
// metrics, or traces).
type SignalConfig struct {
	// Subject is the NATS subject to subscribe to (core NATS) or the filter
	// subject of the JetStream consumer. Wildcards are allowed.
	//
	// See: https://docs.nats.io/nats-concepts/subjects
	Subject string `mapstructure:"subject"`

	// QueueGroup, when set, joins the core NATS subscription to a queue group so
	// that messages are load balanced across receivers sharing the group. Core
	// NATS only; in JetStream mode, share a Durable consumer instead.
	//
	// See: https://docs.nats.io/nats-concepts/core-nats/queue
	QueueGroup string `mapstructure:"queue_group"`

	// Encoding selects a built-in unmarshaler for incoming payloads. Mutually
	// exclusive with EncodingExtension.
	//
	// Supported encodings:
	//   - otlp_proto (default)
	//   - otlp_json
	Encoding string `mapstructure:"encoding"`

	// EncodingExtension is the component ID of an encoding extension used to
	// unmarshal incoming payloads. Mutually exclusive with Encoding.
	//
	// See: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/extension/encoding
	EncodingExtension string `mapstructure:"encoding_extension"`

	// Stream is the JetStream stream to consume from. JetStream only; empty
	// looks up the stream capturing Subject.
	Stream string `mapstructure:"stream"`

	// Durable is the durable consumer name. JetStream only; empty creates an
	// ephemeral consumer that does not survive restarts.
	Durable string `mapstructure:"durable"`

	// Prevent unkeyed literal initialization.
	_ struct{}
}

// JetStreamConfig configures consumption via a NATS JetStream consumer
// (acknowledged, redelivered on failure) instead of a core NATS subscription.
//
// See: https://docs.nats.io/nats-concepts/jetstream/consumers
type JetStreamConfig struct {
	// Domain optionally selects a JetStream domain, e.g. when consuming through
	// a leaf node from a hub. Empty uses the server's default domain.
	Domain string `mapstructure:"domain"`

	// AckWait bounds how long the server waits for an ack before redelivering
	// a message. Zero uses the server default.
	AckWait time.Duration `mapstructure:"ack_wait"`

	// MaxDeliver caps delivery attempts per message. Zero uses the server
	// default.
	MaxDeliver int `mapstructure:"max_deliver"`

	// Prevent unkeyed literal initialization.
	_ struct{}
}

// Config defines the configuration for the NATS receiver.
type Config struct {
	// ClientConfig holds the NATS connection settings (endpoint, pedantic, tls,
	// and auth).
	ClientConfig natsclient.ClientConfig `mapstructure:",squash"`

	// JetStream, when set, consumes via a JetStream consumer (acknowledged,
	// redelivered on failure) instead of a core NATS subscription.
	JetStream *JetStreamConfig `mapstructure:"jetstream"`

	// Logs holds the configuration for the logs signal.
	Logs SignalConfig `mapstructure:"logs"`
	// Metrics holds the configuration for the metrics signal.
	Metrics SignalConfig `mapstructure:"metrics"`
	// Traces holds the configuration for the traces signal.
	Traces SignalConfig `mapstructure:"traces"`

	// Prevent unkeyed literal initialization.
	_ struct{}
}

var _ component.Config = (*Config)(nil)

func (c *SignalConfig) Validate() error {
	var errs error
	if c.Subject == "" {
		errs = multierr.Append(errs, errors.New("subject must not be empty"))
	}
	if strings.ContainsAny(c.QueueGroup, " \t\r\n") {
		errs = multierr.Append(errs, fmt.Errorf("queue_group must not contain whitespace: %q", c.QueueGroup))
	}
	if c.Encoding != "" && c.EncodingExtension != "" {
		errs = multierr.Append(errs, errors.New("encoding configured more than once"))
	}
	if c.Encoding != "" {
		switch c.Encoding {
		case encodingOTLPProto, encodingOTLPJSON:
		default:
			errs = multierr.Append(errs, fmt.Errorf("unsupported encoding: %q", c.Encoding))
		}
	}
	if c.EncodingExtension != "" {
		var id component.ID
		if err := id.UnmarshalText([]byte(c.EncodingExtension)); err != nil {
			errs = multierr.Append(errs, fmt.Errorf("failed to parse encoding extension name: %w", err))
		}
	}
	return errs
}

// validateMode rejects options that do not apply to the configured delivery
// mode, so they are not silently ignored.
func (c *SignalConfig) validateMode(jetstream bool) error {
	if jetstream && c.QueueGroup != "" {
		return errors.New("queue_group is not supported with jetstream; share a durable consumer instead")
	}
	if !jetstream && (c.Stream != "" || c.Durable != "") {
		return errors.New("stream and durable require jetstream to be configured")
	}
	return nil
}

func (c *JetStreamConfig) Validate() error {
	var errs error
	if c.AckWait < 0 {
		errs = multierr.Append(errs, errors.New("jetstream ack_wait must not be negative"))
	}
	if c.MaxDeliver < 0 {
		errs = multierr.Append(errs, errors.New("jetstream max_deliver must not be negative"))
	}
	return errs
}

// Validate checks cross-field constraints only; nested structs are validated
// by confmap.Validate.
func (c *Config) Validate() error {
	var errs error
	signals := []struct {
		name string
		cfg  *SignalConfig
	}{{"logs", &c.Logs}, {"metrics", &c.Metrics}, {"traces", &c.Traces}}
	for _, s := range signals {
		if err := s.cfg.validateMode(c.JetStream != nil); err != nil {
			errs = multierr.Append(errs, fmt.Errorf("%s: %w", s.name, err))
		}
	}
	return errs
}
