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

package k8sclusterreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver"

import (
	"fmt"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

// Config defines configuration for kubernetes cluster receiver.
type Config struct {
	k8sconfig.APIConfig `mapstructure:",squash"`

	// Collection interval for metrics.
	CollectionInterval time.Duration `mapstructure:"collection_interval"`

	// Node condition types to report. See all condition types, see
	// here: https://kubernetes.io/docs/concepts/architecture/nodes/#condition.
	NodeConditionTypesToReport []string `mapstructure:"node_conditions_to_report"`
	// Allocate resource types to report. See all resource types, see
	// here: https://kubernetes.io/docs/concepts/architecture/nodes/#capacity
	AllocatableTypesToReport []string `mapstructure:"allocatable_types_to_report"`
	// List of exporters to which metadata from this receiver should be forwarded to.
	MetadataExporters []string `mapstructure:"metadata_exporters"`

	// Whether OpenShift supprot should be enabled or not.
	Distribution string `mapstructure:"distribution"`
}

func (cfg *Config) Validate() error {
	switch cfg.Distribution {
	case distributionOpenShift:
	case distributionKubernetes:
	default:
		return fmt.Errorf("\"%s\" is not a supported distribution. Must be one of: \"openshift\", \"kubernetes\"", cfg.Distribution)
	}
	return nil
}
