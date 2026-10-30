// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecs

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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
	enrichmentCluster           = "arn:aws:ecs:us-west-2:123456789123:cluster/my-cluster"
	enrichmentContainerInstance = "arn:aws:ecs:us-west-2:123456789123:container-instance/my-cluster/456"
	enrichmentInstance          = "i-0123456789abcdef0"
)

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
	taskARN, containerARN              string
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
	assert.Equal(a.t, []string{enrichmentInstance}, in.InstanceIds)
	a.calls = append(a.calls, "instances")
	return a.instances, a.instanceErr
}

func newEnrichmentDetector(t *testing.T) (*Detector, *enrichmentAPI, *enrichmentMetadataProvider, *observer.ObservedLogs) {
	p := &enrichmentMetadataProvider{task: ecsutil.TaskMetadata{Cluster: "my-cluster", TaskARN: enrichmentTask, LaunchType: "EC2", AvailabilityZone: "task-zone"}}
	a := &enrichmentAPI{
		t: t, ctx: t.Context(), taskARN: enrichmentTask, containerARN: enrichmentContainerInstance,
		tasks:      &ecs.DescribeTasksOutput{Tasks: []ecstypes.Task{{TaskArn: aws.String(enrichmentTask), ClusterArn: aws.String(enrichmentCluster), ContainerInstanceArn: aws.String(enrichmentContainerInstance)}}},
		containers: &ecs.DescribeContainerInstancesOutput{ContainerInstances: []ecstypes.ContainerInstance{{ContainerInstanceArn: aws.String(enrichmentContainerInstance), Ec2InstanceId: aws.String(enrichmentInstance)}}},
		instances:  &ec2.DescribeInstancesOutput{Reservations: []ec2types.Reservation{{OwnerId: aws.String("123456789123"), Instances: []ec2types.Instance{{InstanceId: aws.String(enrichmentInstance), PrivateDnsName: aws.String("ip-10-0-0-1.us-west-2.compute.internal"), InstanceType: ec2types.InstanceTypeT3Micro, ImageId: aws.String("ami-123"), Placement: &ec2types.Placement{AvailabilityZone: aws.String("us-west-2a")}}}}}},
	}
	core, logs := observer.New(zap.WarnLevel)
	d := &Detector{provider: p, rb: metadata.NewResourceBuilder(metadata.DefaultResourceAttributesConfig()), ec2Metadata: true, logger: zap.New(core), clients: func(ctx context.Context, region string) (ecsAPI, ec2API, error) {
		assert.Equal(t, a.ctx, ctx)
		assert.Equal(t, "us-west-2", region)
		return a, a, nil
	}}
	return d, a, p, logs
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
			assert.Equal(t, 0, logs.Len())
		})
	}
}

func TestEC2MetadataSkipped(t *testing.T) {
	assert.False(t, CreateDefaultConfig().EC2Metadata)
	for _, tt := range []struct {
		name, launch string
		enabled      bool
	}{{"disabled", "EC2", false}, {"fargate", "FARGATE", true}, {"missing launch", "", true}, {"unknown launch", "other", true}} {
		t.Run(tt.name, func(t *testing.T) {
			d, a, p, logs := newEnrichmentDetector(t)
			d.ec2Metadata = tt.enabled
			p.task.LaunchType = tt.launch
			d.clients = func(context.Context, string) (ecsAPI, ec2API, error) {
				t.Fatal("unexpected AWS clients")
				return nil, nil, nil
			}
			resource, _, err := d.Detect(t.Context())
			require.NoError(t, err)
			assert.NotContains(t, resource.Attributes().AsRaw(), "host.id")
			assert.Empty(t, a.calls)
			assert.Zero(t, logs.Len())
		})
	}
}

func TestEC2MetadataFailuresPreserveECS(t *testing.T) {
	denial := &smithy.GenericAPIError{Code: "AccessDeniedException", Message: "secret response"}
	for _, tt := range []struct {
		name   string
		change func(*Detector, *enrichmentAPI, *enrichmentMetadataProvider)
		calls  int
	}{
		{"malformed task", func(_ *Detector, _ *enrichmentAPI, p *enrichmentMetadataProvider) { p.task.TaskARN = "broken" }, 0},
		{"wrong service", func(_ *Detector, _ *enrichmentAPI, p *enrichmentMetadataProvider) {
			p.task.TaskARN = "arn:aws:s3:us-west-2:123456789123:task/my-cluster/123"
		}, 0},
		{"invalid account", func(_ *Detector, _ *enrichmentAPI, p *enrichmentMetadataProvider) {
			p.task.TaskARN = "arn:aws:ecs:us-west-2:owner:task/my-cluster/123"
		}, 0},
		{"missing cluster", func(_ *Detector, _ *enrichmentAPI, p *enrichmentMetadataProvider) { p.task.Cluster = "" }, 0},
		{"cluster mismatch", func(_ *Detector, _ *enrichmentAPI, p *enrichmentMetadataProvider) { p.task.Cluster = "other" }, 0},
		{"cluster account mismatch", func(_ *Detector, _ *enrichmentAPI, p *enrichmentMetadataProvider) {
			p.task.Cluster = "arn:aws:ecs:us-west-2:999999999999:cluster/my-cluster"
		}, 0},
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
		{"wrong returned cluster", func(_ *Detector, a *enrichmentAPI, _ *enrichmentMetadataProvider) {
			a.tasks.Tasks[0].ClusterArn = aws.String("arn:aws:ecs:us-west-2:123456789123:cluster/other")
		}, 1},
		{"missing container ARN", func(_ *Detector, a *enrichmentAPI, _ *enrichmentMetadataProvider) {
			a.tasks.Tasks[0].ContainerInstanceArn = nil
		}, 1},
		{"wrong container region", func(_ *Detector, a *enrichmentAPI, _ *enrichmentMetadataProvider) {
			a.tasks.Tasks[0].ContainerInstanceArn = aws.String("arn:aws:ecs:us-east-1:123456789123:container-instance/my-cluster/456")
		}, 1},
		{"wrong container cluster", func(_ *Detector, a *enrichmentAPI, _ *enrichmentMetadataProvider) {
			a.tasks.Tasks[0].ContainerInstanceArn = aws.String("arn:aws:ecs:us-west-2:123456789123:container-instance/other/456")
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
		{"wrong owner", func(_ *Detector, a *enrichmentAPI, _ *enrichmentMetadataProvider) {
			a.instances.Reservations[0].OwnerId = aws.String("999999999999")
		}, 3},
		{"no instances", func(_ *Detector, a *enrichmentAPI, _ *enrichmentMetadataProvider) {
			a.instances.Reservations[0].Instances = nil
		}, 3},
		{"ambiguous instances", func(_ *Detector, a *enrichmentAPI, _ *enrichmentMetadataProvider) {
			a.instances.Reservations[0].Instances = append(a.instances.Reservations[0].Instances, a.instances.Reservations[0].Instances[0])
		}, 3},
		{"wrong instance", func(_ *Detector, a *enrichmentAPI, _ *enrichmentMetadataProvider) {
			a.instances.Reservations[0].Instances[0].InstanceId = aws.String("i-other")
		}, 3},
		{"missing DNS", func(_ *Detector, a *enrichmentAPI, _ *enrichmentMetadataProvider) {
			a.instances.Reservations[0].Instances[0].PrivateDnsName = nil
		}, 3},
		{"missing type", func(_ *Detector, a *enrichmentAPI, _ *enrichmentMetadataProvider) {
			a.instances.Reservations[0].Instances[0].InstanceType = ""
		}, 3},
		{"missing image", func(_ *Detector, a *enrichmentAPI, _ *enrichmentMetadataProvider) {
			a.instances.Reservations[0].Instances[0].ImageId = nil
		}, 3},
		{"missing placement", func(_ *Detector, a *enrichmentAPI, _ *enrichmentMetadataProvider) {
			a.instances.Reservations[0].Instances[0].Placement = nil
		}, 3},
		{"missing zone", func(_ *Detector, a *enrichmentAPI, _ *enrichmentMetadataProvider) {
			a.instances.Reservations[0].Instances[0].Placement.AvailabilityZone = nil
		}, 3},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d, a, p, logs := newEnrichmentDetector(t)
			d.failOnMissingMetadata = true
			tt.change(d, a, p)
			resource, _, err := d.Detect(t.Context())
			require.NoError(t, err)
			attrs := resource.Attributes().AsRaw()
			assert.Equal(t, p.task.TaskARN, attrs["aws.ecs.task.arn"])
			assert.Equal(t, "aws_ecs", attrs["cloud.platform"])
			assert.Equal(t, "task-zone", attrs["cloud.availability_zone"])
			for _, key := range []string{"host.id", "host.name", "host.type", "host.image.id"} {
				assert.NotContains(t, attrs, key)
			}
			assert.Len(t, a.calls, tt.calls)
			require.Equal(t, 1, logs.Len())
			assert.NotContains(t, logs.All()[0].ContextMap()["error"], "secret")
		})
	}
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
	assert.NotContains(t, resource.Attributes().AsRaw(), "host.id")
	require.Equal(t, 1, logs.Len())
	assert.Contains(t, logs.All()[0].ContextMap()["error"], "context deadline exceeded")
}

func TestEC2MetadataAttributeSelection(t *testing.T) {
	for _, key := range []string{"host.id", "host.name", "host.image.id", "host.type", "cloud.account.id", "cloud.region", "cloud.availability_zone"} {
		t.Run(key, func(t *testing.T) {
			d, _, _, _ := newEnrichmentDetector(t)
			cfg := metadata.DefaultResourceAttributesConfig()
			switch key {
			case "host.id":
				cfg.HostID.Enabled = false
			case "host.name":
				cfg.HostName.Enabled = false
			case "host.image.id":
				cfg.HostImageID.Enabled = false
			case "host.type":
				cfg.HostType.Enabled = false
			case "cloud.account.id":
				cfg.CloudAccountID.Enabled = false
			case "cloud.region":
				cfg.CloudRegion.Enabled = false
			case "cloud.availability_zone":
				cfg.CloudAvailabilityZone.Enabled = false
			}
			d.rb = metadata.NewResourceBuilder(cfg)
			resource, _, err := d.Detect(t.Context())
			require.NoError(t, err)
			attrs := resource.Attributes().AsRaw()
			assert.NotContains(t, attrs, key)
			for _, other := range []string{"host.id", "host.name", "host.image.id", "host.type", "cloud.account.id", "cloud.region", "cloud.availability_zone"} {
				if other != key {
					assert.Contains(t, attrs, other)
				}
			}
			assert.Equal(t, "aws_ecs", attrs["cloud.platform"])
		})
	}
}

func TestEC2MetadataNewDetector(t *testing.T) {
	t.Setenv(endpoints.TaskMetadataEndpointV4EnvVar, "http://localhost/task")
	cfg := CreateDefaultConfig()
	cfg.EC2Metadata = true
	detector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), cfg, true)
	require.NoError(t, err)
	d := detector.(*Detector)
	assert.True(t, d.ec2Metadata)
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

func TestEC2MetadataSafeError(t *testing.T) {
	err := ec2MetadataError(t.Context(), "DescribeTasks", &smithy.GenericAPIError{Code: "AccessDeniedException", Message: "secret response"})
	assert.EqualError(t, err, "DescribeTasks: AccessDeniedException")
	err = ec2MetadataError(t.Context(), "DescribeTasks", &smithy.GenericAPIError{Code: "unsafe code secret", Message: "secret response"})
	assert.EqualError(t, err, "DescribeTasks failed")
	err = ec2MetadataError(t.Context(), "AWS configuration", errors.New("secret response"))
	assert.EqualError(t, err, "AWS configuration failed")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	assert.ErrorIs(t, ec2MetadataError(ctx, "DescribeTasks", errors.New("secret response")), context.Canceled)
}
