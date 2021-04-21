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
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ecs"
)

// Task contains both raw task info and its definition.
// It is generated from TaskFetcher.
type Task struct {
	Task       *ecs.Task
	Definition *ecs.TaskDefinition
	EC2        *ec2.Instance
	Service    *ecs.Service
	Matched    []MatchedContainer
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
	if len(def.DockerLabels) == 0 {
		return nil
	}
	labels := make(map[string]string, len(def.DockerLabels))
	for k, v := range def.DockerLabels {
		labels[k] = aws.StringValue(v)
	}
	return labels
}
