// Copyright  OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ecssd

import (
	"time"

	"gopkg.in/yaml.v2"
)

const (
	DefaultRefreshInterval = 30 * time.Second
	DefaultJobLabelName    = "prometheus_job"
	AWSRegionEnvKey        = "AWS_REGION"
)

type Config struct {
	// ClusterName is the target ECS cluster name for service discovery.
	ClusterName string `mapstructure:"cluster_name" yaml:"cluster_name"`
	// ClusterRegion is the target ECS cluster's AWS region.
	ClusterRegion string `mapstructure:"cluster_region" yaml:"cluster_region"`
	// RefreshInterval determines how frequency at which the observer
	// needs to poll for collecting information about new processes.
	RefreshInterval time.Duration `mapstructure:"refresh_interval" yaml:"refresh_interval"`
	// ResultFile is the output path of the discovered targets YAML file (optional).
	// This is mainly used in conjunction with the Prometheus receiver.
	ResultFile string `mapstructure:"result_file" yaml:"result_file"`
	// JobLabelName is the override for prometheus job label, using `job` literal will cause error
	// in otel prometheus receiver. See https://github.com/open-telemetry/opentelemetry-collector/issues/575
	JobLabelName string `mapstructure:"job_label_name" yaml:"job_label_name"`
	// Services is a list of service name patterns for filtering tasks.
	Services []ServiceConfig `mapstructure:"services" yaml:"services"`
	// TaskDefinitions is a list of task definition arn patterns for filtering tasks.
	TaskDefinitions []TaskDefinitionConfig `mapstructure:"task_definitions" yaml:"task_definitions"`
	// DockerLabels is a list of docker labels for filtering containers within tasks.
	DockerLabels []DockerLabelConfig `mapstructure:"docker_labels" yaml:"docker_labels"`
}

// LoadConfig use yaml.v2 to decode the struct.
// It returns the yaml decode error directly.
func LoadConfig(b []byte) (Config, error) {
	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		return Config{}, err
	}
	return c, nil
}

// ExampleConfig returns an example instance that matches testdata/config_example.yaml.
// It can be used to validate if the struct tags like mapstructure, yaml are working properly.
func ExampleConfig() Config {
	return Config{
		ClusterName:     "ecs-sd-test-1",
		ClusterRegion:   "us-west-2",
		ResultFile:      "/etc/ecs_sd_targets.yaml",
		RefreshInterval: 15 * time.Second,
		JobLabelName:    DefaultJobLabelName,
		Services: []ServiceConfig{
			{
				NamePattern: "^retail-.*$",
			},
		},
		TaskDefinitions: []TaskDefinitionConfig{
			{
				CommonExporterConfig: CommonExporterConfig{
					JobName:      "task_def_1",
					MetricsPath:  "/not/metrics",
					MetricsPorts: []int{9113, 9090},
				},
				ArnPattern: ".*:task-definition/nginx:[0-9]+",
			},
		},
		DockerLabels: []DockerLabelConfig{
			{
				PortLabel: "ECS_PROMETHEUS_EXPORTER_PORT",
			},
		},
	}
}
