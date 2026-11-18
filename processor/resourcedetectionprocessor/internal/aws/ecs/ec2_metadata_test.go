// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build (!remove_all_resourcedetection_detectors || enable_detector_aws_ecs) && !omit_detector_aws_ecs

package ecs

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"syscall"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/smithy-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil/endpoints"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/ecs/internal/metadata"
)

const (
	enrichmentTask              = "arn:aws:ecs:us-west-2:123456789123:task/my-cluster/123"
	enrichmentOtherTask         = "arn:aws:ecs:us-west-2:123456789123:task/my-cluster/789"
	enrichmentCluster           = "arn:aws:ecs:us-west-2:123456789123:cluster/my-cluster"
	enrichmentContainerInstance = "arn:aws:ecs:us-west-2:123456789123:container-instance/my-cluster/456"
	enrichmentInstance          = "i-0123456789abcdef0"
	enrichmentOtherInstance     = "i-0fedcba9876543210"
)

var hostAttributeKeys = []string{"host.id", "host.name", "host.type", "host.image.id"}

type enrichmentMetadataProvider struct {
	task ecsutil.TaskMetadata
}

func (p *enrichmentMetadataProvider) FetchTaskMetadata() (*ecsutil.TaskMetadata, error) {
	return &p.task, nil
}

func (*enrichmentMetadataProvider) FetchContainerMetadata() (*ecsutil.ContainerMetadata, error) {
	return &ecsutil.ContainerMetadata{}, nil
}

type enrichmentAPI struct {
	t                                  *testing.T
	ctx                                context.Context
	taskARN, containerARN, instanceID  string
	tasks                              *ecs.DescribeTasksOutput
	containers                         *ecs.DescribeContainerInstancesOutput
	instances                          *ec2.DescribeInstancesOutput
	taskErr, containerErr, instanceErr error
	calls                              []string
	wait                               bool
}

func (a *enrichmentAPI) DescribeTasks(ctx context.Context, in *ecs.DescribeTasksInput, _ ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error) {
	assert.Equal(a.t, a.ctx, ctx)
	assert.Equal(a.t, []string{a.taskARN}, in.Tasks)
	assert.Contains(a.t, []string{"my-cluster", enrichmentCluster}, aws.ToString(in.Cluster))
	a.calls = append(a.calls, "tasks")
	if a.wait {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return a.tasks, a.taskErr
}

func (a *enrichmentAPI) DescribeContainerInstances(ctx context.Context, in *ecs.DescribeContainerInstancesInput, _ ...func(*ecs.Options)) (*ecs.DescribeContainerInstancesOutput, error) {
	assert.Equal(a.t, a.ctx, ctx)
	assert.Equal(a.t, []string{a.containerARN}, in.ContainerInstances)
	assert.Contains(a.t, []string{"my-cluster", enrichmentCluster}, aws.ToString(in.Cluster))
	a.calls = append(a.calls, "containers")
	return a.containers, a.containerErr
}

func (a *enrichmentAPI) DescribeInstances(ctx context.Context, in *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	assert.Equal(a.t, a.ctx, ctx)
	assert.Equal(a.t, []string{a.instanceID}, in.InstanceIds)
	a.calls = append(a.calls, "instances")
	return a.instances, a.instanceErr
}

func (a *enrichmentAPI) instance() *ec2types.Instance {
	return &a.instances.Reservations[0].Instances[0]
}

// setTask makes both the metadata endpoint and the API fixtures describe taskARN
// running on the EC2 instance instanceID.
func setTask(p *enrichmentMetadataProvider, a *enrichmentAPI, taskARN, instanceID string) {
	p.task.TaskARN = taskARN
	a.taskARN = taskARN
	a.tasks.Tasks[0].TaskArn = aws.String(taskARN)
	a.containers.ContainerInstances[0].Ec2InstanceId = aws.String(instanceID)
	a.instanceID = instanceID
	a.instance().InstanceId = aws.String(instanceID)
}

// hostAttributes enables the host attributes only when keys lists them.
func hostAttributes(keys ...string) metadata.ResourceAttributesConfig {
	cfg := metadata.DefaultResourceAttributesConfig()
	cfg.HostID.Enabled = false
	cfg.HostName.Enabled = false
	cfg.HostType.Enabled = false
	cfg.HostImageID.Enabled = false
	for _, key := range keys {
		switch key {
		case "host.id":
			cfg.HostID.Enabled = true
		case "host.name":
			cfg.HostName.Enabled = true
		case "host.type":
			cfg.HostType.Enabled = true
		case "host.image.id":
			cfg.HostImageID.Enabled = true
		}
	}
	return cfg
}

func newEnrichmentDetector(t *testing.T) (*Detector, *enrichmentAPI, *enrichmentMetadataProvider, *observer.ObservedLogs) {
	p := &enrichmentMetadataProvider{task: ecsutil.TaskMetadata{Cluster: "my-cluster", TaskARN: enrichmentTask, LaunchType: "EC2", AvailabilityZone: "task-zone"}}
	a := &enrichmentAPI{
		t: t, ctx: t.Context(), taskARN: enrichmentTask, containerARN: enrichmentContainerInstance, instanceID: enrichmentInstance,
		tasks:      &ecs.DescribeTasksOutput{Tasks: []ecstypes.Task{{TaskArn: aws.String(enrichmentTask), ClusterArn: aws.String(enrichmentCluster), ContainerInstanceArn: aws.String(enrichmentContainerInstance)}}},
		containers: &ecs.DescribeContainerInstancesOutput{ContainerInstances: []ecstypes.ContainerInstance{{ContainerInstanceArn: aws.String(enrichmentContainerInstance), Ec2InstanceId: aws.String(enrichmentInstance)}}},
		instances:  &ec2.DescribeInstancesOutput{Reservations: []ec2types.Reservation{{OwnerId: aws.String("123456789123"), Instances: []ec2types.Instance{{InstanceId: aws.String(enrichmentInstance), PrivateDnsName: aws.String("ip-10-0-0-1.us-west-2.compute.internal"), InstanceType: ec2types.InstanceTypeT3Micro, ImageId: aws.String("ami-123"), Placement: &ec2types.Placement{AvailabilityZone: aws.String("us-west-2a")}}}}}},
	}
	core, logs := observer.New(zap.WarnLevel)
	d := &Detector{provider: p, rb: metadata.NewResourceBuilder(hostAttributes(hostAttributeKeys...)), hostLookup: true, logger: zap.New(core), clients: func(ctx context.Context, region string) (ecsAPI, ec2API, error) {
		assert.Equal(t, a.ctx, ctx)
		assert.Equal(t, "us-west-2", region)
		return a, a, nil
	}}
	return d, a, p, logs
}

func assertNoHostAttributes(t *testing.T, attrs map[string]any) {
	t.Helper()
	for _, key := range hostAttributeKeys {
		assert.NotContains(t, attrs, key)
	}
}

func TestEC2MetadataDefaultsDoNotLookUp(t *testing.T) {
	cfg := CreateDefaultConfig()
	for _, enabled := range []bool{cfg.ResourceAttributes.HostID.Enabled, cfg.ResourceAttributes.HostName.Enabled, cfg.ResourceAttributes.HostType.Enabled, cfg.ResourceAttributes.HostImageID.Enabled} {
		assert.False(t, enabled)
	}

	t.Setenv(endpoints.TaskMetadataEndpointV4EnvVar, "http://localhost/task")
	detector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, false)
	require.NoError(t, err)
	d := detector.(*Detector)
	assert.False(t, d.hostLookup)

	d, a, _, logs := newEnrichmentDetector(t)
	d.hostLookup = false
	d.rb = metadata.NewResourceBuilder(metadata.DefaultResourceAttributesConfig())
	d.clients = func(context.Context, string) (ecsAPI, ec2API, error) {
		t.Fatal("unexpected AWS clients")
		return nil, nil, nil
	}
	resource, _, err := d.Detect(t.Context())
	require.NoError(t, err)
	attrs := resource.Attributes().AsRaw()
	assertNoHostAttributes(t, attrs)
	assert.Equal(t, "task-zone", attrs["cloud.availability_zone"])
	assert.Empty(t, a.calls)
	assert.Zero(t, logs.Len())
}

func TestEC2MetadataEachHostAttributeTriggersLookup(t *testing.T) {
	want := map[string]string{"host.id": enrichmentInstance, "host.name": "ip-10-0-0-1.us-west-2.compute.internal", "host.type": "t3.micro", "host.image.id": "ami-123"}
	for _, key := range hostAttributeKeys {
		t.Run(key, func(t *testing.T) {
			t.Setenv(endpoints.TaskMetadataEndpointV4EnvVar, "http://localhost/task")
			cfg := CreateDefaultConfig()
			cfg.ResourceAttributes = hostAttributes(key)
			detector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, false)
			require.NoError(t, err)
			created := detector.(*Detector)
			require.True(t, created.hostLookup)

			d, a, _, logs := newEnrichmentDetector(t)
			d.rb = created.rb
			resource, _, err := d.Detect(t.Context())
			require.NoError(t, err)
			attrs := resource.Attributes().AsRaw()
			for _, other := range hostAttributeKeys {
				if other == key {
					assert.Equal(t, want[other], attrs[other], other)
				} else {
					assert.NotContains(t, attrs, other)
				}
			}
			assert.Equal(t, []string{"tasks", "containers", "instances"}, a.calls)
			assert.Zero(t, logs.Len())
		})
	}
}

func TestEC2MetadataSuccess(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(map[bool]string{false: "qualified", true: "legacy"}[legacy], func(t *testing.T) {
			d, a, p, logs := newEnrichmentDetector(t)
			if legacy {
				p.task.TaskARN = "arn:aws:ecs:us-west-2:123456789123:task/123"
				p.task.Cluster = enrichmentCluster
				a.taskARN = p.task.TaskARN
				a.tasks.Tasks[0].TaskArn = aws.String(a.taskARN)
				a.containerARN = "arn:aws:ecs:us-west-2:123456789123:container-instance/456"
				a.tasks.Tasks[0].ContainerInstanceArn = aws.String(a.containerARN)
				a.containers.ContainerInstances[0].ContainerInstanceArn = aws.String(a.containerARN)
			}
			resource, _, err := d.Detect(t.Context())
			require.NoError(t, err)
			attrs := resource.Attributes().AsRaw()
			for key, want := range map[string]string{"host.id": enrichmentInstance, "host.name": "ip-10-0-0-1.us-west-2.compute.internal", "host.type": "t3.micro", "host.image.id": "ami-123", "cloud.account.id": "123456789123", "cloud.region": "us-west-2", "cloud.availability_zone": "us-west-2a", "cloud.platform": "aws_ecs"} {
				assert.Equal(t, want, attrs[key], key)
			}
			assert.Equal(t, []string{"tasks", "containers", "instances"}, a.calls)
			assert.Zero(t, logs.Len())
		})
	}
}

func TestEC2MetadataSkippedForOtherLaunchTypes(t *testing.T) {
	for _, launch := range []string{"FARGATE", "", "other"} {
		t.Run("launch_"+launch, func(t *testing.T) {
			d, a, p, logs := newEnrichmentDetector(t)
			p.task.LaunchType = launch
			d.clients = func(context.Context, string) (ecsAPI, ec2API, error) {
				t.Fatal("unexpected AWS clients")
				return nil, nil, nil
			}
			resource, _, err := d.Detect(t.Context())
			require.NoError(t, err)
			assertNoHostAttributes(t, resource.Attributes().AsRaw())
			assert.Empty(t, a.calls)
			assert.Zero(t, logs.Len())
		})
	}
}

// enrichmentFailures lists the ways the identity chain or the AWS APIs can fail.
// calls is the number of AWS API calls made before the failure.
func enrichmentFailures() []struct {
	name   string
	change func(*Detector, *enrichmentAPI, *enrichmentMetadataProvider)
	calls  int
} {
	denial := &smithy.GenericAPIError{Code: "AccessDeniedException", Message: "secret response"}
	return []struct {
		name   string
		change func(*Detector, *enrichmentAPI, *enrichmentMetadataProvider)
		calls  int
	}{
		{"malformed task", func(_ *Detector, _ *enrichmentAPI, p *enrichmentMetadataProvider) { p.task.TaskARN = "broken" }, 0},
		{"task without region", func(_ *Detector, _ *enrichmentAPI, p *enrichmentMetadataProvider) {
			p.task.TaskARN = "arn:aws:ecs::123456789123:task/my-cluster/123"
		}, 0},
		{"missing cluster", func(_ *Detector, _ *enrichmentAPI, p *enrichmentMetadataProvider) { p.task.Cluster = "" }, 0},
		{"configuration", func(d *Detector, _ *enrichmentAPI, _ *enrichmentMetadataProvider) {
			d.clients = func(context.Context, string) (ecsAPI, ec2API, error) { return nil, nil, errors.New("secret config") }
		}, 0},
		{"task IAM", func(_ *Detector, a *enrichmentAPI, _ *enrichmentMetadataProvider) { a.taskErr = denial }, 1},
		{"nil tasks", func(_ *Detector, a *enrichmentAPI, _ *enrichmentMetadataProvider) { a.tasks = nil }, 1},
		{"task failures", func(_ *Detector, a *enrichmentAPI, _ *enrichmentMetadataProvider) {
			a.tasks.Failures = []ecstypes.Failure{{Reason: aws.String("MISSING")}}
		}, 1},
		{"missing tasks", func(_ *Detector, a *enrichmentAPI, _ *enrichmentMetadataProvider) { a.tasks.Tasks = nil }, 1},
		{"ambiguous tasks", func(_ *Detector, a *enrichmentAPI, _ *enrichmentMetadataProvider) {
			a.tasks.Tasks = append(a.tasks.Tasks, a.tasks.Tasks[0])
		}, 1},
		{"wrong task", func(_ *Detector, a *enrichmentAPI, _ *enrichmentMetadataProvider) {
			a.tasks.Tasks[0].TaskArn = aws.String("wrong")
		}, 1},
		{"missing container ARN", func(_ *Detector, a *enrichmentAPI, _ *enrichmentMetadataProvider) {
			a.tasks.Tasks[0].ContainerInstanceArn = nil
		}, 1},
		{"container IAM", func(_ *Detector, a *enrichmentAPI, _ *enrichmentMetadataProvider) { a.containerErr = denial }, 2},
		{"nil containers", func(_ *Detector, a *enrichmentAPI, _ *enrichmentMetadataProvider) { a.containers = nil }, 2},
		{"container failures", func(_ *Detector, a *enrichmentAPI, _ *enrichmentMetadataProvider) {
			a.containers.Failures = []ecstypes.Failure{{Reason: aws.String("MISSING")}}
		}, 2},
		{"ambiguous containers", func(_ *Detector, a *enrichmentAPI, _ *enrichmentMetadataProvider) {
			a.containers.ContainerInstances = append(a.containers.ContainerInstances, a.containers.ContainerInstances[0])
		}, 2},
		{"wrong container", func(_ *Detector, a *enrichmentAPI, _ *enrichmentMetadataProvider) {
			a.containers.ContainerInstances[0].ContainerInstanceArn = aws.String("wrong")
		}, 2},
		{"missing instance ID", func(_ *Detector, a *enrichmentAPI, _ *enrichmentMetadataProvider) {
			a.containers.ContainerInstances[0].Ec2InstanceId = nil
		}, 2},
		{"instance IAM", func(_ *Detector, a *enrichmentAPI, _ *enrichmentMetadataProvider) { a.instanceErr = denial }, 3},
		{"nil instances", func(_ *Detector, a *enrichmentAPI, _ *enrichmentMetadataProvider) { a.instances = nil }, 3},
		{"no reservations", func(_ *Detector, a *enrichmentAPI, _ *enrichmentMetadataProvider) { a.instances.Reservations = nil }, 3},
		{"ambiguous reservations", func(_ *Detector, a *enrichmentAPI, _ *enrichmentMetadataProvider) {
			a.instances.Reservations = append(a.instances.Reservations, a.instances.Reservations[0])
		}, 3},
		{"no instances", func(_ *Detector, a *enrichmentAPI, _ *enrichmentMetadataProvider) {
			a.instances.Reservations[0].Instances = nil
		}, 3},
		{"ambiguous instances", func(_ *Detector, a *enrichmentAPI, _ *enrichmentMetadataProvider) {
			a.instances.Reservations[0].Instances = append(a.instances.Reservations[0].Instances, a.instances.Reservations[0].Instances[0])
		}, 3},
		{"wrong instance", func(_ *Detector, a *enrichmentAPI, _ *enrichmentMetadataProvider) {
			a.instance().InstanceId = aws.String("i-other")
		}, 3},
	}
}

func TestEC2MetadataSoftFailuresPreserveECS(t *testing.T) {
	for _, tt := range enrichmentFailures() {
		t.Run(tt.name, func(t *testing.T) {
			d, a, p, logs := newEnrichmentDetector(t)
			tt.change(d, a, p)
			resource, _, err := d.Detect(t.Context())
			require.NoError(t, err)
			attrs := resource.Attributes().AsRaw()
			assert.Equal(t, p.task.TaskARN, attrs["aws.ecs.task.arn"])
			assert.Equal(t, "aws_ecs", attrs["cloud.platform"])
			assert.Equal(t, "task-zone", attrs["cloud.availability_zone"])
			assertNoHostAttributes(t, attrs)
			assert.Len(t, a.calls, tt.calls)
			require.Equal(t, 1, logs.Len())
			assert.NotEmpty(t, logs.All()[0].ContextMap()["error"])
		})
	}
}

func TestEC2MetadataStrictFailuresReturnError(t *testing.T) {
	for _, tt := range enrichmentFailures() {
		t.Run(tt.name, func(t *testing.T) {
			d, a, p, logs := newEnrichmentDetector(t)
			d.failOnMissingMetadata = true
			tt.change(d, a, p)
			resource, _, err := d.Detect(t.Context())
			require.ErrorContains(t, err, "unable to enrich ECS resource with EC2 metadata")
			assert.Zero(t, resource.Attributes().Len())
			assert.Len(t, a.calls, tt.calls)
			assert.Zero(t, logs.Len())
		})
	}
}

func TestEC2MetadataStrictRetriesAfterFailure(t *testing.T) {
	d, a, _, _ := newEnrichmentDetector(t)
	d.failOnMissingMetadata = true

	// A canceled lookup is an error, so the caller retries it.
	ctx, cancel := context.WithCancel(t.Context())
	a.ctx = ctx
	a.wait = true
	cancel()
	resource, _, err := d.Detect(ctx)
	require.ErrorIs(t, err, context.Canceled)
	assert.Zero(t, resource.Attributes().Len())

	// An API error is an error too.
	a.ctx, a.wait = t.Context(), false
	a.taskErr = &smithy.GenericAPIError{Code: "ThrottlingException", Message: "secret response"}
	_, _, err = d.Detect(t.Context())
	require.ErrorContains(t, err, "DescribeTasks: api error ThrottlingException: secret response")

	// The next attempt succeeds and is not poisoned by the earlier failures.
	a.taskErr = nil
	resource, _, err = d.Detect(t.Context())
	require.NoError(t, err)
	attrs := resource.Attributes().AsRaw()
	assert.Equal(t, enrichmentInstance, attrs["host.id"])
	assert.Equal(t, "us-west-2a", attrs["cloud.availability_zone"])
}

func TestEC2MetadataContext(t *testing.T) {
	d, a, _, logs := newEnrichmentDetector(t)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	a.ctx = ctx
	a.wait = true
	started := time.Now()
	resource, _, err := d.Detect(ctx)
	require.NoError(t, err)
	assert.Less(t, time.Since(started), time.Second)
	assertNoHostAttributes(t, resource.Attributes().AsRaw())
	require.Equal(t, 1, logs.Len())
	assert.Contains(t, logs.All()[0].ContextMap()["error"], "context deadline exceeded")
}

func TestEC2MetadataSuccessfulHostIsCachedForSameTask(t *testing.T) {
	for _, strict := range []bool{false, true} {
		t.Run(map[bool]string{false: "soft", true: "strict"}[strict], func(t *testing.T) {
			d, a, _, logs := newEnrichmentDetector(t)
			d.failOnMissingMetadata = strict

			// A failed lookup is not cached and is retried.
			a.taskErr = &smithy.GenericAPIError{Code: "ThrottlingException", Message: "throttled"}
			resource, _, err := d.Detect(t.Context())
			if strict {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assertNoHostAttributes(t, resource.Attributes().AsRaw())
				require.Equal(t, 1, logs.Len())
			}
			assert.Equal(t, []string{"tasks"}, a.calls)
			assert.Nil(t, d.lastHost)

			// The retry succeeds and populates the cache.
			a.taskErr = nil
			a.calls = nil
			resource, _, err = d.Detect(t.Context())
			require.NoError(t, err)
			assert.Equal(t, enrichmentInstance, resource.Attributes().AsRaw()["host.id"])
			assert.Equal(t, []string{"tasks", "containers", "instances"}, a.calls)

			// Later detections for the same task make no API calls, even if the
			// APIs would now fail or report other values.
			a.calls = nil
			a.taskErr = &smithy.GenericAPIError{Code: "ThrottlingException", Message: "throttled"}
			a.instance().InstanceType = ec2types.InstanceTypeM5Large
			logs.TakeAll()
			for range 3 {
				resource, _, err = d.Detect(t.Context())
				require.NoError(t, err)
				attrs := resource.Attributes().AsRaw()
				assert.Equal(t, enrichmentInstance, attrs["host.id"])
				assert.Equal(t, "ip-10-0-0-1.us-west-2.compute.internal", attrs["host.name"])
				assert.Equal(t, "t3.micro", attrs["host.type"])
				assert.Equal(t, "ami-123", attrs["host.image.id"])
				assert.Equal(t, "us-west-2a", attrs["cloud.availability_zone"])
				assert.Equal(t, enrichmentTask, attrs["aws.ecs.task.arn"])
			}
			assert.Empty(t, a.calls)
			assert.Zero(t, logs.Len())
		})
	}
}

func TestEC2MetadataChangedTaskDoesNotReuseHost(t *testing.T) {
	d, a, p, _ := newEnrichmentDetector(t)
	resource, _, err := d.Detect(t.Context())
	require.NoError(t, err)
	assert.Equal(t, enrichmentInstance, resource.Attributes().AsRaw()["host.id"])

	// A different task that cannot be looked up must not inherit the previous task's host.
	setTask(p, a, enrichmentOtherTask, enrichmentOtherInstance)
	a.taskErr = &smithy.GenericAPIError{Code: "ThrottlingException", Message: "secret response"}
	resource, _, err = d.Detect(t.Context())
	require.NoError(t, err)
	attrs := resource.Attributes().AsRaw()
	assertNoHostAttributes(t, attrs)
	assert.Equal(t, enrichmentOtherTask, attrs["aws.ecs.task.arn"])

	// Going back to the original task does not resurrect its old host either.
	setTask(p, a, enrichmentTask, enrichmentInstance)
	resource, _, err = d.Detect(t.Context())
	require.NoError(t, err)
	assertNoHostAttributes(t, resource.Attributes().AsRaw())

	// A successful lookup for the changed task is used and cached for that task.
	setTask(p, a, enrichmentOtherTask, enrichmentOtherInstance)
	a.taskErr = nil
	resource, _, err = d.Detect(t.Context())
	require.NoError(t, err)
	assert.Equal(t, enrichmentOtherInstance, resource.Attributes().AsRaw()["host.id"])
	a.taskErr = &smithy.GenericAPIError{Code: "ThrottlingException"}
	a.calls = nil
	resource, _, err = d.Detect(t.Context())
	require.NoError(t, err)
	assert.Equal(t, enrichmentOtherInstance, resource.Attributes().AsRaw()["host.id"])
	assert.Empty(t, a.calls)
}

func TestEC2MetadataStrictChangedTaskDoesNotReuseHost(t *testing.T) {
	d, a, p, _ := newEnrichmentDetector(t)
	d.failOnMissingMetadata = true
	_, _, err := d.Detect(t.Context())
	require.NoError(t, err)

	setTask(p, a, enrichmentOtherTask, enrichmentOtherInstance)
	a.taskErr = &smithy.GenericAPIError{Code: "ThrottlingException"}
	a.calls = nil
	resource, _, err := d.Detect(t.Context())
	require.Error(t, err)
	assert.Zero(t, resource.Attributes().Len())
	assert.Equal(t, []string{"tasks"}, a.calls)
	assert.Nil(t, d.lastHost)
}

func TestEC2MetadataOptionalFieldsDoNotBlockOthers(t *testing.T) {
	for _, tt := range []struct {
		name    string
		change  func(*ec2types.Instance)
		missing []string
		zone    string
	}{
		{"missing DNS", func(i *ec2types.Instance) { i.PrivateDnsName = nil }, []string{"host.name"}, "us-west-2a"},
		{"blank DNS", func(i *ec2types.Instance) { i.PrivateDnsName = aws.String(" ") }, []string{"host.name"}, "us-west-2a"},
		{"missing type", func(i *ec2types.Instance) { i.InstanceType = "" }, []string{"host.type"}, "us-west-2a"},
		{"missing image", func(i *ec2types.Instance) { i.ImageId = nil }, []string{"host.image.id"}, "us-west-2a"},
		{"missing placement", func(i *ec2types.Instance) { i.Placement = nil }, nil, "task-zone"},
		{"missing zone", func(i *ec2types.Instance) { i.Placement.AvailabilityZone = nil }, nil, "task-zone"},
		{"only ID", func(i *ec2types.Instance) { *i = ec2types.Instance{InstanceId: i.InstanceId} }, []string{"host.name", "host.type", "host.image.id"}, "task-zone"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d, a, _, logs := newEnrichmentDetector(t)
			tt.change(a.instance())
			resource, _, err := d.Detect(t.Context())
			require.NoError(t, err)
			attrs := resource.Attributes().AsRaw()
			assert.Equal(t, enrichmentInstance, attrs["host.id"])
			for _, key := range hostAttributeKeys {
				if key == "host.id" {
					continue
				}
				if slices.Contains(tt.missing, key) {
					assert.NotContains(t, attrs, key)
				} else {
					assert.NotEmpty(t, attrs[key], key)
				}
			}
			assert.Equal(t, tt.zone, attrs["cloud.availability_zone"])
			assert.Equal(t, "123456789123", attrs["cloud.account.id"])
			assert.Equal(t, "us-west-2", attrs["cloud.region"])
			assert.Zero(t, logs.Len())

			// Strict mode treats the missing optional fields as a success too.
			d, a, _, _ = newEnrichmentDetector(t)
			d.failOnMissingMetadata = true
			tt.change(a.instance())
			_, _, err = d.Detect(t.Context())
			require.NoError(t, err)
		})
	}
}

func TestEC2MetadataAttributeSelection(t *testing.T) {
	for _, key := range append([]string{"cloud.availability_zone"}, hostAttributeKeys...) {
		t.Run("disable "+key, func(t *testing.T) {
			d, _, _, _ := newEnrichmentDetector(t)
			cfg := hostAttributes(hostAttributeKeys...)
			switch key {
			case "host.id":
				cfg.HostID.Enabled = false
			case "host.name":
				cfg.HostName.Enabled = false
			case "host.image.id":
				cfg.HostImageID.Enabled = false
			case "host.type":
				cfg.HostType.Enabled = false
			case "cloud.availability_zone":
				cfg.CloudAvailabilityZone.Enabled = false
			}
			d.rb = metadata.NewResourceBuilder(cfg)
			resource, _, err := d.Detect(t.Context())
			require.NoError(t, err)
			attrs := resource.Attributes().AsRaw()
			assert.NotContains(t, attrs, key)
			for _, other := range append([]string{"cloud.availability_zone"}, hostAttributeKeys...) {
				if other != key {
					assert.Contains(t, attrs, other)
				}
			}
			assert.Equal(t, "aws_ecs", attrs["cloud.platform"])
		})
	}

	t.Run("zone alone does not look up", func(t *testing.T) {
		t.Setenv(endpoints.TaskMetadataEndpointV4EnvVar, "http://localhost/task")
		cfg := CreateDefaultConfig()
		cfg.ResourceAttributes = hostAttributes()
		cfg.ResourceAttributes.CloudAvailabilityZone.Enabled = true
		detector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, false)
		require.NoError(t, err)
		assert.False(t, detector.(*Detector).hostLookup)
	})
}

func TestEC2MetadataNewDetector(t *testing.T) {
	t.Setenv(endpoints.TaskMetadataEndpointV4EnvVar, "http://localhost/task")
	cfg := CreateDefaultConfig()
	cfg.ResourceAttributes = hostAttributes("host.name")
	detector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, true)
	require.NoError(t, err)
	d := detector.(*Detector)
	assert.True(t, d.hostLookup)
	assert.True(t, d.failOnMissingMetadata)
	assert.NotNil(t, d.logger)
	assert.NotNil(t, d.clients)
	assert.NotNil(t, d.rb)
}

func TestEC2MetadataContainerFailure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "preserve", true: "fail"}[fail], func(t *testing.T) {
			d, a, _, logs := newEnrichmentDetector(t)
			d.provider = &mockMetaDataProvider{isV4: true, taskArnVersion: 2, containerErr: errors.New("container failure")}
			d.failOnMissingMetadata = fail
			resource, _, err := d.Detect(t.Context())
			if fail {
				require.ErrorContains(t, err, "ecs container metadata unavailable")
				assert.Zero(t, resource.Attributes().Len())
			} else {
				require.NoError(t, err)
				assert.Equal(t, enrichmentInstance, resource.Attributes().AsRaw()["host.id"])
			}
			assert.Len(t, a.calls, 3)
			assert.Zero(t, logs.Len())
		})
	}
}

func TestEC2MetadataCredentialsDoNotUseIMDS(t *testing.T) {
	// A missing task role must fail promptly without probing the instance role.
	for _, key := range []string{"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN", "AWS_PROFILE", "AWS_DEFAULT_PROFILE", "AWS_CONTAINER_CREDENTIALS_RELATIVE_URI", "AWS_CONTAINER_CREDENTIALS_FULL_URI", "AWS_WEB_IDENTITY_TOKEN_FILE", "AWS_ROLE_ARN", "AWS_EC2_METADATA_DISABLED"} {
		t.Setenv(key, "")
	}
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(t.TempDir(), "missing"))
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(t.TempDir(), "missing"))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("unexpected IMDS request")
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()
	t.Setenv("AWS_EC2_METADATA_SERVICE_ENDPOINT", server.URL)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	ecsc, _, err := newEC2MetadataClients(ctx, "us-west-2")
	require.NoError(t, err)
	client := ecsc.(*ecs.Client)
	_, err = client.Options().Credentials.Retrieve(ctx)
	require.Error(t, err)
	assert.NoError(t, ctx.Err())
}

func TestEC2MetadataTaskRoleCredentials(t *testing.T) {
	for _, key := range []string{"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY", "AWS_SESSION_TOKEN", "AWS_PROFILE", "AWS_DEFAULT_PROFILE", "AWS_CONTAINER_CREDENTIALS_RELATIVE_URI", "AWS_WEB_IDENTITY_TOKEN_FILE", "AWS_ROLE_ARN"} {
		t.Setenv(key, "")
	}
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(t.TempDir(), "missing"))
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(t.TempDir(), "missing"))
	t.Setenv("AWS_REGION", "us-east-1")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write([]byte(`{"AccessKeyId":"task-access","SecretAccessKey":"task-secret","Token":"task-token","Expiration":"2099-01-01T00:00:00Z"}`))
		assert.NoError(t, err)
	}))
	defer server.Close()
	t.Setenv("AWS_CONTAINER_CREDENTIALS_FULL_URI", server.URL)
	ecsc, ec2c, err := newEC2MetadataClients(t.Context(), "us-west-2")
	require.NoError(t, err)
	ecsOptions := ecsc.(*ecs.Client).Options()
	assert.Equal(t, "us-west-2", ecsOptions.Region)
	assert.Equal(t, "us-west-2", ec2c.(*ec2.Client).Options().Region)
	credentials, err := ecsOptions.Credentials.Retrieve(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "task-access", credentials.AccessKeyID)
	assert.Equal(t, "task-token", credentials.SessionToken)
}

func TestEC2MetadataErrorsArePreserved(t *testing.T) {
	credentialErr := errors.New("failed to retrieve credentials: no EC2 IMDS role found")
	networkErr := &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED}
	apiErr := &smithy.GenericAPIError{Code: "AccessDeniedException", Message: "not authorized to perform ecs:DescribeTasks"}

	tests := []struct {
		name      string
		change    func(*Detector, *enrichmentAPI)
		operation string
		target    error
	}{
		{"configuration credentials", func(d *Detector, _ *enrichmentAPI) {
			d.clients = func(context.Context, string) (ecsAPI, ec2API, error) { return nil, nil, credentialErr }
		}, "AWS configuration", credentialErr},
		{"DescribeTasks API", func(_ *Detector, a *enrichmentAPI) { a.taskErr = apiErr }, "DescribeTasks", apiErr},
		{"DescribeTasks network", func(_ *Detector, a *enrichmentAPI) { a.taskErr = networkErr }, "DescribeTasks", networkErr},
		{"DescribeContainerInstances API", func(_ *Detector, a *enrichmentAPI) { a.containerErr = apiErr }, "DescribeContainerInstances", apiErr},
		{"DescribeContainerInstances credentials", func(_ *Detector, a *enrichmentAPI) { a.containerErr = credentialErr }, "DescribeContainerInstances", credentialErr},
		{"DescribeInstances API", func(_ *Detector, a *enrichmentAPI) { a.instanceErr = apiErr }, "DescribeInstances", apiErr},
		{"DescribeInstances network", func(_ *Detector, a *enrichmentAPI) { a.instanceErr = networkErr }, "DescribeInstances", networkErr},
		{"DescribeInstances deadline", func(_ *Detector, a *enrichmentAPI) { a.instanceErr = context.DeadlineExceeded }, "DescribeInstances", context.DeadlineExceeded},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, a, p, _ := newEnrichmentDetector(t)
			tt.change(d, a)
			_, err := d.lookupEC2Host(t.Context(), &p.task)
			require.Error(t, err)
			assert.ErrorIs(t, err, tt.target)
			assert.ErrorContains(t, err, tt.operation+": ")
			assert.ErrorContains(t, err, tt.target.Error())
		})
	}

	t.Run("API error is reachable with errors.As", func(t *testing.T) {
		d, a, p, _ := newEnrichmentDetector(t)
		a.taskErr = &smithy.OperationError{ServiceID: "ECS", OperationName: "DescribeTasks", Err: apiErr}
		_, err := d.lookupEC2Host(t.Context(), &p.task)
		api, ok := errors.AsType[smithy.APIError](err)
		require.True(t, ok)
		assert.Equal(t, "AccessDeniedException", api.ErrorCode())

		d, a, p, _ = newEnrichmentDetector(t)
		a.instanceErr = networkErr
		_, err = d.lookupEC2Host(t.Context(), &p.task)
		require.Error(t, err)
		netErr, ok := errors.AsType[*net.OpError](err)
		require.True(t, ok)
		assert.Equal(t, "dial", netErr.Op)
	})

	t.Run("client context error propagates", func(t *testing.T) {
		d, a, p, _ := newEnrichmentDetector(t)
		ctx, cancel := context.WithCancel(t.Context())
		a.ctx = ctx
		a.wait = true
		cancel()
		_, err := d.lookupEC2Host(ctx, &p.task)
		require.ErrorIs(t, err, context.Canceled)
		assert.ErrorContains(t, err, "DescribeTasks: ")
	})

	t.Run("arbitrary error is not replaced by context error", func(t *testing.T) {
		d, a, p, _ := newEnrichmentDetector(t)
		ctx, cancel := context.WithCancel(t.Context())
		a.ctx = ctx
		a.taskErr = apiErr
		cancel()
		_, err := d.lookupEC2Host(ctx, &p.task)
		require.ErrorIs(t, err, apiErr)
		assert.NotErrorIs(t, err, context.Canceled)
	})
}
