// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0 language governing permissions and
// limitations under the License.

package datadogreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver"

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/config/confighttp"

	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
)

const (
	defaultConfigIntakeProxyAPISite = "datadoghq.com"
	defaultConfigIntakeBehavior     = configIntakeBehaviorDisable

	configIntakeBehaviorDisable = "disable"
	configIntakeBehaviorProxy   = "proxy"
)

type Config struct {
	confighttp.ServerConfig `mapstructure:",squash"`
	// ReadTimeout of the http server
	ReadTimeout time.Duration `mapstructure:"read_timeout"`
	// TraceIDCacheSize sets the cache size for the 64 bits to 128 bits mapping
	TraceIDCacheSize int `mapstructure:"trace_id_cache_size"`
	// Intake controls the `/intake` endpoint behavior
	Intake IntakeConfig `mapstructure:"intake"`
	// IdleSeriesTimeout is the duration after which a series is considered stale
	// and removed from memory if no new data points are received.
	// The default value is 0, which disables this feature.
	IdleSeriesTimeout time.Duration `mapstructure:"idle_series_timeout"`
	// IdleSeriesCleanupInterval defines how frequently the receiver checks for
	// and removes idle series. Defaults to 5 minutes.
	IdleSeriesCleanupInterval time.Duration `mapstructure:"idle_series_cleanup_interval"`
	// Logs controls log-specific receiver behavior.
	Logs LogsConfig `mapstructure:"logs"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// LogsConfig controls log-specific receiver behavior.
type LogsConfig struct {
	// DecodeJSONMessage, when true, parses log records whose message is itself a JSON object and lifts
	// the inner fields into the OTel log record — mirroring Datadog's server-side "Preprocessing for
	// JSON logs". The inner reserved attributes (message, status/level, timestamp, host, service, and
	// dd.trace_id/dd.span_id for trace correlation) take precedence over the agent envelope, and the
	// remaining inner keys become attributes. This is useful because the Datadog Agent forwards
	// application JSON logs as an opaque message string; Datadog's backend normally does this parsing.
	// Enabled by default. set to false to keep the raw JSON message as the log body.
	DecodeJSONMessage bool `mapstructure:"decode_json_message"`

	// prevent unkeyed literal initialization
	_ struct{}
}

// IntakeConfig controls the `/intake` endpoint behavior
type IntakeConfig struct {
	// Behavior sets how the `/intake` endpoint should behave.
	// The value should be one of:
	// `disable` (default) - disable the endpoint entirely
	// `proxy` - proxy the requests to Datadog itself
	Behavior string `mapstructure:"behavior"`
	// Proxy controls how the `/intake` proxy operates
	Proxy ProxyConfig `mapstructure:"proxy"`
}

// ProxyConfig controls how the `/intake` proxy operates
type ProxyConfig struct {
	// API defines the settings for calling Datadog with the proxied requests
	API datadogconfig.APIConfig `mapstructure:"api"`

	// prevent unkeyed literal initialization
	_ struct{}
}

func (c *Config) Validate() error {
	behavior := c.Intake.Behavior
	isValidBehavior := behavior == "" || behavior == configIntakeBehaviorDisable || behavior == configIntakeBehaviorProxy
	if !isValidBehavior {
		return fmt.Errorf(`"intake.behavior" has an invalid value "%s"`, behavior)
	}
	return nil
}
