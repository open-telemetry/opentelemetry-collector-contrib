// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fluentforwardreceiver

import (
	"errors"

	"go.opentelemetry.io/collector/config"
)

// Config defines configuration for the SignalFx receiver.
type Config struct {
	config.ReceiverSettings `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct

	// The address to listen on for incoming Fluent Forward events.  Should be
	// of the form `<ip addr>:<port>` (TCP) or `unix://<socket_path>` (Unix
	// domain socket).
	ListenAddress string `mapstructure:"endpoint"`

	Mappings MappingWhitelist `mapstructure:"mappings"`

	BodyAsString string `mapstructure:"body_encoding"`
}

func (c *Config) validate() error {
	if c.ListenAddress == "" {
		return errors.New("`endpoint` not specified")
	}

	if c.BodyAsString == "" {
		c.BodyAsString = "otel"
	}
	if !Contains([]string{"otel", "map"}, c.BodyAsString) {
		return errors.New("Invalid body_encoding")
	}

	if len(c.Mappings.Body) == 0 {
		c.Mappings.Body = []string{"log", "message"}
	}
	return nil
}

// MappingWhitelist defines the fluent attributes mapping configuration
type MappingWhitelist struct {
	// Body is the list of fluent attributes ok to become the OTEL body
	Body []string `mapstructure:"body"`

	// Severity is the list of fluent attributes ok to become the OTEL SeverityText
	Severity []string `mapstructure:"severity"`
}
