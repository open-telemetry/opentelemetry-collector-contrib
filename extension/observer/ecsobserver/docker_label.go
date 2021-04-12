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

package ecsobserver

import (
	"fmt"
)

// DockerLabelConfig matches all tasks based on their docker label.
//
// NOTE: it's possible to make DockerLabelConfig part of CommonExporterConfig
// and use it both ServiceConfig and TaskDefinitionConfig.
// However, based on existing users, few people mix different types of filters.
// If that usecase arises in the future, we can rewrite the top level docker lable filter
// using a task definition filter with arn_pattern:*.
type DockerLabelConfig struct {
	CommonExporterConfig `mapstructure:",squash" yaml:",inline"`

	// PortLabel is mandetory, empty string means docker label based match is skipped.
	PortLabel        string `mapstructure:"port_label" yaml:"port_label"`
	JobNameLabel     string `mapstructure:"job_name_label" yaml:"job_name_label"`
	MetricsPathLabel string `mapstructure:"metrics_path_label" yaml:"metrics_path_label"`
}

func (d *DockerLabelConfig) Init() error {
	// It's possible to support it in the future, but for now just fail at config,
	// so user don't need to wonder which port is used in the exported target.
	if len(d.MetricsPorts) != 0 {
		return fmt.Errorf("metrics_ports is not supported in docker_labels, got %v", d.MetricsPorts)
	}
	if d.PortLabel == "" {
		return fmt.Errorf("port_label is empty")
	}
	return nil
}
