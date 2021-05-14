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
	"reflect"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ecs"
)

// PageLimit defines number of items in a single page for different APIs.
// Those numbers can be found on the Input and and Output struct comments.
// Call DefaultPageLimit() to config the mock to use numbers same as the actual AWS API.
type PageLimit struct {
	ListTaskOutput                 int // default 100, max 100
	DescribeTaskInput              int // max 100
	ListServiceOutput              int // default 10, max 100
	DescribeServiceInput           int // max 10
	DescribeContainerInstanceInput int // max 100
}

func DefaultPageLimit() PageLimit {
	return PageLimit{
		ListTaskOutput:                 100,
		DescribeTaskInput:              100,
		ListServiceOutput:              10,
		DescribeServiceInput:           10,
		DescribeContainerInstanceInput: 100,
	}
}

// Cluster implements both ECS and EC2 API for a single cluster.
type Cluster struct {
	definitions map[string]*ecs.TaskDefinition
	taskList    []*ecs.Task
	taskMap     map[string]*ecs.Task
	limit       PageLimit
	stats       ClusterStats
}

// NewCluster creates a mock ECS cluster with default limits.
func NewCluster() *Cluster {
	return &Cluster{
		taskMap: make(map[string]*ecs.Task),
		limit:   DefaultPageLimit(),
	}
}

// APIStat keep track of individual API calls.
type APIStat struct {
	Called int
	Error  int
}

// ClusterStats keep track of API methods for one ECS cluster.
// Not all methods are tracked
type ClusterStats struct {
	DescribeTaskDefinition APIStat
}

// API Start

func (c *Cluster) Stats() ClusterStats {
	return c.stats
}

func (c *Cluster) ListTasksWithContext(_ context.Context, input *ecs.ListTasksInput, _ ...request.Option) (*ecs.ListTasksOutput, error) {
	page, err := getPage(pageInput{
		nextToken: input.NextToken,
		size:      len(c.taskList),
		limit:     c.limit.ListTaskOutput,
	})
	if err != nil {
		return nil, err
	}
	res := c.taskList[page.start:page.end]
	return &ecs.ListTasksOutput{
		TaskArns: getArns(res, func(i int) *string {
			return res[i].TaskArn
		}),
		NextToken: page.nextToken,
	}, nil
}

func (c *Cluster) DescribeTasksWithContext(_ context.Context, input *ecs.DescribeTasksInput, _ ...request.Option) (*ecs.DescribeTasksOutput, error) {
	var (
		failures []*ecs.Failure
		tasks    []*ecs.Task
	)
	for i, taskArn := range input.Tasks {
		arn := aws.StringValue(taskArn)
		task, ok := c.taskMap[arn]
		if !ok {
			failures = append(failures, &ecs.Failure{
				Arn:    taskArn,
				Detail: aws.String(fmt.Sprintf("task not found index %d arn %s total tasks %d", i, arn, len(c.taskMap))),
				Reason: aws.String("task not found"),
			})
			continue
		}
		tasks = append(tasks, task)
	}
	return &ecs.DescribeTasksOutput{Failures: failures, Tasks: tasks}, nil
}

func (c *Cluster) DescribeTaskDefinitionWithContext(_ context.Context, input *ecs.DescribeTaskDefinitionInput, opts ...request.Option) (*ecs.DescribeTaskDefinitionOutput, error) {
	c.stats.DescribeTaskDefinition.Called++
	defArn := aws.StringValue(input.TaskDefinition)
	def, ok := c.definitions[defArn]
	if !ok {
		c.stats.DescribeTaskDefinition.Error++
		return nil, fmt.Errorf("task definition not found for arn %q", defArn)
	}
	return &ecs.DescribeTaskDefinitionOutput{TaskDefinition: def}, nil
}

// API End

// Hook Start

// SetTasks update both list and map.
func (c *Cluster) SetTasks(tasks []*ecs.Task) {
	c.taskList = tasks
	m := make(map[string]*ecs.Task, len(tasks))
	for _, t := range tasks {
		m[aws.StringValue(t.TaskArn)] = t
	}
	c.taskMap = m
}

// SetTaskDefinitions updates the map.
// NOTE: we could have both list and map, but we are not using list task def in ecsobserver.
func (c *Cluster) SetTaskDefinitions(defs []*ecs.TaskDefinition) {
	m := make(map[string]*ecs.TaskDefinition, len(defs))
	for _, d := range defs {
		m[aws.StringValue(d.TaskDefinitionArn)] = d
	}
	c.definitions = m
}

// Hook End

// Generator Start

// GenTasks returns tasks with TaskArn set to arnPrefix+offset, where offset is [0, count).
func GenTasks(arnPrefix string, count int, modifier func(i int, task *ecs.Task)) []*ecs.Task {
	var tasks []*ecs.Task
	for i := 0; i < count; i++ {
		t := &ecs.Task{
			TaskArn: aws.String(arnPrefix + strconv.Itoa(i)),
		}
		if modifier != nil {
			modifier(i, t)
		}
		tasks = append(tasks, t)
	}
	return tasks
}

// GenTaskDefinitions returns tasks with TaskArn set to arnPrefix+offset+version, where offset is [0, count).
// e.g. foo0:1, foo1:1 the `:` is following the task family version syntax.
func GenTaskDefinitions(arnPrefix string, count int, version int, modifier func(i int, def *ecs.TaskDefinition)) []*ecs.TaskDefinition {
	var defs []*ecs.TaskDefinition
	for i := 0; i < count; i++ {
		d := &ecs.TaskDefinition{
			TaskDefinitionArn: aws.String(fmt.Sprintf("%s%d:%d", arnPrefix, i, version)),
		}
		if modifier != nil {
			modifier(i, d)
		}
		defs = append(defs, d)
	}
	return defs
}

// Generator End

// pagination Start

type pageInput struct {
	nextToken *string
	size      int
	limit     int
}

type pageOutput struct {
	start     int
	end       int
	nextToken *string
}

// getPage returns new page offset based on existing one.
// It is not using the actual AWS token format, it simply uses number string to keep track of offset.
func getPage(p pageInput) (*pageOutput, error) {
	var err error
	start := 0
	if p.nextToken != nil {
		token := aws.StringValue(p.nextToken)
		start, err = strconv.Atoi(token)
		if err != nil {
			return nil, fmt.Errorf("invalid next token %q: %w", token, err)
		}
	}
	end := minInt(p.size, start+p.limit)
	var newNextToken *string
	if end < p.size {
		newNextToken = aws.String(strconv.Itoa(end))
	}
	return &pageOutput{
		start:     start,
		end:       end,
		nextToken: newNextToken,
	}, nil
}

// pagination Emd

// 'generic' Start

// getArns is used by both ListTasks and ListServices
func getArns(items interface{}, arnGetter func(i int) *string) []*string {
	rv := reflect.ValueOf(items)
	var arns []*string
	for i := 0; i < rv.Len(); i++ {
		arns = append(arns, arnGetter(i))
	}
	return arns
}

// 'generic' End

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
