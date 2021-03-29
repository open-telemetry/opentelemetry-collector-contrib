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
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/stretchr/testify/assert"
)

func TestTask_Tags(t *testing.T) {
	t.Run("ec2", func(t *testing.T) {
		task := Task{}
		assert.Equal(t, map[string]string(nil), task.EC2Tags())
		task.EC2 = &ec2.Instance{
			Tags: []*ec2.Tag{
				{
					Key:   aws.String("k"),
					Value: aws.String("v"),
				},
			},
		}
		assert.Equal(t, map[string]string{"k": "v"}, task.EC2Tags())
	})

	t.Run("task", func(t *testing.T) {
		task := Task{Task: &ecs.Task{}}
		assert.Equal(t, map[string]string(nil), task.TaskTags())
		task.Task.Tags = []*ecs.Tag{
			{
				Key:   aws.String("k"),
				Value: aws.String("v"),
			},
		}
		assert.Equal(t, map[string]string{"k": "v"}, task.TaskTags())
	})

	t.Run("container", func(t *testing.T) {
		task := Task{Definition: &ecs.TaskDefinition{ContainerDefinitions: []*ecs.ContainerDefinition{{}}}}
		assert.Equal(t, map[string]string(nil), task.ContainerLabels(0))
		task.Definition.ContainerDefinitions[0].DockerLabels = map[string]*string{
			"k": aws.String("v"),
		}
		assert.Equal(t, map[string]string{"k": "v"}, task.ContainerLabels(0))
	})
}
