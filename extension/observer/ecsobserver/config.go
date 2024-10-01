// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecsobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecsobserver"

import (
	"fmt"
	"os"
	"time"
)

const (
	defaultRefreshInterval             = 30 * time.Second
	defaultJobLabelName                = "prometheus_job"
	awsRegionEnvKey                    = "AWS_REGION"
	defaultDockerLabelMatcherPortLabel = "ECS_PROMETHEUS_EXPORTER_PORT"
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

// Validate overrides the embedded noop validation so that load config can trigger
// our own validation logic.
func (c *Config) Validate() error {
	if c.ClusterName == "" {
		// TODO: https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/3188
		// would allow auto detect cluster name in extension
		return fmt.Errorf("must specify ECS cluster name directly")
	}
	for _, s := range c.Services {
		if err := s.validate(); err != nil {
			return err
		}
	}
	for _, t := range c.TaskDefinitions {
		if err := t.validate(); err != nil {
			return err
		}
	}
	for _, d := range c.DockerLabels {
		if err := d.validate(); err != nil {
			return err
		}
	}
	return nil
}

// defaultConfig only applies docker label
func defaultConfig() Config {
	return Config{
		ClusterName:     "default",
		ClusterRegion:   os.Getenv(awsRegionEnvKey),
		ResultFile:      "/etc/ecs_sd_targets.yaml",
		RefreshInterval: defaultRefreshInterval,
		JobLabelName:    defaultJobLabelName,
		DockerLabels: []DockerLabelConfig{
			{
				PortLabel: defaultDockerLabelMatcherPortLabel,
			},
		},
	}
}

// exampleConfig returns an example instance that matches testdata/config_example.yaml.
// It can be used to validate if the struct tags like mapstructure, yaml are working properly.
func exampleConfig() *Config {
	return &Config{
		ClusterName:     "ecs-sd-test-1",
		ClusterRegion:   "us-west-2",
		ResultFile:      "/etc/ecs_sd_targets.yaml",
		RefreshInterval: 15 * time.Second,
		JobLabelName:    defaultJobLabelName,
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
