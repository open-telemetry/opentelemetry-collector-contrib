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

package k8sclusterreceiver

import (
	"time"

	quotaclientset "github.com/openshift/client-go/quota/clientset/versioned"
	"go.opentelemetry.io/collector/config"
	k8s "k8s.io/client-go/kubernetes"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig"
)

// Config defines configuration for kubernetes cluster receiver.
type Config struct {
	config.ReceiverSettings `mapstructure:",squash"`
	k8sconfig.APIConfig     `mapstructure:",squash"`

	// Collection interval for metrics.
	CollectionInterval time.Duration `mapstructure:"collection_interval"`

	// Node condition types to report. See all condition types, see
	// here: https://kubernetes.io/docs/concepts/architecture/nodes/#condition.
	NodeConditionTypesToReport []string `mapstructure:"node_conditions_to_report"`
	// List of exporters to which metadata from this receiver should be forwarded to.
	MetadataExporters []string `mapstructure:"metadata_exporters"`

	// Whether OpenShift supprot should be enabled or not.
	Distribution string `mapstructure:"distribution"`

	// For mocking.
	makeClient               func(apiConf k8sconfig.APIConfig) (k8s.Interface, error)
	makeOpenShiftQuotaClient func(apiConf k8sconfig.APIConfig) (quotaclientset.Interface, error)
}

func (cfg *Config) Validate() error {
	return cfg.APIConfig.Validate()
}

func (cfg *Config) getK8sClient() (k8s.Interface, error) {
	if cfg.makeClient == nil {
		cfg.makeClient = k8sconfig.MakeClient
	}
	return cfg.makeClient(cfg.APIConfig)
}

func (cfg *Config) getOpenShiftQuotaClient() (quotaclientset.Interface, error) {
	if cfg.makeOpenShiftQuotaClient == nil {
		cfg.makeOpenShiftQuotaClient = k8sconfig.MakeOpenShiftQuotaClient
	}
	return cfg.makeOpenShiftQuotaClient(cfg.APIConfig)
}
