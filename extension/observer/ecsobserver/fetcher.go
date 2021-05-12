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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ecs"
	"go.uber.org/zap"
)

// ecsClient includes API required by taskFetcher.
type ecsClient interface {
	ListTasksWithContext(ctx context.Context, input *ecs.ListTasksInput, opts ...request.Option) (*ecs.ListTasksOutput, error)
	DescribeTasksWithContext(ctx context.Context, input *ecs.DescribeTasksInput, opts ...request.Option) (*ecs.DescribeTasksOutput, error)
}

type taskFetcher struct {
	logger  *zap.Logger
	ecs     ecsClient
	cluster string
}

type taskFetcherOptions struct {
	Logger  *zap.Logger
	Cluster string
	Region  string

	// test overrides
	ecsOverride ecsClient
}

func newTaskFetcher(opts taskFetcherOptions) (*taskFetcher, error) {
	fetcher := taskFetcher{
		logger:  opts.Logger,
		ecs:     opts.ecsOverride,
		cluster: opts.Cluster,
	}
	// Return early if clients are mocked
	if fetcher.ecs != nil {
		return &fetcher, nil
	}
	return nil, fmt.Errorf("actual aws init logic not implemented")
}

// GetAllTasks get arns of all running tasks and describe those tasks.
// There is no API to list task detail without arn so we need to call two APIs.
func (f *taskFetcher) GetAllTasks(ctx context.Context) ([]*ecs.Task, error) {
	svc := f.ecs
	cluster := aws.String(f.cluster)
	req := ecs.ListTasksInput{Cluster: cluster}
	var tasks []*ecs.Task
	for {
		listRes, err := svc.ListTasksWithContext(ctx, &req)
		if err != nil {
			return nil, fmt.Errorf("ecs.ListTasks failed: %w", err)
		}
		// NOTE: the limit for list task response and describe task request are both 100.
		descRes, err := svc.DescribeTasksWithContext(ctx, &ecs.DescribeTasksInput{
			Cluster: cluster,
			Tasks:   listRes.TaskArns,
		})
		if err != nil {
			return nil, fmt.Errorf("ecs.DescribeTasks failed: %w", err)
		}
		tasks = append(tasks, descRes.Tasks...)
		if listRes.NextToken == nil {
			break
		}
		req.NextToken = listRes.NextToken
	}
	return tasks, nil
}
