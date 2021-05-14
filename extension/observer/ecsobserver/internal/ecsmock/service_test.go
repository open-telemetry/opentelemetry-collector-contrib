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

package ecsmock

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCluster_ListTasksWithContext(t *testing.T) {
	ctx := context.Background()
	c := NewCluster()
	count := DefaultPageLimit().ListTaskOutput*2 + 1
	c.SetTasks(GenTasks("p", count, nil))

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
	c := NewCluster()
	count := 10
	c.SetTasks(GenTasks("p", count, func(i int, task *ecs.Task) {
		task.LastStatus = aws.String("running")
	}))

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
	c := NewCluster()
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
