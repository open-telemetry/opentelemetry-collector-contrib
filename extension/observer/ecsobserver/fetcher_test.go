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
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecsobserver/internal/ecsmock"
)

func TestFetcher_GetAllTasks(t *testing.T) {
	c := ecsmock.NewCluster()
	f, err := newTaskFetcher(taskFetcherOptions{
		Logger:      zap.NewExample(),
		Cluster:     "not used",
		Region:      "not used",
		ecsOverride: c,
	})
	require.NoError(t, err)
	const nTasks = 203
	c.SetTasks(ecsmock.GenTasks("p", nTasks, nil))
	ctx := context.Background()
	tasks, err := f.GetAllTasks(ctx)
	require.NoError(t, err)
	assert.Equal(t, nTasks, len(tasks))
}

func TestFetcher_AttachTaskDefinitions(t *testing.T) {
	c := ecsmock.NewCluster()
	f, err := newTaskFetcher(taskFetcherOptions{
		Logger:      zap.NewExample(),
		Cluster:     "not used",
		Region:      "not used",
		ecsOverride: c,
	})
	require.NoError(t, err)

	const nTasks = 5
	ctx := context.Background()
	// one task per def
	c.SetTasks(ecsmock.GenTasks("p", nTasks, func(i int, task *ecs.Task) {
		task.TaskDefinitionArn = aws.String(fmt.Sprintf("pdef%d:1", i))
	}))
	c.SetTaskDefinitions(ecsmock.GenTaskDefinitions("pdef", nTasks, 1, nil))

	// no cache
	tasks, err := f.GetAllTasks(ctx)
	require.NoError(t, err)
	attached, err := f.AttachTaskDefinition(ctx, tasks)
	stats := c.Stats()
	require.NoError(t, err)
	assert.Equal(t, len(tasks), len(attached))
	assert.Equal(t, nTasks, stats.DescribeTaskDefinition.Called)

	// all cached
	tasks, err = f.GetAllTasks(ctx)
	require.NoError(t, err)
	// do it again to trigger cache logic
	attached, err = f.AttachTaskDefinition(ctx, tasks)
	stats = c.Stats()
	require.NoError(t, err)
	assert.Equal(t, len(tasks), len(attached))
	assert.Equal(t, nTasks, stats.DescribeTaskDefinition.Called) // no api call due to cache

	// add a new task
	c.SetTasks(ecsmock.GenTasks("p", nTasks+1, func(i int, task *ecs.Task) {
		task.TaskDefinitionArn = aws.String(fmt.Sprintf("pdef%d:1", i))
	}))
	c.SetTaskDefinitions(ecsmock.GenTaskDefinitions("pdef", nTasks+1, 1, nil))
	tasks, err = f.GetAllTasks(ctx)
	require.NoError(t, err)
	_, err = f.AttachTaskDefinition(ctx, tasks)
	stats = c.Stats()
	require.NoError(t, err)
	assert.Equal(t, nTasks+1, stats.DescribeTaskDefinition.Called) // +1 for new task
}
