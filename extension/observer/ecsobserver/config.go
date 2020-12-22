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

package ecsobserver

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/config/configmodels"
	"go.uber.org/zap"
)

const (
	portSeparator = ";"
)

// DockerLabelConfig defines the configuration fo docker label-based service discovery.
type DockerLabelConfig struct {
	JobNameLabel     string `mapstructure:"job_name_label"`
	PortLabel        string `mapstructure:"port_label"`
	MetricsPathLabel string `mapstructure:"metrics_path_label"`
}

// TaskDefinitionConfig defines the configuration for task definition-based service discovery.
type TaskDefinitionConfig struct {
	ContainerNamePattern string `mapstructure:"container_name_pattern"`
	JobName              string `mapstructure:"job_name"`
	MetricsPath          string `mapstructure:"metrics_path"`
	MetricsPorts         string `mapstructure:"metrics_ports"`
	TaskDefArnPattern    string `mapstructure:"task_definition_arn_pattern"`

	containerNameRegex *regexp.Regexp
	taskDefRegex       *regexp.Regexp
	metricsPortList    []int
}

// init initializes the task definition config by compiling regex patterns and extracting
// the list of metric ports.
func (t *TaskDefinitionConfig) init() {
	t.taskDefRegex = regexp.MustCompile(t.TaskDefArnPattern)

	if t.ContainerNamePattern != "" {
		t.containerNameRegex = regexp.MustCompile(t.ContainerNamePattern)
	}

	ports := strings.Split(t.MetricsPorts, portSeparator)
	for _, v := range ports {
		if port, err := strconv.Atoi(strings.TrimSpace(v)); err != nil || port < 0 {
			continue
		} else {
			t.metricsPortList = append(t.metricsPortList, port)
		}
	}
}

// Config defines the configuration for ECS observers.
type Config struct {
	configmodels.ExtensionSettings `mapstructure:",squash"`

	// RefreshInterval determines how frequency at which the observer
	// needs to poll for collecting information about new processes.
	RefreshInterval time.Duration `mapstructure:"refresh_interval"`
	// ClusterName is the target ECS cluster name for service discovery.
	ClusterName string `mapstructure:"cluster_name"`
	// ClusterRegion is the target ECS cluster's AWS region.
	ClusterRegion string `mapstructure:"cluster_region"`
	// ResultFile is the output path of the discovered targets YAML file (optional).
	// This is mainly used in conjunction with the Prometheus receiver.
	ResultFile string `mapstructure:"sd_result_file"`
	// DockerLabel provides the configuration for docker label-based service discovery.
	// If this is not provided, docker label-based service discovery is disabled.
	DockerLabel *DockerLabelConfig `mapstructure:"docker_label"`
	// TaskDefinitions is a list of task definition configurations for task
	// definition-based service discovery (optional). If this is not provided,
	// task definition-based SD is disabled.
	TaskDefinitions []*TaskDefinitionConfig `mapstructure:"task_definitions"`

	logger *zap.Logger
}
