// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0 language governing permissions and
// limitations under the License.

package datadogreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver"

import (
	"time"

	"go.opentelemetry.io/collector/config/confighttp"
)

type Config struct {
	confighttp.HTTPServerSettings `mapstructure:",squash"`
	// ReadTimeout of the http server
	ReadTimeout time.Duration `mapstructure:"read_timeout"`
}
