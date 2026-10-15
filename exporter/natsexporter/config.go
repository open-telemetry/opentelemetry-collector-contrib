// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package natsexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natsexporter"

import (
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/natsclient"
)

const (
	// marshalerOTLPProto encodes payloads as OTLP protobuf.
	marshalerOTLPProto = "otlp_proto"
	// marshalerOTLPJSON encodes payloads as OTLP JSON.
	marshalerOTLPJSON = "otlp_json"
)

// SignalConfig defines the configuration for a single signal type (logs,
// metrics, or traces).
type SignalConfig struct {
	// Subject is the OTTL value expression used to construct the NATS subject
	// the signal is published to.
	//
	// See: https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/pkg/ottl/README.md
	// See: https://docs.nats.io/nats-concepts/subjects
	Subject string `mapstructure:"subject"`

	// Marshaler selects a built-in marshaler for outgoing payloads. Mutually
	// exclusive with EncodingExtension.
	//
	// Supported marshalers:
	//   - otlp_proto (default)
	//   - otlp_json
	Marshaler string `mapstructure:"marshaler"`

	// EncodingExtension is the component ID of an encoding extension used to
	// marshal outgoing payloads. Mutually exclusive with Marshaler.
	//
	// See: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/extension/encoding
	EncodingExtension string `mapstructure:"encoding_extension"`

	// Prevent unkeyed literal initialization.
	_ struct{}
}

// JetStreamConfig configures publishing via NATS JetStream (durable,
// acknowledged delivery) instead of core NATS. When present, every exported
// payload is published with JetStream and the publish blocks until the server
// acknowledges persistence.
//
// See: https://docs.nats.io/nats-concepts/jetstream
type JetStreamConfig struct {
	// Domain optionally selects a JetStream domain, e.g. when publishing through
	// a leaf node to a hub. Empty uses the server's default domain.
	Domain string `mapstructure:"domain"`

	// PublishTimeout bounds how long to wait for each publish acknowledgement.
	// Zero means no exporter-imposed deadline (the surrounding context still
	// applies).
	PublishTimeout time.Duration `mapstructure:"publish_timeout"`

	// Prevent unkeyed literal initialization.
	_ struct{}
}

// Config defines the configuration for the NATS exporter.
type Config struct {
	// ClientConfig holds the NATS connection settings (endpoint, pedantic, tls,
	// and auth).
	ClientConfig natsclient.ClientConfig `mapstructure:",squash"`

	// JetStream, when set, publishes via NATS JetStream (durable, acknowledged
	// delivery) instead of core NATS.
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
	if c.Marshaler != "" && c.EncodingExtension != "" {
		return errors.New("marshaler configured more than once")
	}
	if c.Marshaler != "" {
		switch c.Marshaler {
		case marshalerOTLPProto, marshalerOTLPJSON:
		default:
			return fmt.Errorf("unsupported marshaler: %q", c.Marshaler)
		}
	}
	if c.EncodingExtension != "" {
		var id component.ID
		if err := id.UnmarshalText([]byte(c.EncodingExtension)); err != nil {
			return fmt.Errorf("failed to parse encoding extension name: %w", err)
		}
	}
	return nil
}

func (c *JetStreamConfig) Validate() error {
	if c.PublishTimeout < 0 {
		return errors.New("jetstream publish_timeout must not be negative")
	}
	return nil
}

func (c *Config) Validate() error {
	var errs error
	errs = multierr.Append(errs, c.ClientConfig.TLS.Validate())
	errs = multierr.Append(errs, c.Logs.Validate())
	errs = multierr.Append(errs, c.Metrics.Validate())
	errs = multierr.Append(errs, c.Traces.Validate())
	errs = multierr.Append(errs, c.ClientConfig.Auth.Validate())
	if c.JetStream != nil {
		errs = multierr.Append(errs, c.JetStream.Validate())
	}
	return errs
}
