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

package ecsmock // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecsobserver/internal/ecsmock"

import (
	"context"
	"fmt"
	"reflect"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ec2"
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
	DescribeInstanceOutput         int // max 1000

}

func DefaultPageLimit() PageLimit {
	return PageLimit{
		ListTaskOutput:                 100,
		DescribeTaskInput:              100,
		ListServiceOutput:              10,
		DescribeServiceInput:           10,
		DescribeContainerInstanceInput: 100,
		DescribeInstanceOutput:         1000,
	}
}

// Cluster implements both ECS and EC2 API for a single cluster.
type Cluster struct {
	name                  string                         // optional
	definitions           map[string]*ecs.TaskDefinition // key is task definition arn
	taskMap               map[string]*ecs.Task           // key is task arn
	taskList              []*ecs.Task
	containerInstanceMap  map[string]*ecs.ContainerInstance // key is container instance arn
	containerInstanceList []*ecs.ContainerInstance
	ec2Map                map[string]*ec2.Instance // key is instance id
	ec2List               []*ec2.Instance
	serviceMap            map[string]*ecs.Service
	serviceList           []*ecs.Service
	limit                 PageLimit
	stats                 ClusterStats
}

// NewCluster creates a mock ECS cluster with default limits.
func NewCluster() *Cluster {
	return NewClusterWithName("")
}

// NewClusterWithName creates a cluster that checks for cluster name if request includes a non empty cluster name.
func NewClusterWithName(name string) *Cluster {
	return &Cluster{
		name: name,
		// NOTE: we don't set the maps by design, they should be injected and API calls
		// without setting up data should just panic.
		limit: DefaultPageLimit(),
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
	if err := checkCluster(input.Cluster, c.name); err != nil {
		return nil, err
	}
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
	if err := checkCluster(input.Cluster, c.name); err != nil {
		return nil, err
	}
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

func (c *Cluster) DescribeTaskDefinitionWithContext(_ context.Context, input *ecs.DescribeTaskDefinitionInput, _ ...request.Option) (*ecs.DescribeTaskDefinitionOutput, error) {
	c.stats.DescribeTaskDefinition.Called++
	defArn := aws.StringValue(input.TaskDefinition)
	def, ok := c.definitions[defArn]
	if !ok {
		c.stats.DescribeTaskDefinition.Error++
		return nil, fmt.Errorf("task definition not found for arn %q", defArn)
	}
	return &ecs.DescribeTaskDefinitionOutput{TaskDefinition: def}, nil
}

func (c *Cluster) DescribeContainerInstancesWithContext(_ context.Context, input *ecs.DescribeContainerInstancesInput, _ ...request.Option) (*ecs.DescribeContainerInstancesOutput, error) {
	if err := checkCluster(input.Cluster, c.name); err != nil {
		return nil, err
	}
	var (
		instances []*ecs.ContainerInstance
		failures  []*ecs.Failure
	)
	for _, cid := range input.ContainerInstances {
		ci, ok := c.containerInstanceMap[aws.StringValue(cid)]
		if !ok {
			failures = append(failures, &ecs.Failure{
				Arn:    cid,
				Detail: aws.String(fmt.Sprintf("container instance not found %s", aws.StringValue(cid))),
				Reason: aws.String("container instance not found"),
			})
			continue
		}
		instances = append(instances, ci)
	}
	return &ecs.DescribeContainerInstancesOutput{ContainerInstances: instances, Failures: failures}, nil
}

// DescribeInstancesWithContext supports get all the instances and get instance by ids.
// It does NOT support filter. Result always has a single reservation, which is not the case in actual EC2 API.
func (c *Cluster) DescribeInstancesWithContext(_ context.Context, input *ec2.DescribeInstancesInput, _ ...request.Option) (*ec2.DescribeInstancesOutput, error) {
	var (
		instances []*ec2.Instance
		nextToken *string
	)
	if len(input.InstanceIds) != 0 {
		for _, id := range input.InstanceIds {
			ins, ok := c.ec2Map[aws.StringValue(id)]
			if !ok {
				return nil, fmt.Errorf("instance %q not found", aws.StringValue(id))
			}
			instances = append(instances, ins)
		}
	} else {
		page, err := getPage(pageInput{
			nextToken: input.NextToken,
			size:      len(c.ec2List),
			limit:     c.limit.DescribeInstanceOutput,
		})
		if err != nil {
			return nil, err
		}
		instances = c.ec2List[page.start:page.end]
		nextToken = page.nextToken
	}
	return &ec2.DescribeInstancesOutput{
		Reservations: []*ec2.Reservation{
			{
				Instances: instances,
			},
		},
		NextToken: nextToken,
	}, nil
}

func (c *Cluster) ListServicesWithContext(_ context.Context, input *ecs.ListServicesInput, _ ...request.Option) (*ecs.ListServicesOutput, error) {
	if err := checkCluster(input.Cluster, c.name); err != nil {
		return nil, err
	}
	page, err := getPage(pageInput{
		nextToken: input.NextToken,
		size:      len(c.serviceList),
		limit:     c.limit.ListServiceOutput,
	})
	if err != nil {
		return nil, err
	}
	res := c.serviceList[page.start:page.end]
	return &ecs.ListServicesOutput{
		ServiceArns: getArns(res, func(i int) *string {
			return res[i].ServiceArn
		}),
		NextToken: page.nextToken,
	}, nil
}

func (c *Cluster) DescribeServicesWithContext(_ context.Context, input *ecs.DescribeServicesInput, _ ...request.Option) (*ecs.DescribeServicesOutput, error) {
	if err := checkCluster(input.Cluster, c.name); err != nil {
		return nil, err
	}
	var (
		failures []*ecs.Failure
		services []*ecs.Service
	)
	for i, serviceArn := range input.Services {
		arn := aws.StringValue(serviceArn)
		svc, ok := c.serviceMap[arn]
		if !ok {
			failures = append(failures, &ecs.Failure{
				Arn:    serviceArn,
				Detail: aws.String(fmt.Sprintf("service not found index %d arn %s total services %d", i, arn, len(c.serviceMap))),
				Reason: aws.String("service not found"),
			})
			continue
		}
		services = append(services, svc)
	}
	return &ecs.DescribeServicesOutput{Failures: failures, Services: services}, nil
}

// API End

// Hook Start

// SetTasks update both list and map.
func (c *Cluster) SetTasks(tasks []*ecs.Task) {
	m := make(map[string]*ecs.Task, len(tasks))
	for _, t := range tasks {
		m[aws.StringValue(t.TaskArn)] = t
	}
	c.taskMap = m
	c.taskList = tasks
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

// SetContainerInstances updates the list and map.
func (c *Cluster) SetContainerInstances(instances []*ecs.ContainerInstance) {
	m := make(map[string]*ecs.ContainerInstance, len(instances))
	for _, ci := range instances {
		m[aws.StringValue(ci.ContainerInstanceArn)] = ci
	}
	c.containerInstanceMap = m
	c.containerInstanceList = instances
}

// SetEc2Instances updates the list and map.
func (c *Cluster) SetEc2Instances(instances []*ec2.Instance) {
	m := make(map[string]*ec2.Instance, len(instances))
	for _, i := range instances {
		m[aws.StringValue(i.InstanceId)] = i
	}
	c.ec2Map = m
	c.ec2List = instances
}

// SetServices updates the list and map.
func (c *Cluster) SetServices(services []*ecs.Service) {
	m := make(map[string]*ecs.Service, len(services))
	for _, s := range services {
		m[aws.StringValue(s.ServiceArn)] = s
	}
	c.serviceMap = m
	c.serviceList = services
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

func GenContainerInstances(arnPrefix string, count int, modifier func(i int, ci *ecs.ContainerInstance)) []*ecs.ContainerInstance {
	var instances []*ecs.ContainerInstance
	for i := 0; i < count; i++ {
		ci := &ecs.ContainerInstance{
			ContainerInstanceArn: aws.String(fmt.Sprintf("%s%d", arnPrefix, i)),
		}
		if modifier != nil {
			modifier(i, ci)
		}
		instances = append(instances, ci)
	}
	return instances
}

func GenEc2Instances(idPrefix string, count int, modifier func(i int, ins *ec2.Instance)) []*ec2.Instance {
	var instances []*ec2.Instance
	for i := 0; i < count; i++ {
		ins := &ec2.Instance{
			InstanceId: aws.String(fmt.Sprintf("%s%d", idPrefix, i)),
		}
		if modifier != nil {
			modifier(i, ins)
		}
		instances = append(instances, ins)
	}
	return instances
}

func GenServices(arnPrefix string, count int, modifier func(i int, s *ecs.Service)) []*ecs.Service {
	var services []*ecs.Service
	for i := 0; i < count; i++ {
		svc := &ecs.Service{
			ServiceArn:  aws.String(fmt.Sprintf("%s%d", arnPrefix, i)),
			ServiceName: aws.String(fmt.Sprintf("%s%d", arnPrefix, i)),
		}
		if modifier != nil {
			modifier(i, svc)
		}
		services = append(services, svc)
	}
	return services
}

// Generator End

var _ awserr.Error = (*ecsError)(nil)

// ecsError implements awserr.Error interface
type ecsError struct {
	code    string
	message string
}

func (e *ecsError) Code() string {
	return e.code
}

func (e *ecsError) Message() string {
	return e.message
}

func (e *ecsError) Error() string {
	return "code " + e.code + " message " + e.message
}

func (e *ecsError) OrigErr() error {
	return nil
}

func checkCluster(reqClusterName *string, mockClusterName string) error {
	if reqClusterName == nil || mockClusterName == "" || *reqClusterName == mockClusterName {
		return nil
	}
	return &ecsError{
		code:    ecs.ErrCodeClusterNotFoundException,
		message: fmt.Sprintf("Want cluster %s but the mock cluster is %s", *reqClusterName, mockClusterName),
	}
}

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
