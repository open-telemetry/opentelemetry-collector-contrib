// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecsobserver // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecsobserver"

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/hashicorp/golang-lru/v2/simplelru"
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
	ListTasks(ctx context.Context, params *ecs.ListTasksInput, optFns ...func(*ecs.Options)) (*ecs.ListTasksOutput, error)
	DescribeTasks(ctx context.Context, params *ecs.DescribeTasksInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error)
	DescribeTaskDefinition(ctx context.Context, params *ecs.DescribeTaskDefinitionInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error)
	DescribeContainerInstances(ctx context.Context, params *ecs.DescribeContainerInstancesInput, optFns ...func(*ecs.Options)) (*ecs.DescribeContainerInstancesOutput, error)
	ListServices(ctx context.Context, params *ecs.ListServicesInput, optFns ...func(*ecs.Options)) (*ecs.ListServicesOutput, error)
	DescribeServices(ctx context.Context, params *ecs.DescribeServicesInput, optFns ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error)
}

// ec2Client includes API required by TaskFetcher.
type ec2Client interface {
	DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
}

type taskFetcher struct {
	logger            *zap.Logger
	ecs               ecsClient
	ec2               ec2Client
	cluster           string
	taskDefCache      *simplelru.LRU[string, *ecstypes.TaskDefinition]
	ec2Cache          *simplelru.LRU[string, *ec2types.Instance]
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

func newTaskFetcherFromConfig(ctx context.Context, cfg Config, logger *zap.Logger) (*taskFetcher, error) {
	svcNameFilter, err := serviceConfigsToFilter(cfg.Services)
	if err != nil {
		return nil, fmt.Errorf("init service name filter failed: %w", err)
	}
	return newTaskFetcher(ctx, taskFetcherOptions{
		Logger:            logger,
		Region:            cfg.ClusterRegion,
		Cluster:           cfg.ClusterName,
		serviceNameFilter: svcNameFilter,
	})
}

func newTaskFetcher(ctx context.Context, opts taskFetcherOptions) (*taskFetcher, error) {
	// Init cache
	taskDefCache, err := simplelru.NewLRU[string, *ecstypes.TaskDefinition](taskDefCacheSize, nil)
	if err != nil {
		return nil, err
	}
	ec2Cache, err := simplelru.NewLRU[string, *ec2types.Instance](ec2CacheSize, nil)
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
		return nil, errors.New("serviceNameFilter can't be nil")
	}
	// Return early if any clients are mocked, caller should overrides all the clients when mocking.
	if fetcher.ecs != nil || fetcher.ec2 != nil {
		return &fetcher, nil
	}
	if opts.Cluster == "" {
		return nil, errors.New("missing ECS cluster for task fetcher")
	}
	if opts.Region == "" {
		return nil, errors.New("missing aws region for task fetcher")
	}
	logger.Debug("Init TaskFetcher", zap.String("Region", opts.Region), zap.String("Cluster", opts.Cluster))
	awsCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(opts.Region))
	if err != nil {
		return nil, fmt.Errorf("failed to load aws config: %w", err)
	}

	fetcher.ecs = ecs.NewFromConfig(awsCfg)
	fetcher.ec2 = ec2.NewFromConfig(awsCfg)
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
func (f *taskFetcher) getDiscoverableTasks(ctx context.Context) ([]ecstypes.Task, error) {
	svc := f.ecs
	cluster := aws.String(f.cluster)
	req := ecs.ListTasksInput{Cluster: cluster}
	var tasks []ecstypes.Task
	for {
		listRes, err := svc.ListTasks(ctx, &req)
		if err != nil {
			return nil, fmt.Errorf("ecs.ListTasks failed: %w", err)
		}
		// NOTE: the limit for list task response and describe task request are both 100.
		descRes, err := svc.DescribeTasks(ctx, &ecs.DescribeTasksInput{
			Cluster: cluster,
			Tasks:   listRes.TaskArns,
			Include: []ecstypes.TaskField{
				ecstypes.TaskFieldTags,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("ecs.DescribeTasks failed: %w", err)
		}

		for _, task := range descRes.Tasks {
			// Preserve only fargate tasks or EC2 tasks with non-nil ContainerInstanceArn.
			// When ECS task of EC2 launch type is in state Provisioning/Pending, it may
			// not have EC2 instance. Such tasks have `nil` instance arn and the
			// attachContainerInstance call will fail
			if task.ContainerInstanceArn != nil || task.LaunchType != ecstypes.LaunchTypeEc2 {
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
func (f *taskFetcher) attachTaskDefinition(ctx context.Context, tasks []ecstypes.Task) ([]*taskAnnotated, error) {
	svc := f.ecs
	// key is task definition arn
	arn2Def := make(map[string]*ecstypes.TaskDefinition)
	for _, t := range tasks {
		if t.TaskDefinitionArn != nil {
			arn2Def[*t.TaskDefinitionArn] = nil
		}
	}

	for arn := range arn2Def {
		if arn == "" {
			continue
		}
		var def *ecstypes.TaskDefinition
		if cached, ok := f.taskDefCache.Get(arn); ok {
			def = cached
		} else {
			res, err := svc.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
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

	tasksWithDef := make([]*taskAnnotated, len(tasks))
	for i, t := range tasks {
		if t.TaskDefinitionArn != nil {
			tasksWithDef[i] = &taskAnnotated{
				Task:       t,
				Definition: arn2Def[*t.TaskDefinitionArn],
			}
		}
	}
	return tasksWithDef, nil
}

// attachContainerInstance fetches all the container instances' underlying EC2 vms
// and attach EC2 info to tasks.
func (f *taskFetcher) attachContainerInstance(ctx context.Context, tasks []*taskAnnotated) error {
	// Map container instance to EC2, key is container instance id.
	ciToEC2 := make(map[string]*ec2types.Instance)
	// Only EC2 instance type need to fetch EC2 info
	for _, t := range tasks {
		if t.Task.LaunchType != ecstypes.LaunchTypeEc2 {
			continue
		}
		ciToEC2[*t.Task.ContainerInstanceArn] = nil
	}
	// All fargate, skip
	if len(ciToEC2) == 0 {
		return nil
	}

	// Describe container instances that do not have cached EC2 info.
	var instanceList []string
	for instanceArn := range ciToEC2 {
		cached, ok := f.ec2Cache.Get(instanceArn)
		if ok {
			ciToEC2[instanceArn] = cached // use value from cache
		} else {
			instanceList = append(instanceList, instanceArn)
		}
	}
	slices.Sort(instanceList)

	// DescribeContainerInstance size limit is 100, do it in batch.
	for i := 0; i < len(instanceList); i += describeContainerInstanceLimit {
		end := min(i+describeContainerInstanceLimit, len(instanceList))
		if err := f.describeContainerInstances(ctx, instanceList[i:end], ciToEC2); err != nil {
			return fmt.Errorf("describe container instanced failed offset=%d: %w", i, err)
		}
	}

	// Assign the info back to task
	for _, t := range tasks {
		// NOTE: we need to skip fargate here because we are looping all tasks again.
		if t.Task.LaunchType != ecstypes.LaunchTypeEc2 {
			continue
		}
		containerInstance := *t.Task.ContainerInstanceArn
		ec2Info, ok := ciToEC2[containerInstance]
		if !ok {
			return fmt.Errorf("container instance ec2 info not found containerInstance=%q", containerInstance)
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
func (f *taskFetcher) describeContainerInstances(ctx context.Context, instanceList []string,
	ci2EC2 map[string]*ec2types.Instance,
) error {
	// Get container instances
	res, err := f.ecs.DescribeContainerInstances(ctx, &ecs.DescribeContainerInstancesInput{
		Cluster:            aws.String(f.cluster),
		ContainerInstances: instanceList,
	})
	if err != nil {
		return fmt.Errorf("ecs.DescribeContainerInstance failed: %w", err)
	}

	// Create the index to map ec2 id back to container instance id.
	ec2Ids := make([]string, len(res.ContainerInstances))
	ec2IdToCI := make(map[string]string)
	for i, containerInstance := range res.ContainerInstances {
		ec2Id := containerInstance.Ec2InstanceId
		ec2Ids[i] = *ec2Id
		ec2IdToCI[*ec2Id] = *containerInstance.ContainerInstanceArn
	}

	// Fetch all ec2 instances and update mapping from container instance id to ec2 info.
	// NOTE: because the limit on ec2 is 1000, much larger than ecs container instance's 100,
	// we don't do paging logic here.
	ec2Res, err := f.ec2.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: ec2Ids,
	})
	if err != nil {
		return fmt.Errorf("ec2.DescribeInstances failed: %w", err)
	}
	for _, reservation := range ec2Res.Reservations {
		for _, instance := range reservation.Instances {
			if instance.InstanceId == nil {
				continue
			}
			ec2Id := *instance.InstanceId
			ci, ok := ec2IdToCI[ec2Id]
			if !ok {
				return fmt.Errorf("mapping from ec2 to container instance not found ec2=%s", ec2Id)
			}
			ci2EC2[ci] = &instance // update mapping
		}
	}
	return nil
}

// serviceNameFilter decides if we should get detail info for a service, i.e. make the describe API call.
type serviceNameFilter func(name string) bool

// getAllServices does not have cache like task definition or ec2 instances
// because we need to get the deployment id to map service to task, which changes frequently.
func (f *taskFetcher) getAllServices(ctx context.Context) ([]ecstypes.Service, error) {
	svc := f.ecs
	cluster := aws.String(f.cluster)
	// List and filter out services we need to describe.
	listReq := ecs.ListServicesInput{Cluster: cluster}
	var servicesToDescribe []string
	for {
		res, err := svc.ListServices(ctx, &listReq)
		if err != nil {
			return nil, err
		}
		for _, arn := range res.ServiceArns {
			segs := strings.Split(arn, "/")
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
	var services []ecstypes.Service
	for i := 0; i < len(servicesToDescribe); i += describeServiceLimit {
		end := min(i+describeServiceLimit, len(servicesToDescribe))
		res, err := svc.DescribeServices(ctx, &ecs.DescribeServicesInput{
			Cluster:  cluster,
			Services: servicesToDescribe[i:end],
		})
		if err != nil {
			return nil, fmt.Errorf("ecs.DescribeServices failed %w", err)
		}
		services = append(services, res.Services...)
	}
	return services, nil
}

// attachService map service to task using deployment id.
// Each service can have multiple deployment and each task keep track of the deployment in task.StartedBy.
func (f *taskFetcher) attachService(tasks []*taskAnnotated, services []ecstypes.Service) {
	// Map deployment ID to service name
	idToService := make(map[string]*ecstypes.Service)
	for _, svc := range services {
		for _, deployment := range svc.Deployments {
			status := *deployment.Status
			if status == deploymentStatusActive || status == deploymentStatusPrimary {
				idToService[*deployment.Id] = &svc
				break
			}
		}
	}

	// Attach service to task
	for _, t := range tasks {
		// taskAnnotated is created using RunTask i.e. not managed by a service.
		if t.Task.StartedBy == nil {
			continue
		}
		deploymentID := *t.Task.StartedBy
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
