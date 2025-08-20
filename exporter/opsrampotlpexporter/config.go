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

package opsrampotlpexporter // import "go.opentelemetry.io/collector/exporter/otlpexporter"

import (
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const grantType = "client_credentials"

type MaskingSettings struct {
	Regexp      string `mapstructure:"regexp"`
	Placeholder string `mapstructure:"placeholder"`
}

type (
	SecuritySettings struct {
		OAuthServiceURL     string             `mapstructure:"oauth_service_url"`
		ClientID            string             `mapstructure:"client_id"`
		ClientSecret        string             `mapstructure:"client_secret"`
		OtelExporterSetting CustomtOtelSetting `mapstructure:"otel_exporter_setting"`
	}

	CustomtOtelSetting struct {
		ReadBufferSize  int `mapstructure:"otel_exporter_read_buffer_size"`
		WriteBufferSize int `mapstructure:"otel_exporter_write_buffer_size"`
		GrpcMaxSendSize int `mapstructure:"grpc_max_call_send_msg_size"`
		GrpcMaxRecvSize int `mapstructure:"grpc_max_call_recv_msg_size"`
	}
)

type Credentials struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   string `json:"expired_in"`
	Scope       string `json:"scope"`
}

func (s *SecuritySettings) Validate() error {
	if len(s.OAuthServiceURL) == 0 {
		return errors.New("oauth service url missed")
	}

	if len(s.ClientID) == 0 {
		return errors.New("client_id missed")
	}

	if len(s.ClientSecret) == 0 {
		return errors.New("client_secret missed")
	}

	return nil
}

// Config defines configuration for OpenCensus exporter.
type Config struct {
	exporterhelper.TimeoutConfig `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	exporterhelper.QueueConfig   `mapstructure:"sending_queue"`
	configretry.BackOffConfig    `mapstructure:"retry_on_failure"`

	Security                SecuritySettings         `mapstructure:"security"`
	configgrpc.ClientConfig `mapstructure:",squash"` // squash ensures fields are correctly decoded in embedded struct.
	Masking                 []MaskingSettings        `mapstructure:"masking"`
	ExpirationSkip          time.Duration            `mapstructure:"expiration_skip"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the exporter configuration is valid
func (cfg *Config) Validate() error {
	if err := cfg.QueueConfig.Validate(); err != nil {
		return fmt.Errorf("queue settings has invalid configuration: %w", err)
	}

	if err := cfg.Security.Validate(); err != nil {
		return fmt.Errorf("security settings has invalid configuration: %w", err)
	}

	return nil
}
