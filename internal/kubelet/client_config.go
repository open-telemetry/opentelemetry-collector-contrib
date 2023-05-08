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

package kubelet // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/kubelet"

import (
	"go.opentelemetry.io/collector/config/configtls"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

// ClientConfig for a kubelet client for talking to a kubelet HTTP endpoint.
type ClientConfig struct {
	k8sconfig.APIConfig  `mapstructure:",squash"`
	configtls.TLSSetting `mapstructure:",squash"`
	// InsecureSkipVerify controls whether the client verifies the server's
	// certificate chain and host name.
	InsecureSkipVerify bool `mapstructure:"insecure_skip_verify"`
}
