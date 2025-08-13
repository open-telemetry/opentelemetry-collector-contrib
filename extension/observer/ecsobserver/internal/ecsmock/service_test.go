// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecsmock

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCluster_ListTasks(t *testing.T) {
	ctx := context.Background()
	c := NewClusterWithName("c1")
	count := DefaultPageLimit().ListTaskOutput*2 + 1
	c.SetTasks(GenTasks("p", count, nil))

	t.Run("invalid cluster", func(t *testing.T) {
		req := &ecs.ListTasksInput{Cluster: aws.String("not c1")}
		_, err := c.ListTasks(ctx, req)
		require.Error(t, err)
		var cnf *ecstypes.ClusterNotFoundException
		assert.ErrorAs(t, err, &cnf)
	})

	t.Run("get all", func(t *testing.T) {
		req := &ecs.ListTasksInput{}
		listedTasks := 0
		pages := 0
		for {
			res, err := c.ListTasks(ctx, req)
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
		_, err := c.ListTasks(ctx, req)
		require.Error(t, err)
	})
}

func TestCluster_DescribeTasks(t *testing.T) {
	ctx := context.Background()
	c := NewClusterWithName("c1")
	count := 10
	c.SetTasks(GenTasks("p", count, func(_ int, task *ecstypes.Task) {
		task.LastStatus = aws.String("running")
		task.Tags = []ecstypes.Tag{
			{
				Key:   aws.String("Name"),
				Value: aws.String("TestDescribeTaskWithTags"),
			},
		}
	}))

	t.Run("invalid cluster", func(t *testing.T) {
		req := &ecs.DescribeTasksInput{Cluster: aws.String("not c1")}
		_, err := c.DescribeTasks(ctx, req)
		require.Error(t, err)
	})

	t.Run("exists", func(t *testing.T) {
		req := &ecs.DescribeTasksInput{Tasks: []string{"p0", fmt.Sprintf("p%d", count-1)}}
		res, err := c.DescribeTasks(ctx, req)
		require.NoError(t, err)
		assert.Len(t, res.Tasks, 2)
		assert.Empty(t, res.Failures)
		assert.Equal(t, "running", *res.Tasks[0].LastStatus)
	})

	t.Run("not found", func(t *testing.T) {
		req := &ecs.DescribeTasksInput{Tasks: []string{"p0", fmt.Sprintf("p%d", count)}}
		res, err := c.DescribeTasks(ctx, req)
		require.NoError(t, err)
		assert.Len(t, res.Tasks, 1)
		assert.Len(t, res.Failures, 1)
	})

	t.Run("tags not found", func(t *testing.T) {
		req := &ecs.DescribeTasksInput{Include: []ecstypes.TaskField{ecstypes.TaskFieldTags}, Tasks: []string{"p0", fmt.Sprintf("p%d", count)}}
		res, err := c.DescribeTasks(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, res.Tasks[0].Tags[0].Key, aws.String("Name"))
		assert.Equal(t, res.Tasks[0].Tags[0].Value, aws.String("TestDescribeTaskWithTags"))
	})
}

func TestCluster_DescribeTaskDefinition(t *testing.T) {
	ctx := context.Background()
	c := NewClusterWithName("c1")
	c.SetTaskDefinitions(GenTaskDefinitions("foo", 10, 1, nil)) // accept nil
	c.SetTaskDefinitions(GenTaskDefinitions("foo", 10, 1, func(_ int, def *ecstypes.TaskDefinition) {
		def.NetworkMode = ecstypes.NetworkModeBridge
	}))

	t.Run("exists", func(t *testing.T) {
		req := &ecs.DescribeTaskDefinitionInput{TaskDefinition: aws.String("foo0:1")}
		res, err := c.DescribeTaskDefinition(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, "foo0:1", *res.TaskDefinition.TaskDefinitionArn)
		assert.Equal(t, ecstypes.NetworkModeBridge, res.TaskDefinition.NetworkMode)
	})

	t.Run("not found", func(t *testing.T) {
		before := c.Stats()
		req := &ecs.DescribeTaskDefinitionInput{TaskDefinition: aws.String("foo0:1+404")}
		_, err := c.DescribeTaskDefinition(ctx, req)
		require.Error(t, err)
		after := c.Stats()
		assert.Equal(t, before.DescribeTaskDefinition.Error+1, after.DescribeTaskDefinition.Error)
	})
}

func TestCluster_DescribeInstances(t *testing.T) {
	ctx := context.Background()
	c := NewCluster()
	count := 10000
	c.SetEc2Instances(GenEc2Instances("i-", count, nil))
	c.SetEc2Instances(GenEc2Instances("i-", count, func(i int, ins *ec2types.Instance) {
		ins.Tags = []ec2types.Tag{
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
			res, err := c.DescribeInstances(ctx, req)
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
		nIDs := 100
		ids := make([]string, 0, nIDs)
		for i := 0; i < nIDs; i++ {
			ids = append(ids, fmt.Sprintf("i-%d", i*10))
		}
		req := &ec2.DescribeInstancesInput{InstanceIds: ids}
		res, err := c.DescribeInstances(ctx, req)
		require.NoError(t, err)
		assert.Len(t, res.Reservations[0].Instances, nIDs)
	})

	t.Run("invalid id", func(t *testing.T) {
		req := &ec2.DescribeInstancesInput{InstanceIds: []string{"di-123"}}
		_, err := c.DescribeInstances(ctx, req)
		require.Error(t, err)
	})

	t.Run("invalid token", func(t *testing.T) {
		req := &ec2.DescribeInstancesInput{NextToken: aws.String("asd")}
		_, err := c.DescribeInstances(ctx, req)
		require.Error(t, err)
	})
}

func TestCluster_DescribeContainerInstances(t *testing.T) {
	ctx := context.Background()
	c := NewClusterWithName("c1")
	count := 10
	c.SetContainerInstances(GenContainerInstances("foo", count, nil))
	c.SetContainerInstances(GenContainerInstances("foo", count, func(i int, ci *ecstypes.ContainerInstance) {
		ci.Ec2InstanceId = aws.String(fmt.Sprintf("i-%d", i))
	}))

	t.Run("invalid cluster", func(t *testing.T) {
		req := &ecs.DescribeContainerInstancesInput{Cluster: aws.String("not c1")}
		_, err := c.DescribeContainerInstances(ctx, req)
		require.Error(t, err)
	})

	t.Run("get by id", func(t *testing.T) {
		nIDs := count
		ids := make([]string, 0, nIDs)
		for i := 0; i < nIDs; i++ {
			ids = append(ids, fmt.Sprintf("foo%d", i))
		}
		req := &ecs.DescribeContainerInstancesInput{ContainerInstances: ids}
		res, err := c.DescribeContainerInstances(ctx, req)
		require.NoError(t, err)
		assert.Len(t, res.ContainerInstances, nIDs)
		assert.Empty(t, res.Failures)
	})

	t.Run("not found", func(t *testing.T) {
		req := &ecs.DescribeContainerInstancesInput{ContainerInstances: []string{"ci-123"}}
		res, err := c.DescribeContainerInstances(ctx, req)
		require.NoError(t, err)
		assert.Contains(t, *res.Failures[0].Detail, "ci-123")
	})
}

func TestCluster_ListServices(t *testing.T) {
	ctx := context.Background()
	c := NewClusterWithName("c1")
	count := 100
	c.SetServices(GenServices("s", count, nil))
	c.SetServices(GenServices("s", count, func(i int, s *ecstypes.Service) {
		s.Deployments = []ecstypes.Deployment{
			{
				Status: aws.String("ACTIVE"),
				Id:     aws.String(fmt.Sprintf("d%d", i)),
			},
		}
	}))

	t.Run("invalid cluster", func(t *testing.T) {
		req := &ecs.ListServicesInput{Cluster: aws.String("not c1")}
		_, err := c.ListServices(ctx, req)
		require.Error(t, err)
	})

	t.Run("get all", func(t *testing.T) {
		req := &ecs.ListServicesInput{}
		listedServices := 0
		pages := 0
		for {
			res, err := c.ListServices(ctx, req)
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
		_, err := c.ListServices(ctx, req)
		require.Error(t, err)
	})
}

func TestCluster_DescribeServices(t *testing.T) {
	ctx := context.Background()
	c := NewClusterWithName("c1")
	count := 100
	c.SetServices(GenServices("s", count, nil))

	t.Run("invalid cluster", func(t *testing.T) {
		req := &ecs.DescribeServicesInput{Cluster: aws.String("not c1")}
		_, err := c.DescribeServices(ctx, req)
		require.Error(t, err)
	})

	t.Run("exists", func(t *testing.T) {
		req := &ecs.DescribeServicesInput{Services: []string{"s0", fmt.Sprintf("s%d", count-1)}}
		res, err := c.DescribeServices(ctx, req)
		require.NoError(t, err)
		assert.Len(t, res.Services, 2)
		assert.Empty(t, res.Failures)
	})

	t.Run("not found", func(t *testing.T) {
		req := &ecs.DescribeServicesInput{Services: []string{"s0", fmt.Sprintf("s%d", count)}}
		res, err := c.DescribeServices(ctx, req)
		require.NoError(t, err)
		assert.Len(t, res.Services, 1)
		assert.Len(t, res.Failures, 1)
	})
}
