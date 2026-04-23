// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecsobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecsobserver"

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecsobserver/internal/errctx"
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

// newExportSetting checks if there are duplicated metrics ports.
func (c *CommonExporterConfig) newExportSetting() (*commonExportSetting, error) {
	m := make(map[int]bool)
	for _, p := range c.MetricsPorts {
		if m[p] {
			return nil, fmt.Errorf("metrics_ports has duplicated port %d", p)
		}
		m[p] = true
	}
	return &commonExportSetting{CommonExporterConfig: *c, metricsPorts: m}, nil
}

// commonExportSetting is generated from CommonExportConfig with some util methods.
type commonExportSetting struct {
	CommonExporterConfig
	metricsPorts map[int]bool
}

func (s *commonExportSetting) hasContainerPort(containerPort int) bool {
	return s.metricsPorts[containerPort]
}

// taskExporter converts annotated taskAnnotated into prometheusECSTarget.
type taskExporter struct {
	logger  *zap.Logger
	cluster string
}

func newTaskExporter(logger *zap.Logger, cluster string) *taskExporter {
	return &taskExporter{
		logger:  logger,
		cluster: cluster,
	}
}

// exportTasks loops a list of tasks and export prometheus scrape targets.
// It keeps track of error but does NOT stop when error occurs.
// The returned targets are valid, invalid targets are saved in a multi error.
// Caller can ignore the error because the only source is failing to get ip and port.
// The error(s) can generates debug log or metrics.
// To print the error with its task as context, use printExporterErrors.
func (e *taskExporter) exportTasks(tasks []*taskAnnotated) ([]prometheusECSTarget, error) {
	var merr error
	var allTargets []prometheusECSTarget
	for _, t := range tasks {
		targets, err := e.exportTask(t)
		multierr.AppendInto(&merr, err) // if err == nil, AppendInto does nothing
		// Even if there are error, returned targets are still valid.
		allTargets = append(allTargets, targets...)
	}
	return allTargets, merr
}

// exportTask exports all the matched container within a single task.
// One task can contain multiple containers. One container can have more than one target
// if there are multiple ports in `metrics_port`.
func (e *taskExporter) exportTask(task *taskAnnotated) ([]prometheusECSTarget, error) {
	// All targets in one task shares same IP.
	privateIP, err := task.PrivateIP()
	if err != nil {
		return nil, errctx.WithValue(err, errKeyTask, task)
	}

	// Base for all the containers in this task, most attributes are same.
	baseTarget := prometheusECSTarget{
		Source:                 aws.ToString(task.Task.TaskArn),
		MetricsPath:            defaultMetricsPath,
		ClusterName:            e.cluster,
		TaskDefinitionFamily:   aws.ToString(task.Definition.Family),
		TaskDefinitionRevision: int(task.Definition.Revision),
		TaskStartedBy:          aws.ToString(task.Task.StartedBy),
		TaskLaunchType:         string(task.Task.LaunchType),
		TaskGroup:              aws.ToString(task.Task.Group),
		TaskTags:               task.TaskTags(),
		HealthStatus:           string(task.Task.HealthStatus),
	}
	if task.Service != nil {
		baseTarget.ServiceName = aws.ToString(task.Service.ServiceName)
	}
	if task.EC2 != nil {
		ec2 := task.EC2
		baseTarget.EC2InstanceID = aws.ToString(ec2.InstanceId)
		baseTarget.EC2InstanceType = string(ec2.InstanceType)
		baseTarget.EC2Tags = task.EC2Tags()
		baseTarget.EC2VpcID = aws.ToString(ec2.VpcId)
		baseTarget.EC2SubnetID = aws.ToString(ec2.SubnetId)
		baseTarget.EC2PrivateIP = privateIP
		baseTarget.EC2PublicIP = aws.ToString(ec2.PublicIpAddress)
	}

	var targetsInTask []prometheusECSTarget
	var merr error
	for _, m := range task.Matched {
		container := task.Definition.ContainerDefinitions[m.ContainerIndex]
		// Shallow copy task level attributes
		containerTarget := baseTarget
		// Add container specific info
		containerTarget.ContainerName = aws.ToString(container.Name)
		containerTarget.ContainerLabels = task.ContainerLabels(m.ContainerIndex)
		// Multiple targets for a single container
		for _, matchedTarget := range m.Targets {
			// Shallow copy from container
			target := containerTarget
			mappedPort, err := task.MappedPort(container, int32(matchedTarget.Port))
			if err != nil {
				err = errctx.WithValues(err, map[string]any{
					errKeyTarget: matchedTarget,
					errKeyTask:   task,
				})
			}
			// Skip this target and keep track of port error, does not abort.
			if multierr.AppendInto(&merr, err) {
				continue
			}
			target.Address = fmt.Sprintf("%s:%d", privateIP, mappedPort)
			if matchedTarget.MetricsPath != "" {
				target.MetricsPath = matchedTarget.MetricsPath
			}
			target.Job = matchedTarget.Job
			targetsInTask = append(targetsInTask, target)
		}
	}
	return targetsInTask, merr
}
