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

	"github.com/aws/aws-sdk-go/aws"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

const (
	defaultMetricsPath = "/metrics"
)

// CommonExporterConfig should be embedded into filter config.
// They set labels like job, metrics_path etc. that can override prometheus default.
type CommonExporterConfig struct {
	JobName      string `mapstructure:"job_name" yaml:"job_name"`
	MetricsPath  string `mapstructure:"metrics_path" yaml:"metrics_path"`
	MetricsPorts []int  `mapstructure:"metrics_ports" yaml:"metrics_ports"`
}

type TaskExporter struct {
	logger   *zap.Logger
	matchers map[MatcherType][]Matcher
	cluster  string
}

type TaskExporterOptions struct {
	Logger   *zap.Logger
	Matchers map[MatcherType][]Matcher
	Cluster  string
}

func NewTaskExporter(opts TaskExporterOptions) *TaskExporter {
	return &TaskExporter{
		logger:   opts.Logger,
		matchers: opts.Matchers,
		cluster:  opts.Cluster,
	}
}

func (e *TaskExporter) ExportTasks(tasks []*Task) ([]PrometheusECSTarget, error) {
	var merr error
	var allTargets []PrometheusECSTarget
	for _, t := range tasks {
		targets, err := e.ExportTask(t)
		if multierr.AppendInto(&merr, err) {
			continue
		}
		allTargets = append(allTargets, targets...)
	}
	return allTargets, merr
}

// ExportTask exports all the matched container within a single task.
// Even one container can contains multiple targets if there is multiple configured metrics_port.
func (e *TaskExporter) ExportTask(task *Task) ([]PrometheusECSTarget, error) {
	privateIP, err := task.PrivateIP()
	if err != nil {
		return nil, fmt.Errorf("get private ip of task failed during export: %w", err)
	}

	taskTarget := PrometheusECSTarget{
		Source:                 aws.StringValue(task.Task.TaskArn),
		MetricsPath:            defaultMetricsPath,
		ClusterName:            e.cluster,
		TaskDefinitionFamily:   aws.StringValue(task.Definition.Family),
		TaskDefinitionRevision: int(aws.Int64Value(task.Definition.Revision)),
		TaskStartedBy:          aws.StringValue(task.Task.StartedBy),
		TaskLaunchType:         aws.StringValue(task.Task.LaunchType),
		TaskGroup:              aws.StringValue(task.Task.Group),
		TaskTags:               task.TaskTags(),
		HealthStatus:           aws.StringValue(task.Task.HealthStatus),
	}
	if task.Service != nil {
		taskTarget.ServiceName = aws.StringValue(task.Service.ServiceName)
	}
	if task.EC2 != nil {
		ec2 := task.EC2
		taskTarget.EC2InstanceID = aws.StringValue(ec2.InstanceId)
		taskTarget.EC2InstanceType = aws.StringValue(ec2.InstanceType)
		taskTarget.EC2Tags = task.EC2Tags()
		taskTarget.EC2VpcID = aws.StringValue(ec2.VpcId)
		taskTarget.EC2SubnetID = aws.StringValue(ec2.SubnetId)
		taskTarget.EC2PrivateIP = privateIP
		taskTarget.EC2PublicIP = aws.StringValue(ec2.PublicIpAddress)
	}

	var targetsInTask []PrometheusECSTarget
	var merr error
	for _, m := range task.Matched {
		container := task.Definition.ContainerDefinitions[m.ContainerIndex]
		// Shallow copy from task
		containerTarget := taskTarget
		// Add container info
		containerTarget.ContainerName = aws.StringValue(container.Name)
		containerTarget.ContainerLabels = task.ContainerLabels(m.ContainerIndex)
		// Multiple targets for a single container
		for _, matchedTarget := range m.Targets {
			// Shallow copy from container
			target := containerTarget
			mappedPort, err := task.MappedPort(container, int64(matchedTarget.Port))
			// NOTE: Ignore port error
			if multierr.AppendInto(&merr, err) {
				continue
			}
			target.Address = formatAddress(privateIP, mappedPort)
			if matchedTarget.MetricsPath != "" {
				target.MetricsPath = matchedTarget.MetricsPath
			}
			target.Job = matchedTarget.Job
			targetsInTask = append(targetsInTask, target)
		}
	}
	return targetsInTask, merr
}

func formatAddress(ip string, port int64) string {
	return fmt.Sprintf("%s:%d", ip, port)
}
