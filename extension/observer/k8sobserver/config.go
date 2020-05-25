// Copyright 2020, OpenTelemetry Authors
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

package k8sobserver

import (
	"go.opentelemetry.io/collector/config/configmodels"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/k8sconfig"
)

// Config defines configuration for k8s attributes processor.
type Config struct {
	configmodels.ExtensionSettings `mapstructure:",squash"`
	k8sconfig.APIConfig            `mapstructure:",squash"`

	// Node should be set to the node name to limit discovered endpoints to. For example, node name can
	// be set using the downward API inside the collector pod spec as follows:
	//
	// env:
	//   - name: K8S_NODE_NAME
	//     valueFrom:
	//       fieldRef:
	//         fieldPath: spec.nodeName
	//
	// Then set this value to ${K8S_NODE_NAME} in the configuration.
	Node string `mapstructure:"node"`
}
