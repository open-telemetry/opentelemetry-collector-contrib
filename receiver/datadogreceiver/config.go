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
	// Intake controls the `/intake` endpoint behavior
	Intake IntakeConfig `mapstructure:"intake"`
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
}

func (c *Config) Validate() error {
	behavior := c.Intake.Behavior
	isValidBehavior := behavior == "" || behavior == configIntakeBehaviorDisable || behavior == configIntakeBehaviorProxy
	if !isValidBehavior {
		return fmt.Errorf(`"intake.behavior" has an invalid value "%s"`, behavior)
	}
	return nil
}
