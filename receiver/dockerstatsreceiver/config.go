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

package dockerstatsreceiver

import (
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/config"
)

var _ config.Receiver = (*Config)(nil)

type Config struct {
	config.ReceiverSettings `mapstructure:",squash"`
	// The URL of the docker server.  Default is "unix:///var/run/docker.sock"
	Endpoint string `mapstructure:"endpoint"`
	// The time between each collection event.  Default is 10s.
	CollectionInterval time.Duration `mapstructure:"collection_interval"`

	// The maximum amount of time to wait for docker API responses.  Default is 5s
	Timeout time.Duration `mapstructure:"timeout"`

	// A mapping of container label names to MetricDescriptor label keys.
	// The corresponding container label value will become the DataPoint label value
	// for the mapped name.  E.g. `io.kubernetes.container.name: container_spec_name`
	// would result in a MetricDescriptor label called `container_spec_name` whose
	// Metric DataPoints have the value of the `io.kubernetes.container.name` container label.
	ContainerLabelsToMetricLabels map[string]string `mapstructure:"container_labels_to_metric_labels"`

	// A mapping of container environment variable names to MetricDescriptor label
	// keys.  The corresponding env var values become the DataPoint label value.
	// E.g. `APP_VERSION: version` would result MetricDescriptors having a label
	// key called `version` whose DataPoint label values are the value of the
	// `APP_VERSION` environment variable configured for that particular container, if
	// present.
	EnvVarsToMetricLabels map[string]string `mapstructure:"env_vars_to_metric_labels"`

	// A list of filters whose matching images are to be excluded.  Supports literals, globs, and regex.
	ExcludedImages []string `mapstructure:"excluded_images"`

	// Whether to report all CPU metrics.  Default is false
	ProvidePerCoreCPUMetrics bool `mapstructure:"provide_per_core_cpu_metrics"`

	// Docker client API version. Default is 1.22
	DockerAPIVersion float64 `mapstructure:"api_version"`
}

func (config Config) Validate() error {
	if config.Endpoint == "" {
		return errors.New("config.Endpoint must be specified")
	}
	if config.CollectionInterval == 0 {
		return errors.New("config.CollectionInterval must be specified")
	}
	if config.DockerAPIVersion == 0 {
		config.DockerAPIVersion = defaultDockerAPIVersion
	}
	if config.DockerAPIVersion < minimalRequiredDockerAPIVersion {
		return fmt.Errorf("Docker API version must be at least %v", minimalRequiredDockerAPIVersion)
	}
	return nil
}
