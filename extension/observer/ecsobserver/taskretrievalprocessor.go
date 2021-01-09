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

	"github.com/aws/aws-sdk-go/service/ecs"
	"go.uber.org/zap"
)

// TaskRetrievalProcessor gets all running tasks for the target cluster.
type TaskRetrievalProcessor struct {
	svcEcs *ecs.ECS
	logger *zap.Logger
}

func (p *TaskRetrievalProcessor) ProcessorName() string {
	return "TaskRetrievalProcessor"
}

// Process retrieves a list of ECS tasks using the ECS API.
func (p *TaskRetrievalProcessor) Process(clusterName string, taskList []*ECSTask) ([]*ECSTask, error) {
	listTasksInput := &ecs.ListTasksInput{Cluster: &clusterName}

	for {
		// List all running task ARNs in the cluster
		listTasksResp, listTasksErr := p.svcEcs.ListTasks(listTasksInput)
		if listTasksErr != nil {
			return taskList, fmt.Errorf("failed to list task ARNs for %s. Error: %s", clusterName, listTasksErr.Error())
		}

		// Retrieve tasks from task ARNs
		descTasksInput := &ecs.DescribeTasksInput{
			Cluster: &clusterName,
			Tasks:   listTasksResp.TaskArns,
		}
		descTasksResp, descTasksErr := p.svcEcs.DescribeTasks(descTasksInput)
		if descTasksErr != nil {
			return taskList, fmt.Errorf("failed to describe ECS Tasks for %s. Error: %s", clusterName, descTasksErr.Error())
		}

		for _, f := range descTasksResp.Failures {
			p.logger.Debug(
				"DescribeTask Failure",
				zap.String("ARN", *f.Arn),
				zap.String("Reason", *f.Reason),
				zap.String("Detail", *f.Detail),
			)
		}

		for _, task := range descTasksResp.Tasks {
			ecsTask := &ECSTask{
				Task: task,
			}
			taskList = append(taskList, ecsTask)
		}

		if listTasksResp.NextToken == nil {
			break
		}
		listTasksInput.NextToken = listTasksResp.NextToken
	}
	return taskList, nil
}

func NewTaskRetrievalProcessor(ecs *ecs.ECS, logger *zap.Logger) *TaskRetrievalProcessor {
	return &TaskRetrievalProcessor{
		svcEcs: ecs,
		logger: logger,
	}
}
