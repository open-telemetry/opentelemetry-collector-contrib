// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package syslogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/syslogexporter"

import (
	"errors"
	"strings"

	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/multierr"
)

var (
	errUnsupportedPort     = errors.New("unsupported port: port is required, must be in the range 1-65535")
	errInvalidEndpoint     = errors.New("invalid endpoint: endpoint is required but it is not configured")
	errUnsupportedNetwork  = errors.New("unsupported network: network is required, only tcp/udp supported")
	errUnsupportedProtocol = errors.New("unsupported protocol: Only rfc5424 and rfc3164 supported")
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

	// TLSSetting struct exposes TLS client configuration.
	TLSSetting configtls.TLSClientSetting `mapstructure:"tls"`

	exporterhelper.QueueSettings   `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings   `mapstructure:"retry_on_failure"`
	exporterhelper.TimeoutSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct
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

	if strings.ToLower(cfg.Network) != "tcp" && strings.ToLower(cfg.Network) != "udp" {
		invalidFields = append(invalidFields, errUnsupportedNetwork)
	}

	switch cfg.Protocol {
	case protocolRFC3164Str:
	case protocolRFC5424Str:
	default:
		invalidFields = append(invalidFields, errUnsupportedProtocol)
	}

	if len(invalidFields) > 0 {
		return multierr.Combine(invalidFields...)
	}

	return nil
}

const (
	// Syslog Network
	DefaultNetwork = "tcp"
	// Syslog Port
	DefaultPort = 514
	// Syslog Protocol
	DefaultProtocol = "rfc5424"
)
