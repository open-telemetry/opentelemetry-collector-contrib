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
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/stretchr/testify/assert"
)

func TestProcess(t *testing.T) {
	taskList := []*ECSTask{
		{
			TaskDefinition: &ecs.TaskDefinition{
				ContainerDefinitions: []*ecs.ContainerDefinition{
					{
						DockerLabels: map[string]*string{
							"LABEL1": nil,
							"LABEL2": nil,
						},
					},
					{
						DockerLabels: map[string]*string{
							"PORT_LABEL": nil,
							"LABEL1":     nil,
						},
					},
				},
			},
		},
		{
			TaskDefinition: &ecs.TaskDefinition{
				TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-2:1234567890:task-definition/prometheus-java-tomcat-fargate-awsvpc:1"),
				ContainerDefinitions: []*ecs.ContainerDefinition{
					{
						Name: aws.String("envoy"),
					},
				},
			},
		},
		{
			TaskDefinition: &ecs.TaskDefinition{
				TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-2:1234567890:task-definition/prometheus-java-tomcat-fargate-awsvpc:1"),
				ContainerDefinitions: []*ecs.ContainerDefinition{
					{
						Name: aws.String("envoy"),
						DockerLabels: map[string]*string{
							"LABEL1": nil,
							"LABEL2": nil,
						},
					},
					{
						DockerLabels: map[string]*string{
							"PORT_LABEL": nil,
							"LABEL1":     nil,
						},
					},
				},
			},
		},
		{
			TaskDefinition: &ecs.TaskDefinition{
				TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-2:1234567890:task-definition/prometheus-java-tomcat-fargate-awsvpc:1"),
				ContainerDefinitions: []*ecs.ContainerDefinition{
					{
						Name: aws.String("envoy_test"),
						DockerLabels: map[string]*string{
							"LABEL1": nil,
							"LABEL2": nil,
						},
					},
				},
			},
		},
	}

	taskDefsConfig := []*TaskDefinitionConfig{
		{TaskDefArnPattern: "^.*prometheus-java-jar-ec2-bridge:2$"},
		{
			TaskDefArnPattern:    "^.*prometheus-java-tomcat-fargate-awsvpc:[1-9][0-9]*$",
			ContainerNamePattern: "^envoy$",
		},
		{TaskDefArnPattern: "^.*task:[0-9]+$"},
	}
	for _, taskDef := range taskDefsConfig {
		taskDef.init()
	}

	p := TaskFilterProcessor{
		dockerPortLabel: "PORT_LABEL",
		taskDefsConfig:  taskDefsConfig,
	}

	processedTasks, err := p.Process("", taskList)
	assert.Nil(t, err)

	assert.Equal(t, 3, len(processedTasks))
	assert.Equal(t, true, processedTasks[0].DockerLabelBased)
	assert.Equal(t, false, processedTasks[0].TaskDefinitionBased)
	assert.Equal(t, false, processedTasks[1].DockerLabelBased)
	assert.Equal(t, true, processedTasks[1].TaskDefinitionBased)
	assert.Equal(t, true, processedTasks[2].DockerLabelBased)
	assert.Equal(t, true, processedTasks[2].TaskDefinitionBased)
}

func TestDiscoverDockerLabelBased(t *testing.T) {
	testCases := []struct {
		testName           string
		containerDefs      []*ecs.ContainerDefinition
		dockerPortLabel    string
		isDockerLabelBased bool
	}{
		{
			"has docker label",
			[]*ecs.ContainerDefinition{
				{
					DockerLabels: map[string]*string{
						"LABEL1": nil,
						"LABEL2": nil,
					},
				},
				{
					DockerLabels: map[string]*string{
						"PORT_LABEL": nil,
						"LABEL1":     nil,
					},
				},
			},
			"PORT_LABEL",
			true,
		},
		{
			"no docker label",
			[]*ecs.ContainerDefinition{
				{
					DockerLabels: map[string]*string{
						"LABEL1": nil,
						"LABEL2": nil,
					},
				},
				{
					DockerLabels: map[string]*string{
						"LABEL1": nil,
					},
				},
			},
			"PORT_LABEL",
			false,
		},
		{
			"empty port label",
			[]*ecs.ContainerDefinition{
				{
					DockerLabels: map[string]*string{
						"LABEL1": nil,
						"LABEL2": nil,
					},
				},
				{
					DockerLabels: map[string]*string{
						"PORT_LABEL": nil,
						"LABEL1":     nil,
					},
				},
			},
			"",
			false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			p := TaskFilterProcessor{
				dockerPortLabel: tc.dockerPortLabel,
			}

			task := &ECSTask{
				TaskDefinition: &ecs.TaskDefinition{
					ContainerDefinitions: tc.containerDefs,
				},
			}

			p.discoverDockerLabelBased(task)

			assert.Equal(t, tc.isDockerLabelBased, task.DockerLabelBased)
		})
	}
}

func TestDiscoverTaskDefinitionBased(t *testing.T) {
	testCases := []struct {
		testName       string
		taskDefsConfig []*TaskDefinitionConfig
		taskDef        *ecs.TaskDefinition
		isTaskDefBased bool
	}{
		{
			"matches a task def ARN pattern w/o container name pattern",
			[]*TaskDefinitionConfig{
				{TaskDefArnPattern: "^.*prometheus-java-jar-ec2-bridge:2$"},
				{TaskDefArnPattern: "^.*prometheus-java-tomcat-fargate-awsvpc:[1-9][0-9]*$"},
				{TaskDefArnPattern: "^.*task:[0-9]+$"},
			},
			&ecs.TaskDefinition{
				TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-2:1234567890:task-definition/prometheus-java-tomcat-fargate-awsvpc:1"),
			},
			true,
		},
		{
			"matches a task def ARN pattern w/ matching container name pattern",
			[]*TaskDefinitionConfig{
				{TaskDefArnPattern: "^.*prometheus-java-jar-ec2-bridge:2$"},
				{
					TaskDefArnPattern:    "^.*prometheus-java-tomcat-fargate-awsvpc:[1-9][0-9]*$",
					ContainerNamePattern: "^envoy$",
				},
				{TaskDefArnPattern: "^.*task:[0-9]+$"},
			},
			&ecs.TaskDefinition{
				TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-2:1234567890:task-definition/prometheus-java-tomcat-fargate-awsvpc:1"),
				ContainerDefinitions: []*ecs.ContainerDefinition{
					{
						Name: aws.String("envoy"),
					},
				},
			},
			true,
		},
		{
			"matches a task def ARN pattern w/ no matching container name pattern",
			[]*TaskDefinitionConfig{
				{TaskDefArnPattern: "^.*prometheus-java-jar-ec2-bridge:2$"},
				{
					TaskDefArnPattern:    "^.*prometheus-java-tomcat-fargate-awsvpc:[1-9][0-9]*$",
					ContainerNamePattern: "^envoy$",
				},
				{TaskDefArnPattern: "^.*task:[0-9]+$"},
			},
			&ecs.TaskDefinition{
				TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-2:1234567890:task-definition/prometheus-java-tomcat-fargate-awsvpc:1"),
				ContainerDefinitions: []*ecs.ContainerDefinition{
					{
						Name: aws.String("envoy_test"),
					},
				},
			},
			false,
		},
		{
			"no match",
			[]*TaskDefinitionConfig{
				{TaskDefArnPattern: "^.*prometheus-java-jar-ec2-bridge:2$"},
				{TaskDefArnPattern: "^.*prometheus-java-tomcat-fargate-awsvpc:[1-9][0-9]*$"},
				{TaskDefArnPattern: "^.*task:[0-9]+$"},
			},
			&ecs.TaskDefinition{
				TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-2:1234567890:task-definition/prometheus-java-jar-ec2-bridge:3"),
			},
			false,
		},
		{
			"no task def ARN",
			[]*TaskDefinitionConfig{
				{TaskDefArnPattern: "^.*prometheus-java-jar-ec2-bridge:2$"},
				{TaskDefArnPattern: "^.*prometheus-java-tomcat-fargate-awsvpc:[1-9][0-9]*$"},
				{TaskDefArnPattern: "^.*task:[0-9]+$"},
			},
			&ecs.TaskDefinition{
				TaskDefinitionArn: nil,
			},
			false,
		},
		{
			"no task def ARN pattern",
			nil,
			&ecs.TaskDefinition{
				TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-2:1234567890:task-definition/prometheus-java-tomcat-fargate-awsvpc:1"),
			},
			false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			for _, taskDef := range tc.taskDefsConfig {
				taskDef.init()
			}
			p := TaskFilterProcessor{
				taskDefsConfig: tc.taskDefsConfig,
			}

			task := &ECSTask{
				TaskDefinition: tc.taskDef,
			}

			p.discoverTaskDefinitionBased(task)
			assert.Equal(t, tc.isTaskDefBased, task.TaskDefinitionBased)
		})
	}
}
