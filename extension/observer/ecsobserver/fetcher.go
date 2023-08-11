// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecsobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecsobserver"

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/hashicorp/golang-lru/simplelru"
	"go.uber.org/zap"
)

const (
	// ECS Service Quota: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/service-quotas.html
	taskDefCacheSize = 2000
	// Based on existing number from cloudwatch-agent
	ec2CacheSize                   = 2000
	describeContainerInstanceLimit = 100
	describeServiceLimit           = 10
	// NOTE: these constants are not defined in go sdk, there are three values for deployment status.
	deploymentStatusActive  = "ACTIVE"
	deploymentStatusPrimary = "PRIMARY"
)

// ecsClient includes API required by taskFetcher.
type ecsClient interface {
	ListTasksWithContext(ctx context.Context, input *ecs.ListTasksInput, opts ...request.Option) (*ecs.ListTasksOutput, error)
	DescribeTasksWithContext(ctx context.Context, input *ecs.DescribeTasksInput, opts ...request.Option) (*ecs.DescribeTasksOutput, error)
	DescribeTaskDefinitionWithContext(ctx context.Context, input *ecs.DescribeTaskDefinitionInput, opts ...request.Option) (*ecs.DescribeTaskDefinitionOutput, error)
	DescribeContainerInstancesWithContext(ctx context.Context, input *ecs.DescribeContainerInstancesInput, opts ...request.Option) (*ecs.DescribeContainerInstancesOutput, error)
	ListServicesWithContext(ctx context.Context, input *ecs.ListServicesInput, opts ...request.Option) (*ecs.ListServicesOutput, error)
	DescribeServicesWithContext(ctx context.Context, input *ecs.DescribeServicesInput, opts ...request.Option) (*ecs.DescribeServicesOutput, error)
}

// ec2Client includes API required by TaskFetcher.
type ec2Client interface {
	DescribeInstancesWithContext(ctx context.Context, input *ec2.DescribeInstancesInput, opts ...request.Option) (*ec2.DescribeInstancesOutput, error)
}

type taskFetcher struct {
	logger            *zap.Logger
	ecs               ecsClient
	ec2               ec2Client
	cluster           string
	taskDefCache      simplelru.LRUCache
	ec2Cache          simplelru.LRUCache
	serviceNameFilter serviceNameFilter
}

type taskFetcherOptions struct {
	Logger            *zap.Logger
	Cluster           string
	Region            string
	serviceNameFilter serviceNameFilter

	// test overrides
	ecsOverride ecsClient
	ec2Override ec2Client
}

func newTaskFetcherFromConfig(cfg Config, logger *zap.Logger) (*taskFetcher, error) {
	svcNameFilter, err := serviceConfigsToFilter(cfg.Services)
	if err != nil {
		return nil, fmt.Errorf("init serivce name filter failed: %w", err)
	}
	return newTaskFetcher(taskFetcherOptions{
		Logger:            logger,
		Region:            cfg.ClusterRegion,
		Cluster:           cfg.ClusterName,
		serviceNameFilter: svcNameFilter,
	})
}

func newTaskFetcher(opts taskFetcherOptions) (*taskFetcher, error) {
	// Init cache
	taskDefCache, err := simplelru.NewLRU(taskDefCacheSize, nil)
	if err != nil {
		return nil, err
	}
	ec2Cache, err := simplelru.NewLRU(ec2CacheSize, nil)
	if err != nil {
		return nil, err
	}

	logger := opts.Logger
	fetcher := taskFetcher{
		logger:            logger,
		ecs:               opts.ecsOverride,
		ec2:               opts.ec2Override,
		cluster:           opts.Cluster,
		taskDefCache:      taskDefCache,
		ec2Cache:          ec2Cache,
		serviceNameFilter: opts.serviceNameFilter,
	}
	// Even if user didn't specify any service related config, we still generates a valid filter
	// that matches nothing. See service.go serviceConfigsToFilter.
	if fetcher.serviceNameFilter == nil {
		return nil, fmt.Errorf("serviceNameFilter can't be nil")
	}
	// Return early if any clients are mocked, caller should overrides all the clients when mocking.
	if fetcher.ecs != nil || fetcher.ec2 != nil {
		return &fetcher, nil
	}
	if opts.Cluster == "" {
		return nil, fmt.Errorf("missing ECS cluster for task fetcher")
	}
	if opts.Region == "" {
		return nil, fmt.Errorf("missing aws region for task fetcher")
	}
	logger.Debug("Init TaskFetcher", zap.String("Region", opts.Region), zap.String("Cluster", opts.Cluster))
	awsCfg := aws.NewConfig().WithRegion(opts.Region).WithCredentialsChainVerboseErrors(true)
	sess, err := session.NewSession(awsCfg)
	if err != nil {
		return nil, fmt.Errorf("create aws session failed: %w", err)
	}
	fetcher.ecs = ecs.New(sess, awsCfg)
	fetcher.ec2 = ec2.New(sess, awsCfg)
	return &fetcher, nil
}

func (f *taskFetcher) fetchAndDecorate(ctx context.Context) ([]*taskAnnotated, error) {
	// taskAnnotated
	rawTasks, err := f.getDiscoverableTasks(ctx)
	if err != nil {
		return nil, fmt.Errorf("getDiscoverableTasks failed: %w", err)
	}
	tasks, err := f.attachTaskDefinition(ctx, rawTasks)
	if err != nil {
		return nil, fmt.Errorf("attachTaskDefinition failed: %w", err)
	}

	// EC2
	if err = f.attachContainerInstance(ctx, tasks); err != nil {
		return nil, fmt.Errorf("attachContainerInstance failed: %w", err)
	}

	// Services
	services, err := f.getAllServices(ctx)
	if err != nil {
		return nil, fmt.Errorf("getAllServices failed: %w", err)
	}
	f.attachService(tasks, services)
	return tasks, nil
}

// getDiscoverableTasks get arns of all running tasks and describe those tasks
// and filter only fargate tasks or EC2 task which container instance is known.
// There is no API to list task detail without arn so we need to call two APIs.
func (f *taskFetcher) getDiscoverableTasks(ctx context.Context) ([]*ecs.Task, error) {
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

		for _, task := range descRes.Tasks {
			// Preserve only fargate tasks or EC2 tasks with non-nil ContainerInstanceArn.
			// When ECS task of EC2 launch type is in state Provisioning/Pending, it may
			// not have EC2 instance. Such tasks have `nil` instance arn and the
			// attachContainerInstance call will fail
			if task.ContainerInstanceArn != nil || aws.StringValue(task.LaunchType) != ecs.LaunchTypeEc2 {
				tasks = append(tasks, task)
			}
		}
		if listRes.NextToken == nil {
			break
		}
		req.NextToken = listRes.NextToken
	}
	return tasks, nil
}

// attachTaskDefinition converts ecs.Task into a taskAnnotated to include its ecs.TaskDefinition.
func (f *taskFetcher) attachTaskDefinition(ctx context.Context, tasks []*ecs.Task) ([]*taskAnnotated, error) {
	svc := f.ecs
	// key is task definition arn
	arn2Def := make(map[string]*ecs.TaskDefinition)
	for _, t := range tasks {
		arn2Def[aws.StringValue(t.TaskDefinitionArn)] = nil
	}

	for arn := range arn2Def {
		if arn == "" {
			continue
		}
		var def *ecs.TaskDefinition
		if cached, ok := f.taskDefCache.Get(arn); ok {
			def = cached.(*ecs.TaskDefinition)
		} else {
			res, err := svc.DescribeTaskDefinitionWithContext(ctx, &ecs.DescribeTaskDefinitionInput{
				TaskDefinition: aws.String(arn),
			})
			if err != nil {
				return nil, err
			}
			f.taskDefCache.Add(arn, res.TaskDefinition)
			def = res.TaskDefinition
		}
		arn2Def[arn] = def
	}

	var tasksWithDef []*taskAnnotated
	for _, t := range tasks {
		tasksWithDef = append(tasksWithDef, &taskAnnotated{
			Task:       t,
			Definition: arn2Def[aws.StringValue(t.TaskDefinitionArn)],
		})
	}
	return tasksWithDef, nil
}

// attachContainerInstance fetches all the container instances' underlying EC2 vms
// and attach EC2 info to tasks.
func (f *taskFetcher) attachContainerInstance(ctx context.Context, tasks []*taskAnnotated) error {
	// Map container instance to EC2, key is container instance id.
	ciToEC2 := make(map[string]*ec2.Instance)
	// Only EC2 instance type need to fetch EC2 info
	for _, t := range tasks {
		if aws.StringValue(t.Task.LaunchType) != ecs.LaunchTypeEc2 {
			continue
		}
		ciToEC2[aws.StringValue(t.Task.ContainerInstanceArn)] = nil
	}
	// All fargate, skip
	if len(ciToEC2) == 0 {
		return nil
	}

	// Describe container instances that do not have cached EC2 info.
	var instanceList []*string
	for instanceArn := range ciToEC2 {
		cached, ok := f.ec2Cache.Get(instanceArn)
		if ok {
			ciToEC2[instanceArn] = cached.(*ec2.Instance) // use value from cache
		} else {
			instanceList = append(instanceList, aws.String(instanceArn))
		}
	}
	sortStringPointers(instanceList)

	// DescribeContainerInstance size limit is 100, do it in batch.
	for i := 0; i < len(instanceList); i += describeContainerInstanceLimit {
		end := minInt(i+describeContainerInstanceLimit, len(instanceList))
		if err := f.describeContainerInstances(ctx, instanceList[i:end], ciToEC2); err != nil {
			return fmt.Errorf("describe container instanced failed offset=%d: %w", i, err)
		}
	}

	// Assign the info back to task
	for _, t := range tasks {
		// NOTE: we need to skip fargate here because we are looping all tasks again.
		if aws.StringValue(t.Task.LaunchType) != ecs.LaunchTypeEc2 {
			continue
		}
		containerInstance := aws.StringValue(t.Task.ContainerInstanceArn)
		ec2Info, ok := ciToEC2[containerInstance]
		if !ok {
			return fmt.Errorf("container instance ec2 info not found containerInstnace=%q", containerInstance)
		}
		t.EC2 = ec2Info
	}

	// Update the cache
	for ci, ec2Info := range ciToEC2 {
		f.ec2Cache.Add(ci, ec2Info)
	}
	return nil
}

// Run ecs.DescribeContainerInstances and ec2.DescribeInstances for a batch (less than 100 container instances).
func (f *taskFetcher) describeContainerInstances(ctx context.Context, instanceList []*string,
	ci2EC2 map[string]*ec2.Instance) error {
	// Get container instances
	res, err := f.ecs.DescribeContainerInstancesWithContext(ctx, &ecs.DescribeContainerInstancesInput{
		Cluster:            aws.String(f.cluster),
		ContainerInstances: instanceList,
	})
	if err != nil {
		return fmt.Errorf("ecs.DescribeContainerInstance failed: %w", err)
	}

	// Create the index to map ec2 id back to container instance id.
	var ec2Ids []*string
	ec2IdToCI := make(map[string]string)
	for _, containerInstance := range res.ContainerInstances {
		ec2Id := containerInstance.Ec2InstanceId
		ec2Ids = append(ec2Ids, ec2Id)
		ec2IdToCI[aws.StringValue(ec2Id)] = aws.StringValue(containerInstance.ContainerInstanceArn)
	}

	// Fetch all ec2 instances and update mapping from container instance id to ec2 info.
	// NOTE: because the limit on ec2 is 1000, much larger than ecs container instance's 100,
	// we don't do paging logic here.
	req := ec2.DescribeInstancesInput{InstanceIds: ec2Ids}
	ec2Res, err := f.ec2.DescribeInstancesWithContext(ctx, &req)
	if err != nil {
		return fmt.Errorf("ec2.DescribeInstances failed: %w", err)
	}
	for _, reservation := range ec2Res.Reservations {
		for _, instance := range reservation.Instances {
			if instance.InstanceId == nil {
				continue
			}
			ec2Id := aws.StringValue(instance.InstanceId)
			ci, ok := ec2IdToCI[ec2Id]
			if !ok {
				return fmt.Errorf("mapping from ec2 to container instance not found ec2=%s", ec2Id)
			}
			ci2EC2[ci] = instance // update mapping
		}
	}
	return nil
}

// serviceNameFilter decides if we should get detail info for a service, i.e. make the describe API call.
type serviceNameFilter func(name string) bool

// getAllServices does not have cache like task definition or ec2 instances
// because we need to get the deployment id to map service to task, which changes frequently.
func (f *taskFetcher) getAllServices(ctx context.Context) ([]*ecs.Service, error) {
	svc := f.ecs
	cluster := aws.String(f.cluster)
	// List and filter out services we need to desribe.
	listReq := ecs.ListServicesInput{Cluster: cluster}
	var servicesToDescribe []*string
	for {
		res, err := svc.ListServicesWithContext(ctx, &listReq)
		if err != nil {
			return nil, err
		}
		for _, arn := range res.ServiceArns {
			segs := strings.Split(aws.StringValue(arn), "/")
			name := segs[len(segs)-1]
			if f.serviceNameFilter(name) {
				servicesToDescribe = append(servicesToDescribe, arn)
			}
		}
		if res.NextToken == nil {
			break
		}
		listReq.NextToken = res.NextToken
	}

	// DescribeServices size limit is 10 so we need to do paging on client side.
	var services []*ecs.Service
	for i := 0; i < len(servicesToDescribe); i += describeServiceLimit {
		end := minInt(i+describeServiceLimit, len(servicesToDescribe))
		desc := &ecs.DescribeServicesInput{
			Cluster:  cluster,
			Services: servicesToDescribe[i:end],
		}
		res, err := svc.DescribeServicesWithContext(ctx, desc)
		if err != nil {
			return nil, fmt.Errorf("ecs.DescribeServices failed %w", err)
		}
		services = append(services, res.Services...)
	}
	return services, nil
}

// attachService map service to task using deployment id.
// Each service can have multiple deployment and each task keep track of the deployment in task.StartedBy.
func (f *taskFetcher) attachService(tasks []*taskAnnotated, services []*ecs.Service) {
	// Map deployment ID to service name
	idToService := make(map[string]*ecs.Service)
	for _, svc := range services {
		for _, deployment := range svc.Deployments {
			status := aws.StringValue(deployment.Status)
			if status == deploymentStatusActive || status == deploymentStatusPrimary {
				idToService[aws.StringValue(deployment.Id)] = svc
				break
			}
		}
	}

	// Attach service to task
	for _, t := range tasks {
		// taskAnnotated is created using RunTask i.e. not manged by a service.
		if t.Task.StartedBy == nil {
			continue
		}
		deploymentID := aws.StringValue(t.Task.StartedBy)
		svc := idToService[deploymentID]
		// Service not found happen a lot because we only fetch services defined in ServiceConfig.
		// However, we fetch all the tasks, which could be started by other services no mentioned in config
		// or started using RunTasks API directly.
		if svc == nil {
			continue
		}
		t.Service = svc
	}
}

// Util Start

func sortStringPointers(ps []*string) {
	var ss []string
	for _, p := range ps {
		ss = append(ss, aws.StringValue(p))
	}
	sort.Strings(ss)
	for i := range ss {
		ps[i] = aws.String(ss[i])
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Util End
