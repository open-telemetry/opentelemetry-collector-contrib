// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0 language governing permissions and
// limitations under the License.

package datadogreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver"

import (
	"time"

	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/featuregate"
)

var FullTraceIDFeatureGate = featuregate.GlobalRegistry().MustRegister(
	"receiver.datadogreceiver.Enable128BitTraceID",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("When enabled, adds support for 128bits TraceIDs for spans coming from Datadog instrumented services."),
	featuregate.WithRegisterFromVersion("v0.125.0"),
	featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/36926"),
)

type Config struct {
	confighttp.ServerConfig `mapstructure:",squash"`
	// ReadTimeout of the http server
	ReadTimeout time.Duration `mapstructure:"read_timeout"`
	// TraceIDCacheSize sets the cache size for the 64 bits to 128 bits mapping
	TraceIDCacheSize int `mapstructure:"trace_id_cache_size"`
}
