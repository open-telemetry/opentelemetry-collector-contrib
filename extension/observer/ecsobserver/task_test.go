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
	"github.com/stretchr/testify/require"
)

func TestTask_MappedPort(t *testing.T) {
	// Network mode is optional for ecs and it default to ec2 bridge
	t.Run("empty is ec2 bridge", func(t *testing.T) {
		task := Task{
			Task: &ecs.Task{
				TaskArn: aws.String("arn:task:1"),
				Containers: []*ecs.Container{
					{
						Name: aws.String("c1"),
						NetworkBindings: []*ecs.NetworkBinding{
							{
								ContainerPort: aws.Int64(2112),
								HostPort:      aws.Int64(2345),
							},
						},
					},
				},
			},
			Definition: &ecs.TaskDefinition{NetworkMode: nil},
			EC2:        &ec2.Instance{PrivateIpAddress: aws.String("172.168.1.1")},
		}
		ip, err := task.PrivateIP()
		require.Nil(t, err)
		assert.Equal(t, "172.168.1.1", ip)
		p, err := task.MappedPort(&ecs.ContainerDefinition{Name: aws.String("c1")}, 2112)
		require.Nil(t, err)
		assert.Equal(t, int64(2345), p)
	})
}

func TestTask_AddMatchedContainer(t *testing.T) {
	task := Task{
		Matched: []MatchedContainer{
			{
				ContainerIndex: 0,
				Targets: []MatchedTarget{
					{
						MatcherType: MatcherTypeService,
						Port:        1,
					},
				},
			},
		},
	}

	// Different container
	task.AddMatchedContainer(MatchedContainer{
		ContainerIndex: 1,
		Targets: []MatchedTarget{
			{
				MatcherType: MatcherTypeDockerLabel,
				Port:        2,
			},
		},
	})
	assert.Equal(t, []MatchedContainer{
		{
			ContainerIndex: 0,
			Targets: []MatchedTarget{
				{
					MatcherType: MatcherTypeService,
					Port:        1,
				},
			},
		},
		{
			ContainerIndex: 1,
			Targets: []MatchedTarget{
				{
					MatcherType: MatcherTypeDockerLabel,
					Port:        2,
				},
			},
		},
	}, task.Matched)

	// Same container different metrics path
	task.AddMatchedContainer(MatchedContainer{
		ContainerIndex: 0,
		Targets: []MatchedTarget{
			{
				MatcherType: MatcherTypeTaskDefinition,
				Port:        1,
				MetricsPath: "/metrics2",
			},
		},
	})
	assert.Equal(t, []MatchedContainer{
		{
			ContainerIndex: 0,
			Targets: []MatchedTarget{
				{
					MatcherType: MatcherTypeService,
					Port:        1,
				},
				{
					MatcherType: MatcherTypeTaskDefinition,
					Port:        1,
					MetricsPath: "/metrics2",
				},
			},
		},
		{
			ContainerIndex: 1,
			Targets: []MatchedTarget{
				{
					MatcherType: MatcherTypeDockerLabel,
					Port:        2,
				},
			},
		},
	}, task.Matched)
}
