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
	"github.com/aws/aws-sdk-go/aws"
)

// TaskFilterProcessor filters out tasks that don't match config.
type TaskFilterProcessor struct {
	taskDefsConfig  []*TaskDefinitionConfig
	dockerPortLabel string
}

func (p *TaskFilterProcessor) ProcessorName() string {
	return "TaskFilterProcessor"
}

// Process marks tasks that match the docker label config or task definition configs and filters
// out tasks that don't match any.
func (p *TaskFilterProcessor) Process(cluster string, taskList []*ECSTask) ([]*ECSTask, error) {
	var filteredTasks []*ECSTask

	for _, task := range taskList {
		p.discoverDockerLabelBased(task)
		p.discoverTaskDefinitionBased(task)

		if task.DockerLabelBased || task.TaskDefinitionBased {
			filteredTasks = append(filteredTasks, task)
		}
	}

	return filteredTasks, nil
}

// discoverDockerLabelBased checks if task matches docker label config and marks it if so.
func (p *TaskFilterProcessor) discoverDockerLabelBased(task *ECSTask) {
	if p.dockerPortLabel == "" {
		return
	}

	for _, containerDef := range task.TaskDefinition.ContainerDefinitions {
		if _, ok := containerDef.DockerLabels[p.dockerPortLabel]; ok {
			task.DockerLabelBased = true
			return
		}
	}
}

// discoverTaskDefinitionBased checks if task matches task definition configs and marks it if so.
func (p *TaskFilterProcessor) discoverTaskDefinitionBased(task *ECSTask) {
	if task.TaskDefinition.TaskDefinitionArn == nil {
		return
	}

	taskDefArn := aws.StringValue(task.TaskDefinition.TaskDefinitionArn)
	for _, taskDefConfig := range p.taskDefsConfig {
		if taskDefConfig.taskDefRegex.MatchString(taskDefArn) {
			if taskDefConfig.ContainerNamePattern == "" || containerNameMatches(task, taskDefConfig) {
				task.TaskDefinitionBased = true
				return
			}
		}
	}
}

// containerNameMatches checks if any of the task's container names matches the regex pattern
// defined in the task definition config.
func containerNameMatches(task *ECSTask, config *TaskDefinitionConfig) bool {
	for _, c := range task.TaskDefinition.ContainerDefinitions {
		if config.containerNameRegex.MatchString(aws.StringValue(c.Name)) {
			return true
		}
	}
	return false
}

func NewTaskFilterProcessor(taskDefsConfig []*TaskDefinitionConfig, dockerLabelConfig *DockerLabelConfig) *TaskFilterProcessor {
	dockerPortLabel := ""
	if dockerLabelConfig != nil {
		dockerPortLabel = dockerLabelConfig.PortLabel
	}

	return &TaskFilterProcessor{
		taskDefsConfig:  taskDefsConfig,
		dockerPortLabel: dockerPortLabel,
	}
}
