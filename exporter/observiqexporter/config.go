// Copyright  OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package observiqexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/observiqexporter"

import (
	"errors"
	"net/url"
	"os"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

// Config object for observIQ exporter
type Config struct {
	config.ExporterSettings        `mapstructure:",squash"`
	exporterhelper.TimeoutSettings `mapstructure:",squash"`
	exporterhelper.QueueSettings   `mapstructure:"sending_queue"`
	exporterhelper.RetrySettings   `mapstructure:"retry_on_failure"`
	// TLSSetting is the TLS settings for http client to use when sending logs to endpoint
	TLSSetting configtls.TLSClientSetting `mapstructure:"tls,omitempty"`
	// APIkey is the api key for authenticating with ingestion endpoint (required if no SecretKey)
	APIKey string `mapstructure:"api_key"`
	// SecretKey is the secret key for authenticating with the ingestion endpoint (required if no APIKey)
	SecretKey string `mapstructure:"secret_key"`
	// Endpoint is the url that defines the ingestion endpoint (default: "https://nozzle.app.observiq.com/v1/add")
	Endpoint string `mapstructure:"endpoint"`
	// AgentID is the ID that identifies this agent (default: uuid based off os.HostName())
	AgentID string `mapstructure:"agent_id"`
	// AgentName is the name identifies this agent (default: os.HostName())
	AgentName string `mapstructure:"agent_name"`
	// DialerTimeout is the amount of time to wait before aborting establishing the tcp connection when making
	// an http request (default: 10s)
	DialerTimeout time.Duration `mapstructure:"dialer_timeout"`
}

func (c *Config) Validate() error {
	if c.APIKey == "" && c.SecretKey == "" {
		return errors.New("api_key or secret_key must be specified")
	}

	if c.APIKey != "" && c.SecretKey != "" {
		return errors.New("only one of api_key OR secret_key can be specified, not both")
	}

	if c.Endpoint == "" {
		return errors.New("endpoint must not be empty")
	}

	url, urlParseError := url.Parse(c.Endpoint)

	if urlParseError != nil {
		return urlParseError
	}

	if url.Scheme != "http" && url.Scheme != "https" {
		return errors.New("url scheme must be http or https")
	}

	return nil
}

// Default agent name will be the hostname
func defaultAgentName() string {
	const fallbackAgentName = "otel collector"
	hn, err := os.Hostname()

	if err != nil {
		return fallbackAgentName
	}

	return hn
}

// Default agent ID will be UUID based off hostname
func defaultAgentID() string {
	const fallbackID = "00000000-0000-0000-0000-000000000000"

	hn, err := os.Hostname()
	if err != nil {
		return fallbackID
	}

	id := uuid.NewMD5(uuid.Nil, []byte(hn))

	return id.String()

}
