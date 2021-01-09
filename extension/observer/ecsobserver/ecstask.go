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
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
)

const (
	containerNameLabel   = "container_name"
	taskFamilyLabel      = "TaskDefinitionFamily"
	taskRevisionLabel    = "TaskRevision"
	taskGroupLabel       = "TaskGroup"
	taskStartedbyLabel   = "StartedBy"
	taskLaunchTypeLabel  = "LaunchType"
	taskJobNameLabel     = "job"
	taskMetricsPathLabel = "__metrics_path__"
	ec2InstanceTypeLabel = "InstanceType"
	ec2VpcIDLabel        = "VpcId"
	ec2SubnetIDLabel     = "SubnetId"
)

type EC2MetaData struct {
	ContainerInstanceID string
	ECInstanceID        string
	PrivateIP           string
	InstanceType        string
	VpcID               string
	SubnetID            string
}

type ECSTask struct {
	Task           *ecs.Task
	TaskDefinition *ecs.TaskDefinition
	EC2Info        *EC2MetaData

	DockerLabelBased    bool
	TaskDefinitionBased bool
}

func (t *ECSTask) addTargets(targets map[string]*Target, config *Config) {
	ip := t.getPrivateIP()
	if ip == "" {
		return
	}
	for _, c := range t.TaskDefinition.ContainerDefinitions {
		t.addDockerLabelBasedTarget(ip, c, targets, config)
		t.addTaskDefinitionBasedTargets(ip, c, targets, config)
	}
}

// getPrivateIP retrieves the private ip of the ECS task.
func (t *ECSTask) getPrivateIP() (ip string) {
	if t.TaskDefinition.NetworkMode == nil {
		return
	}

	// AWSVPC: Get Private IP from tasks->attachments (ElasticNetworkInterface -> privateIPv4Address)
	if *t.TaskDefinition.NetworkMode == ecs.NetworkModeAwsvpc {
		for _, v := range t.Task.Attachments {
			if aws.StringValue(v.Type) != "ElasticNetworkInterface" {
				continue
			}
			for _, d := range v.Details {
				if aws.StringValue(d.Name) == "privateIPv4Address" {
					return aws.StringValue(d.Value)
				}
			}
		}
	}

	if t.EC2Info != nil {
		return t.EC2Info.PrivateIP
	}

	return
}

// addDockerLabelBasedTarget adds a target based on docker labels.
func (t *ECSTask) addDockerLabelBasedTarget(ip string, c *ecs.ContainerDefinition, targets map[string]*Target, config *Config) {
	if !t.DockerLabelBased {
		return
	}

	configuredPortStr, ok := c.DockerLabels[config.DockerLabel.PortLabel]
	if !ok {
		// skip the container without matching sd_port_label
		return
	}

	port, err := strconv.Atoi(aws.StringValue(configuredPortStr))
	if err != nil || port < 0 {
		// an invalid port definition.
		return
	}

	hostPort := t.getHostPort(int64(port), c)
	if hostPort == 0 {
		return
	}

	metricsPath := ""
	if v, ok := c.DockerLabels[config.DockerLabel.MetricsPathLabel]; ok {
		metricsPath = aws.StringValue(v)
	}

	endpoint := fmt.Sprintf("%s:%d%s", ip, hostPort, metricsPath)
	if _, ok := targets[endpoint]; ok {
		return
	}

	customizedJobName := ""
	if jobName, ok := c.DockerLabels[config.DockerLabel.JobNameLabel]; ok {
		customizedJobName = aws.StringValue(jobName)
	}

	targets[endpoint] = &Target{
		IP:          ip,
		Port:        hostPort,
		MetricsPath: metricsPath,
		Labels:      t.generateTargetLabels(c, metricsPath, customizedJobName),
	}
}

// addTaskDefinitionBasedTargets adds targets based on task definition.
func (t *ECSTask) addTaskDefinitionBasedTargets(ip string, c *ecs.ContainerDefinition, targets map[string]*Target, config *Config) {
	if !t.TaskDefinitionBased {
		return
	}

	for _, taskDef := range config.TaskDefinitions {
		taskDefArn := aws.StringValue(t.Task.TaskDefinitionArn)
		// skip if task def regex mismatch
		if !taskDef.taskDefRegex.MatchString(taskDefArn) {
			continue
		}

		containerName := aws.StringValue(c.Name)
		// skip if there is container name regex pattern configured and container name mismatch
		if taskDef.ContainerNamePattern != "" && !taskDef.containerNameRegex.MatchString(containerName) {
			continue
		}

		for _, port := range taskDef.metricsPortList {
			// TODO: see if possible to optimize this instead of iterating through containers each time
			hostPort := t.getHostPort(int64(port), c)
			if hostPort == 0 {
				continue
			}

			endpoint := fmt.Sprintf("%s:%d%s", ip, hostPort, taskDef.MetricsPath)
			if _, ok := targets[endpoint]; !ok {
				targets[endpoint] = &Target{
					IP:          ip,
					Port:        hostPort,
					MetricsPath: taskDef.MetricsPath,
					Labels:      t.generateTargetLabels(c, taskDef.MetricsPath, taskDef.JobName),
				}
			}
		}

	}
}

// getHostPort gets the host port of the container with the given container port.
func (t *ECSTask) getHostPort(containerPort int64, c *ecs.ContainerDefinition) int64 {
	networkMode := aws.StringValue(t.TaskDefinition.NetworkMode)
	if networkMode == "" || networkMode == ecs.NetworkModeNone {
		// for network type: none, skipped directly
		return 0
	}

	if networkMode == ecs.NetworkModeAwsvpc || networkMode == ecs.NetworkModeHost {
		// for network type: awsvpc or host, get the mapped port from: taskDefinition->containerDefinitions->portMappings
		for _, v := range c.PortMappings {
			if aws.Int64Value(v.ContainerPort) == containerPort {
				return aws.Int64Value(v.HostPort)
			}
		}
	} else if networkMode == ecs.NetworkModeBridge {
		// for network type: bridge, get the mapped port from: task->containers->networkBindings
		containerName := aws.StringValue(c.Name)
		for _, tc := range t.Task.Containers {
			if containerName != aws.StringValue(tc.Name) {
				continue
			}
			for _, v := range tc.NetworkBindings {
				if aws.Int64Value(v.ContainerPort) == containerPort {
					return aws.Int64Value(v.HostPort)
				}
			}
		}
	}

	return 0
}

// generateTargetLabels creates labels for discovered target.
func (t *ECSTask) generateTargetLabels(c *ecs.ContainerDefinition, metricsPath string, jobName string) map[string]string {
	labels := make(map[string]string)
	revisionStr := fmt.Sprintf("%d", *t.TaskDefinition.Revision)

	addTargetLabel(labels, containerNameLabel, c.Name)
	addTargetLabel(labels, taskFamilyLabel, t.TaskDefinition.Family)
	addTargetLabel(labels, taskGroupLabel, t.Task.Group)
	addTargetLabel(labels, taskLaunchTypeLabel, t.Task.LaunchType)
	addTargetLabel(labels, taskMetricsPathLabel, &metricsPath)
	addTargetLabel(labels, taskRevisionLabel, &revisionStr)
	addTargetLabel(labels, taskStartedbyLabel, t.Task.StartedBy)

	if t.EC2Info != nil {
		addTargetLabel(labels, ec2InstanceTypeLabel, &t.EC2Info.InstanceType)
		addTargetLabel(labels, ec2SubnetIDLabel, &t.EC2Info.SubnetID)
		addTargetLabel(labels, ec2VpcIDLabel, &t.EC2Info.VpcID)
	}

	for k, v := range c.DockerLabels {
		addTargetLabel(labels, k, v)
	}

	// handle customized job label last, so the previous job docker label is overridden
	addTargetLabel(labels, taskJobNameLabel, &jobName)

	return labels
}

// addTargetLabel adds a label to the labels map if the given label value is valid.
func addTargetLabel(labels map[string]string, labelKey string, labelValuePtr *string) {
	labelValue := aws.StringValue(labelValuePtr)
	if labelValue != "" {
		labels[labelKey] = labelValue
	}
}
