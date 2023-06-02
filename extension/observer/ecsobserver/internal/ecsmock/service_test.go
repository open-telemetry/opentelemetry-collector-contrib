// Copyright The OpenTelemetry Authors
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

package ecsmock

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCluster_ListTasksWithContext(t *testing.T) {
	ctx := context.Background()
	c := NewClusterWithName("c1")
	count := DefaultPageLimit().ListTaskOutput*2 + 1
	c.SetTasks(GenTasks("p", count, nil))

	t.Run("invalid cluster", func(t *testing.T) {
		req := &ecs.ListTasksInput{Cluster: aws.String("not c1")}
		_, err := c.ListTasksWithContext(ctx, req)
		require.Error(t, err)
		var aerr awserr.Error
		assert.True(t, errors.As(err, &aerr))
		assert.Equal(t, ecs.ErrCodeClusterNotFoundException, aerr.Code())
		assert.Equal(t, "code "+ecs.ErrCodeClusterNotFoundException+" message "+aerr.Message(), aerr.Error())
		assert.Nil(t, aerr.OrigErr())
	})

	t.Run("get all", func(t *testing.T) {
		req := &ecs.ListTasksInput{}
		listedTasks := 0
		pages := 0
		for {
			res, err := c.ListTasksWithContext(ctx, req)
			require.NoError(t, err)
			listedTasks += len(res.TaskArns)
			pages++
			if res.NextToken == nil {
				break
			}
			req.NextToken = res.NextToken
		}
		assert.Equal(t, count, listedTasks)
		assert.Equal(t, 3, pages)
	})

	t.Run("invalid token", func(t *testing.T) {
		req := &ecs.ListTasksInput{NextToken: aws.String("asd")}
		_, err := c.ListTasksWithContext(ctx, req)
		require.Error(t, err)
	})
}

func TestCluster_DescribeTasksWithContext(t *testing.T) {
	ctx := context.Background()
	c := NewClusterWithName("c1")
	count := 10
	c.SetTasks(GenTasks("p", count, func(i int, task *ecs.Task) {
		task.LastStatus = aws.String("running")
	}))

	t.Run("invalid cluster", func(t *testing.T) {
		req := &ecs.DescribeTasksInput{Cluster: aws.String("not c1")}
		_, err := c.DescribeTasksWithContext(ctx, req)
		require.Error(t, err)
	})

	t.Run("exists", func(t *testing.T) {
		req := &ecs.DescribeTasksInput{Tasks: []*string{aws.String("p0"), aws.String(fmt.Sprintf("p%d", count-1))}}
		res, err := c.DescribeTasksWithContext(ctx, req)
		require.NoError(t, err)
		assert.Len(t, res.Tasks, 2)
		assert.Len(t, res.Failures, 0)
		assert.Equal(t, "running", aws.StringValue(res.Tasks[0].LastStatus))
	})

	t.Run("not found", func(t *testing.T) {
		req := &ecs.DescribeTasksInput{Tasks: []*string{aws.String("p0"), aws.String(fmt.Sprintf("p%d", count))}}
		res, err := c.DescribeTasksWithContext(ctx, req)
		require.NoError(t, err)
		assert.Len(t, res.Tasks, 1)
		assert.Len(t, res.Failures, 1)
	})
}

func TestCluster_DescribeTaskDefinitionWithContext(t *testing.T) {
	ctx := context.Background()
	c := NewClusterWithName("c1")
	c.SetTaskDefinitions(GenTaskDefinitions("foo", 10, 1, nil)) // accept nil
	c.SetTaskDefinitions(GenTaskDefinitions("foo", 10, 1, func(i int, def *ecs.TaskDefinition) {
		def.NetworkMode = aws.String(ecs.NetworkModeBridge)
	}))

	t.Run("exists", func(t *testing.T) {
		req := &ecs.DescribeTaskDefinitionInput{TaskDefinition: aws.String("foo0:1")}
		res, err := c.DescribeTaskDefinitionWithContext(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, "foo0:1", aws.StringValue(res.TaskDefinition.TaskDefinitionArn))
		assert.Equal(t, ecs.NetworkModeBridge, aws.StringValue(res.TaskDefinition.NetworkMode))
	})

	t.Run("not found", func(t *testing.T) {
		before := c.Stats()
		req := &ecs.DescribeTaskDefinitionInput{TaskDefinition: aws.String("foo0:1+404")}
		_, err := c.DescribeTaskDefinitionWithContext(ctx, req)
		require.Error(t, err)
		after := c.Stats()
		assert.Equal(t, before.DescribeTaskDefinition.Error+1, after.DescribeTaskDefinition.Error)
	})
}

func TestCluster_DescribeInstancesWithContext(t *testing.T) {
	ctx := context.Background()
	c := NewCluster()
	count := 10000
	c.SetEc2Instances(GenEc2Instances("i-", count, nil))
	c.SetEc2Instances(GenEc2Instances("i-", count, func(i int, ins *ec2.Instance) {
		ins.Tags = []*ec2.Tag{
			{
				Key:   aws.String("my-id"),
				Value: aws.String(fmt.Sprintf("mid-%d", i)),
			},
		}
	}))

	t.Run("get all", func(t *testing.T) {
		req := &ec2.DescribeInstancesInput{}
		listedInstances := 0
		pages := 0
		for {
			res, err := c.DescribeInstancesWithContext(ctx, req)
			require.NoError(t, err)
			listedInstances += len(res.Reservations[0].Instances)
			pages++
			if res.NextToken == nil {
				break
			}
			req.NextToken = res.NextToken
		}
		assert.Equal(t, count, listedInstances)
		assert.Equal(t, 10, pages)
	})

	t.Run("get by id", func(t *testing.T) {
		var ids []*string
		nIds := 100
		for i := 0; i < nIds; i++ {
			ids = append(ids, aws.String(fmt.Sprintf("i-%d", i*10)))
		}
		req := &ec2.DescribeInstancesInput{InstanceIds: ids}
		res, err := c.DescribeInstancesWithContext(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, nIds, len(res.Reservations[0].Instances))
	})

	t.Run("invalid id", func(t *testing.T) {
		req := &ec2.DescribeInstancesInput{InstanceIds: []*string{aws.String("di-123")}}
		_, err := c.DescribeInstancesWithContext(ctx, req)
		require.Error(t, err)
	})

	t.Run("invalid token", func(t *testing.T) {
		req := &ec2.DescribeInstancesInput{NextToken: aws.String("asd")}
		_, err := c.DescribeInstancesWithContext(ctx, req)
		require.Error(t, err)
	})
}

func TestCluster_DescribeContainerInstancesWithContext(t *testing.T) {
	ctx := context.Background()
	c := NewClusterWithName("c1")
	count := 10
	c.SetContainerInstances(GenContainerInstances("foo", count, nil))
	c.SetContainerInstances(GenContainerInstances("foo", count, func(i int, ci *ecs.ContainerInstance) {
		ci.Ec2InstanceId = aws.String(fmt.Sprintf("i-%d", i))
	}))

	t.Run("invalid cluster", func(t *testing.T) {
		req := &ecs.DescribeContainerInstancesInput{Cluster: aws.String("not c1")}
		_, err := c.DescribeContainerInstancesWithContext(ctx, req)
		require.Error(t, err)
	})

	t.Run("get by id", func(t *testing.T) {
		var ids []*string
		nIds := count
		for i := 0; i < nIds; i++ {
			ids = append(ids, aws.String(fmt.Sprintf("foo%d", i)))
		}
		req := &ecs.DescribeContainerInstancesInput{ContainerInstances: ids}
		res, err := c.DescribeContainerInstancesWithContext(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, nIds, len(res.ContainerInstances))
		assert.Equal(t, 0, len(res.Failures))
	})

	t.Run("not found", func(t *testing.T) {
		req := &ecs.DescribeContainerInstancesInput{ContainerInstances: []*string{aws.String("ci-123")}}
		res, err := c.DescribeContainerInstancesWithContext(ctx, req)
		require.NoError(t, err)
		assert.Contains(t, aws.StringValue(res.Failures[0].Detail), "ci-123")
	})
}

func TestCluster_ListServicesWithContext(t *testing.T) {
	ctx := context.Background()
	c := NewClusterWithName("c1")
	count := 100
	c.SetServices(GenServices("s", count, nil))
	c.SetServices(GenServices("s", count, func(i int, s *ecs.Service) {
		s.Deployments = []*ecs.Deployment{
			{
				Status: aws.String("ACTIVE"),
				Id:     aws.String(fmt.Sprintf("d%d", i)),
			},
		}
	}))

	t.Run("invalid cluster", func(t *testing.T) {
		req := &ecs.ListServicesInput{Cluster: aws.String("not c1")}
		_, err := c.ListServicesWithContext(ctx, req)
		require.Error(t, err)
	})

	t.Run("get all", func(t *testing.T) {
		req := &ecs.ListServicesInput{}
		listedServices := 0
		pages := 0
		for {
			res, err := c.ListServicesWithContext(ctx, req)
			require.NoError(t, err)
			listedServices += len(res.ServiceArns)
			pages++
			if res.NextToken == nil {
				break
			}
			req.NextToken = res.NextToken
		}
		assert.Equal(t, count, listedServices)
		assert.Equal(t, 10, pages)
	})

	t.Run("invalid token", func(t *testing.T) {
		req := &ecs.ListServicesInput{NextToken: aws.String("asd")}
		_, err := c.ListServicesWithContext(ctx, req)
		require.Error(t, err)
	})
}

func TestCluster_DescribeServicesWithContext(t *testing.T) {
	ctx := context.Background()
	c := NewClusterWithName("c1")
	count := 100
	c.SetServices(GenServices("s", count, nil))

	t.Run("invalid cluster", func(t *testing.T) {
		req := &ecs.DescribeServicesInput{Cluster: aws.String("not c1")}
		_, err := c.DescribeServicesWithContext(ctx, req)
		require.Error(t, err)
	})

	t.Run("exists", func(t *testing.T) {
		req := &ecs.DescribeServicesInput{Services: []*string{aws.String("s0"), aws.String(fmt.Sprintf("s%d", count-1))}}
		res, err := c.DescribeServicesWithContext(ctx, req)
		require.NoError(t, err)
		assert.Len(t, res.Services, 2)
		assert.Len(t, res.Failures, 0)
	})

	t.Run("not found", func(t *testing.T) {
		req := &ecs.DescribeServicesInput{Services: []*string{aws.String("s0"), aws.String(fmt.Sprintf("s%d", count))}}
		res, err := c.DescribeServicesWithContext(ctx, req)
		require.NoError(t, err)
		assert.Len(t, res.Services, 1)
		assert.Len(t, res.Failures, 1)
	})
}
