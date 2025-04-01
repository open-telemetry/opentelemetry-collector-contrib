// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package syslogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/syslogexporter"

import (
	"errors"
	"strings"

	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

var (
	errUnsupportedPort     = errors.New("unsupported port: port is required, must be in the range 1-65535")
	errInvalidEndpoint     = errors.New("invalid endpoint: endpoint is required but it is not configured")
	errUnsupportedNetwork  = errors.New("unsupported network: network is required, only tcp/udp supported")
	errUnsupportedProtocol = errors.New("unsupported protocol: Only rfc5424 and rfc3164 supported")
	errOctetCounting       = errors.New("octet counting is only supported for rfc5424 protocol")
)

// Config defines configuration for Syslog exporter.
type Config struct {
	// Syslog server address
	Endpoint string `mapstructure:"endpoint"`
	// Syslog server port
	Port int `mapstructure:"port"`
	// Network for syslog communication
	// options: tcp, udp
	Network string `mapstructure:"network"`
	// Protocol of syslog messages
	// options: rfc5424, rfc3164
	Protocol string `mapstructure:"protocol"`

	// Whether or not to enable RFC 6587 Octet Counting.
	EnableOctetCounting bool `mapstructure:"enable_octet_counting"`

	// TLSSetting struct exposes TLS client configuration.
	TLSSetting configtls.ClientConfig `mapstructure:"tls"`

	QueueSettings             exporterhelper.QueueBatchConfig `mapstructure:"sending_queue"`
	configretry.BackOffConfig `mapstructure:"retry_on_failure"`
	TimeoutSettings           exporterhelper.TimeoutConfig `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct
}

// Validate the configuration for errors. This is required by component.Config.
func (cfg *Config) Validate() error {
	invalidFields := []error{}
	if cfg.Port < 1 || cfg.Port > 65535 {
		invalidFields = append(invalidFields, errUnsupportedPort)
	}

	if cfg.Endpoint == "" {
		invalidFields = append(invalidFields, errInvalidEndpoint)
	}

	cfg.Network = strings.ToLower(cfg.Network)
	if cfg.Network != string(confignet.TransportTypeTCP) && cfg.Network != string(confignet.TransportTypeUDP) {
		invalidFields = append(invalidFields, errUnsupportedNetwork)
	}

	switch cfg.Protocol {
	case protocolRFC3164Str:
	case protocolRFC5424Str:
	default:
		invalidFields = append(invalidFields, errUnsupportedProtocol)
	}

	if cfg.EnableOctetCounting && cfg.Protocol != protocolRFC5424Str {
		invalidFields = append(invalidFields, errOctetCounting)
	}

	if len(invalidFields) > 0 {
		return errors.Join(invalidFields...)
	}

	return nil
}

const (
	// Syslog Network
	DefaultNetwork = string(confignet.TransportTypeTCP)
	// Syslog Port
	DefaultPort = 514
	// Syslog Protocol
	DefaultProtocol = "rfc5424"
)
