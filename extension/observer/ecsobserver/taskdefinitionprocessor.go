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
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/hashicorp/golang-lru/simplelru"
)

const (
	// ECS Service Quota: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/service-quotas.html
	taskDefCacheSize = 2000
)

// TaskDefinitionProcessor gets all running tasks for the target cluster.
type TaskDefinitionProcessor struct {
	svcEcs *ecs.ECS

	taskDefCache *simplelru.LRU
}

func (p *TaskDefinitionProcessor) ProcessorName() string {
	return "TaskDefinitionProcessor"
}

// Process retrieves the task definition for each ECS task.
// Tasks that don't have a task definition are also filtered out.
func (p *TaskDefinitionProcessor) Process(cluster string, taskList []*ECSTask) ([]*ECSTask, error) {
	numValidTasks := 0
	for _, task := range taskList {
		arn := aws.StringValue(task.Task.TaskDefinitionArn)
		if arn == "" {
			continue
		}

		if res, ok := p.taskDefCache.Get(arn); ok {
			// Try retrieving from cache
			task.TaskDefinition = res.(*ecs.TaskDefinition)
			numValidTasks++
		} else {
			// Query API
			input := &ecs.DescribeTaskDefinitionInput{TaskDefinition: &arn}
			resp, err := p.svcEcs.DescribeTaskDefinition(input)
			if err != nil {
				return taskList, fmt.Errorf("failed to describe task definition for %s. Error: %s", arn, err.Error())
			}

			if taskDef := resp.TaskDefinition; taskDef != nil {
				task.TaskDefinition = taskDef
				p.taskDefCache.Add(arn, taskDef)
				numValidTasks++
			}
		}
	}

	// Filter out tasks that don't have a task definition
	filteredTaskList := make([]*ECSTask, numValidTasks)
	idx := 0
	for _, task := range taskList {
		if task.TaskDefinition == nil {
			continue
		}
		filteredTaskList[idx] = task
		idx++
	}

	return filteredTaskList, nil
}

func NewTaskDefinitionProcessor(ecs *ecs.ECS) *TaskDefinitionProcessor {
	// Initiate the task definition LRU caching
	lru, _ := simplelru.NewLRU(taskDefCacheSize, nil)

	return &TaskDefinitionProcessor{
		svcEcs:       ecs,
		taskDefCache: lru,
	}
}
