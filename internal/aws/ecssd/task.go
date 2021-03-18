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
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ecs"
)

// Task contains both raw task info and its definition
type Task struct {
	Task       *ecs.Task
	Definition *ecs.TaskDefinition
	EC2        *ec2.Instance
	Service    *ecs.Service
	Matched    []MatchedContainer
}

func (t *Task) AddMatchedContainer(newContainer MatchedContainer) {
	for i, oldContainer := range t.Matched {
		if oldContainer.ContainerIndex == newContainer.ContainerIndex {
			t.Matched[i].MergeTargets(newContainer.Targets)
			return
		}
	}
	t.Matched = append(t.Matched, newContainer)
}

func (t *Task) TaskTags() map[string]string {
	if len(t.Task.Tags) == 0 {
		return nil
	}
	tags := make(map[string]string, len(t.Task.Tags))
	for _, tag := range t.Task.Tags {
		tags[aws.StringValue(tag.Key)] = aws.StringValue(tag.Value)
	}
	return tags
}

// NOTE: the tag to string conversion is duplicated because the Tag struct is defined in each service's own API package.
// i.e. services don't import a common package that includes tag definition.
func (t *Task) EC2Tags() map[string]string {
	if t.EC2 == nil || len(t.EC2.Tags) == 0 {
		return nil
	}
	tags := make(map[string]string, len(t.EC2.Tags))
	for _, tag := range t.EC2.Tags {
		tags[aws.StringValue(tag.Key)] = aws.StringValue(tag.Value)
	}
	return tags
}

func (t *Task) ContainerLabels(containerIndex int) map[string]string {
	def := t.Definition.ContainerDefinitions[containerIndex]
	labels := make(map[string]string, len(def.DockerLabels))
	for k, v := range def.DockerLabels {
		labels[k] = aws.StringValue(v)
	}
	return labels
}

func (t *Task) PrivateIP() (string, error) {
	arn := aws.StringValue(t.Task.TaskArn)
	switch aws.StringValue(t.Definition.NetworkMode) {
	// Default network mode is bridge on EC2
	case "", ecs.NetworkModeHost, ecs.NetworkModeBridge:
		if t.EC2 == nil {
			return "", fmt.Errorf("task has no network mode and no ec2 info %s", arn)
		}
		return aws.StringValue(t.EC2.PrivateIpAddress), nil
	case ecs.NetworkModeAwsvpc:
		for _, v := range t.Task.Attachments {
			if aws.StringValue(v.Type) == "ElasticNetworkInterface" {
				for _, d := range v.Details {
					if aws.StringValue(d.Name) == "privateIPv4Address" {
						return aws.StringValue(d.Value), nil
					}
				}
			}
		}
		return "", fmt.Errorf("private ipv4 address not found for awsvpc on task %s", arn)
	case ecs.NetworkModeNone:
		return "", fmt.Errorf("task has none network mode %s", arn)
	default:
		return "", fmt.Errorf("unknown task network mode %q for task %s", aws.StringValue(t.Definition.NetworkMode), arn)
	}
}

func (t *Task) MappedPort(def *ecs.ContainerDefinition, containerPort int64) (int64, error) {
	arn := aws.StringValue(t.Task.TaskArn)
	mode := aws.StringValue(t.Definition.NetworkMode)
	switch mode {
	case ecs.NetworkModeNone:
		return 0, fmt.Errorf("task has none network mode %s", arn)
	case ecs.NetworkModeHost, ecs.NetworkModeAwsvpc:
		// taskDefinition->containerDefinitions->portMappings
		for _, v := range def.PortMappings {
			if containerPort == aws.Int64Value(v.ContainerPort) {
				return aws.Int64Value(v.HostPort), nil
			}
		}
		return 0, fmt.Errorf("port %d not found for network mode %s", containerPort, mode)
	case ecs.NetworkModeBridge:
		//  task->containers->networkBindings
		for _, c := range t.Task.Containers {
			if aws.StringValue(def.Name) == aws.StringValue(c.Name) {
				for _, b := range c.NetworkBindings {
					if containerPort == aws.Int64Value(b.ContainerPort) {
						return aws.Int64Value(b.HostPort), nil
					}
				}
			}
		}
		return 0, fmt.Errorf("port %d not found for network mode %s", containerPort, mode)
	default:
		return 0, fmt.Errorf("port %d not found for container %s on task %s",
			containerPort, aws.StringValue(def.Name), arn)
	}
}
