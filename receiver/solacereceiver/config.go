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

package solacereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver"

import (
	"errors"
	"strings"
	"time"

	"go.opentelemetry.io/collector/config/configtls"
)

const (
	// 8Kb
	saslMaxInitFrameSizeOverride = 8000
)

var (
	errMissingAuthDetails       = errors.New("authentication details are required, either for plain user name password or XOAUTH2 or client certificate")
	errMissingQueueName         = errors.New("queue definition is required, queue definition has format queue://<queuename>")
	errMissingPlainTextParams   = errors.New("missing plain text auth params: Username, Password")
	errMissingXauth2Params      = errors.New("missing xauth2 text auth params: Username, Bearer")
	errMissingFlowControl       = errors.New("missing flow control configuration: DelayedRetry must be selected")
	errInvalidDelayedRetryDelay = errors.New("delayed_retry.delay must > 0")
)

// Config defines configuration for Solace receiver.
type Config struct {
	// The list of solace brokers (default localhost:5671)
	Broker []string `mapstructure:"broker"`

	// The name of the solace queue to consume from, it is required parameter
	Queue string `mapstructure:"queue"`

	// The maximum number of unacknowledged messages the Solace broker can transmit, to configure AMQP Link
	MaxUnacked int32 `mapstructure:"max_unacknowledged"`

	TLS configtls.TLSClientSetting `mapstructure:"tls,omitempty"`

	Auth Authentication `mapstructure:"auth"`

	Flow FlowControl `mapstructure:"flow_control"`
}

// Validate checks the receiver configuration is valid
func (cfg *Config) Validate() error {
	if cfg.Auth.PlainText == nil && cfg.Auth.External == nil && cfg.Auth.XAuth2 == nil {
		return errMissingAuthDetails
	}
	if len(strings.TrimSpace(cfg.Queue)) == 0 {
		return errMissingQueueName
	}
	if cfg.Flow.DelayedRetry == nil {
		return errMissingFlowControl
	} else if cfg.Flow.DelayedRetry.Delay <= 0 {
		return errInvalidDelayedRetryDelay
	}
	return nil
}

// Authentication defines authentication strategies.
type Authentication struct {
	PlainText *SaslPlainTextConfig `mapstructure:"sasl_plain"`
	XAuth2    *SaslXAuth2Config    `mapstructure:"sasl_xauth2"`
	External  *SaslExternalConfig  `mapstructure:"sasl_external"`
}

// SaslPlainTextConfig defines SASL PLAIN authentication.
type SaslPlainTextConfig struct {
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

// SaslXAuth2Config defines the configuration for the SASL XAUTH2 authentication.
type SaslXAuth2Config struct {
	Username string `mapstructure:"username"`
	Bearer   string `mapstructure:"bearer"`
}

// SaslExternalConfig defines the configuration for the SASL External used in conjunction with TLS client authentication.
type SaslExternalConfig struct {
}

// FlowControl defines the configuration for what to do in backpressure scenarios, e.g. memorylimiter errors
type FlowControl struct {
	DelayedRetry *FlowControlDelayedRetry `mapstructure:"delayed_retry"`
}

// FlowControlDelayedRetry represents the strategy of waiting for a defined amount of time (in time.Duration) and attempt redelivery
type FlowControlDelayedRetry struct {
	Delay time.Duration `mapstructure:"delay"`
}
