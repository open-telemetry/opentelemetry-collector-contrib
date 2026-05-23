// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0 language governing permissions and
// limitations under the License.

package datadogreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver"

import (
	"fmt"
	"time"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/featuregate"

	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
)

var FullTraceIDFeatureGate = featuregate.GlobalRegistry().MustRegister(
	"receiver.datadogreceiver.Enable128BitTraceID",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("When enabled, adds support for 128bits TraceIDs for spans coming from Datadog instrumented services."),
	featuregate.WithRegisterFromVersion("v0.125.0"),
	featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/36926"),
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
