// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecsobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecsobserver"

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"go.uber.org/zap"
)

// taskAnnotated contains both raw task info and its definition.
// It is generated from taskFetcher.
type taskAnnotated struct {
	Task       ecstypes.Task
	Definition *ecstypes.TaskDefinition
	EC2        *ec2types.Instance
	Service    *ecstypes.Service
	Matched    []matchedContainer
}

// AddMatchedContainer tries to add a new matched container.
// If the container already exists will merge targets within one container (i.e. different port/metrics path).
func (t *taskAnnotated) AddMatchedContainer(newContainer matchedContainer) {
	for i, oldContainer := range t.Matched {
		if oldContainer.ContainerIndex == newContainer.ContainerIndex {
			t.Matched[i].MergeTargets(newContainer.Targets)
			return
		}
	}
	t.Matched = append(t.Matched, newContainer)
}

func (t *taskAnnotated) TaskTags() map[string]string {
	if len(t.Task.Tags) == 0 {
		return nil
	}
	tags := make(map[string]string, len(t.Task.Tags))
	for _, tag := range t.Task.Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}
	return tags
}

// EC2Tags returns ec2 instance tags as it is. Sanitize to prometheus label format is done during export.
// NOTE: the tag to string conversion is duplicated because the Tag struct is defined in each service's own API package.
// i.e. services don't import a common package that includes tag definition.
func (t *taskAnnotated) EC2Tags() map[string]string {
	if t.EC2 == nil || len(t.EC2.Tags) == 0 {
		return nil
	}
	tags := make(map[string]string, len(t.EC2.Tags))
	for _, tag := range t.EC2.Tags {
		tags[aws.ToString(tag.Key)] = aws.ToString(tag.Value)
	}
	return tags
}

func (t *taskAnnotated) ContainerLabels(containerIndex int) map[string]string {
	def := t.Definition.ContainerDefinitions[containerIndex]
	if len(def.DockerLabels) == 0 {
		return nil
	}
	labels := make(map[string]string, len(def.DockerLabels))
	for k, v := range def.DockerLabels {
		labels[k] = v
	}
	return labels
}

// errPrivateIPNotFound indicates the awsvpc private ip or EC2 instance ip is not found.
type errPrivateIPNotFound struct {
	TaskArn     string
	NetworkMode ecstypes.NetworkMode
	Extra       string // extra message
}

func (e *errPrivateIPNotFound) Error() string {
	m := fmt.Sprintf("private ip not found for network mode %q and task %s", e.NetworkMode, e.TaskArn)
	if e.Extra != "" {
		m = m + " " + e.Extra
	}
	return m
}

func (e *errPrivateIPNotFound) message() string {
	return "private ip not found"
}

func (e *errPrivateIPNotFound) zapFields() []zap.Field {
	return []zap.Field{
		zap.String("NetworkMode", string(e.NetworkMode)),
		zap.String("Extra", e.Extra),
	}
}

// PrivateIP returns private ip address based on network mode.
// EC2 launch type can use host/bridge mode and the private ip is the EC2 instance's ip.
// awsvpc has its own ip regardless of launch type.
func (t *taskAnnotated) PrivateIP() (string, error) {
	mode := t.Definition.NetworkMode
	errNotFound := &errPrivateIPNotFound{
		TaskArn:     aws.ToString(t.Task.TaskArn),
		NetworkMode: mode,
	}
	switch mode {
	// When network mode is empty and launch type is EC2, ECS uses bridge network.
	// For fargate it has to be awsvpc and will error on task creation if invalid config is given.
	// See https://docs.aws.amazon.com/AmazonECS/latest/userguide/fargate-task-defs.html#fargate-tasks-networkmod
	// In another word, when network mode is empty, it must be EC2 bridge.
	case "", ecstypes.NetworkModeHost, ecstypes.NetworkModeBridge:
		if t.EC2 == nil {
			errNotFound.Extra = "EC2 info not found"
			return "", errNotFound
		}
		return aws.ToString(t.EC2.PrivateIpAddress), nil
	case ecstypes.NetworkModeAwsvpc:
		for _, v := range t.Task.Attachments {
			if aws.ToString(v.Type) == "ElasticNetworkInterface" {
				for _, d := range v.Details {
					if aws.ToString(d.Name) == "privateIPv4Address" {
						return aws.ToString(d.Value), nil
					}
				}
			}
		}
		return "", errNotFound
	case ecstypes.NetworkModeNone:
		return "", errNotFound
	default:
		return "", errNotFound
	}
}

// errMappedPortNotFound indicates the port specified in config does not exists
// or the location for mapped ports has changed on ECS side.
type errMappedPortNotFound struct {
	TaskArn       string
	NetworkMode   ecstypes.NetworkMode
	ContainerName string
	ContainerPort int32
}

func (e *errMappedPortNotFound) Error() string {
	// Output the error message in this order to make searching easier as only task arn changes frequently.
	// %q for network mode because empty string is valid for ECS EC2.
	return fmt.Sprintf("mapped port not found for container port %d network mode %q on container %s in task %s",
		e.ContainerPort, e.NetworkMode, e.ContainerName, e.TaskArn)
}

func (e *errMappedPortNotFound) message() string {
	return "mapped port not found"
}

func (e *errMappedPortNotFound) zapFields() []zap.Field {
	return []zap.Field{
		zap.String("NetworkMode", string(e.NetworkMode)),
		zap.Int32("ContainerPort", e.ContainerPort),
	}
}

// MappedPort returns 'external' port based on network mode.
// EC2 bridge uses a random host port while EC2 host/awsvpc uses a port specified by the user.
func (t *taskAnnotated) MappedPort(def ecstypes.ContainerDefinition, containerPort int32) (int32, error) {
	mode := t.Definition.NetworkMode
	// the error is same for all network modes (if any)
	errNotFound := &errMappedPortNotFound{
		TaskArn:       aws.ToString(t.Task.TaskArn),
		NetworkMode:   mode,
		ContainerName: aws.ToString(def.Name),
		ContainerPort: containerPort,
	}
	switch mode {
	case ecstypes.NetworkModeNone:
		return 0, errNotFound
	case ecstypes.NetworkModeHost, ecstypes.NetworkModeAwsvpc:
		// taskDefinition->containerDefinitions->portMappings
		for _, v := range def.PortMappings {
			if containerPort == aws.ToInt32(v.ContainerPort) {
				return aws.ToInt32(v.HostPort), nil
			}
		}
		return 0, errNotFound
	case "", ecstypes.NetworkModeBridge:
		//  task->containers->networkBindings
		for _, c := range t.Task.Containers {
			if aws.ToString(def.Name) == aws.ToString(c.Name) {
				for _, b := range c.NetworkBindings {
					if containerPort == aws.ToInt32(b.ContainerPort) {
						return aws.ToInt32(b.HostPort), nil
					}
				}
			}
		}
		return 0, errNotFound
	default:
		return 0, errNotFound
	}
}
