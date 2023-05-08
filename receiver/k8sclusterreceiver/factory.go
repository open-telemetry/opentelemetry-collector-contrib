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
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

const (
	// Value of "type" key in configuration.
	typeStr = "k8s_cluster"
	// The stability level of the receiver.
	stability = component.StabilityLevelBeta

	// supported distributions
	distributionKubernetes = "kubernetes"
	distributionOpenShift  = "openshift"

	// Default config values.
	defaultCollectionInterval = 10 * time.Second
	defaultDistribution       = distributionKubernetes
)

var defaultNodeConditionsToReport = []string{"Ready"}

func createDefaultConfig() component.Config {
	return &Config{
		Distribution:               defaultDistribution,
		CollectionInterval:         defaultCollectionInterval,
		NodeConditionTypesToReport: defaultNodeConditionsToReport,
		APIConfig: k8sconfig.APIConfig{
			AuthType: k8sconfig.AuthTypeServiceAccount,
		},
	}
}

// NewFactory creates a factory for k8s_cluster receiver.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		typeStr,
		createDefaultConfig,
		receiver.WithMetrics(newReceiver, stability))
}
