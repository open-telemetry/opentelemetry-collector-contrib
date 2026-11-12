// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build (!remove_all_resourcedetection_detectors || enable_detector_aws_ecs) && !omit_detector_aws_ecs

package ecs // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/ecs"

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecs"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil"
)

type ecsAPI interface {
	DescribeTasks(context.Context, *ecs.DescribeTasksInput, ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error)
	DescribeContainerInstances(context.Context, *ecs.DescribeContainerInstancesInput, ...func(*ecs.Options)) (*ecs.DescribeContainerInstancesOutput, error)
}

type ec2API interface {
	DescribeInstances(context.Context, *ec2.DescribeInstancesInput, ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
}

// ec2Host holds the EC2 instance metadata for the task identified by taskARN.
// Only the instance ID is required; the other fields are optional and empty
// when EC2 does not report them.
type ec2Host struct {
	taskARN string
	id      string
	name    string
	typ     string
	image   string
	zone    string
}

// Use the default credentials chain (including the ECS task role), but never
// fall back to the EC2 instance role through IMDS.
func newEC2MetadataClients(ctx context.Context, region string) (ecsAPI, ec2API, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region), config.WithEC2IMDSClientEnableState(imds.ClientDisabled))
	if err != nil {
		return nil, nil, err
	}
	return ecs.NewFromConfig(cfg), ec2.NewFromConfig(cfg), nil
}

// lookupEC2Host resolves the EC2 instance running the task described by meta:
// task -> container instance -> EC2 instance.
func (d *Detector) lookupEC2Host(ctx context.Context, meta *ecsutil.TaskMetadata) (*ec2Host, error) {
	task, err := arn.Parse(meta.TaskARN)
	if err != nil || task.Region == "" {
		return nil, errors.New("invalid ECS task ARN")
	}
	if meta.Cluster == "" {
		return nil, errors.New("missing ECS cluster")
	}
	ecsc, ec2c, err := d.clients(ctx, task.Region)
	if err != nil {
		return nil, fmt.Errorf("AWS configuration: %w", err)
	}

	tasks, err := ecsc.DescribeTasks(ctx, &ecs.DescribeTasksInput{Cluster: aws.String(meta.Cluster), Tasks: []string{meta.TaskARN}})
	if err != nil {
		return nil, fmt.Errorf("DescribeTasks: %w", err)
	}
	if tasks == nil || len(tasks.Failures) != 0 || len(tasks.Tasks) != 1 || aws.ToString(tasks.Tasks[0].TaskArn) != meta.TaskARN {
		return nil, errors.New("DescribeTasks returned missing or mismatched task")
	}
	containerInstance := aws.ToString(tasks.Tasks[0].ContainerInstanceArn)
	if containerInstance == "" {
		return nil, errors.New("missing container instance ARN")
	}

	containers, err := ecsc.DescribeContainerInstances(ctx, &ecs.DescribeContainerInstancesInput{Cluster: aws.String(meta.Cluster), ContainerInstances: []string{containerInstance}})
	if err != nil {
		return nil, fmt.Errorf("DescribeContainerInstances: %w", err)
	}
	if containers == nil || len(containers.Failures) != 0 || len(containers.ContainerInstances) != 1 || aws.ToString(containers.ContainerInstances[0].ContainerInstanceArn) != containerInstance {
		return nil, errors.New("DescribeContainerInstances returned missing or mismatched container instance")
	}
	id := aws.ToString(containers.ContainerInstances[0].Ec2InstanceId)
	if id == "" {
		return nil, errors.New("missing EC2 instance ID")
	}

	instances, err := ec2c.DescribeInstances(ctx, &ec2.DescribeInstancesInput{InstanceIds: []string{id}})
	if err != nil {
		return nil, fmt.Errorf("DescribeInstances: %w", err)
	}
	if instances == nil || len(instances.Reservations) != 1 || len(instances.Reservations[0].Instances) != 1 || aws.ToString(instances.Reservations[0].Instances[0].InstanceId) != id {
		return nil, errors.New("DescribeInstances returned missing or mismatched instance")
	}
	instance := instances.Reservations[0].Instances[0]

	host := &ec2Host{
		taskARN: meta.TaskARN,
		id:      id,
		name:    strings.TrimSpace(aws.ToString(instance.PrivateDnsName)),
		typ:     string(instance.InstanceType),
		image:   strings.TrimSpace(aws.ToString(instance.ImageId)),
	}
	if instance.Placement != nil {
		host.zone = strings.TrimSpace(aws.ToString(instance.Placement.AvailabilityZone))
	}
	return host, nil
}

// applyEC2Host records the host attributes that EC2 reported. Attributes the
// user disabled are dropped by the resource builder, and fields EC2 did not
// report are never emitted as empty values. Account and Region are already set
// from the task ARN; the zone is the only cloud attribute EC2 adds.
func (d *Detector) applyEC2Host(host *ec2Host) {
	d.rb.SetHostID(host.id)
	if host.name != "" {
		d.rb.SetHostName(host.name)
	}
	if host.typ != "" {
		d.rb.SetHostType(host.typ)
	}
	if host.image != "" {
		d.rb.SetHostImageID(host.image)
	}
	if host.zone != "" {
		d.rb.SetCloudAvailabilityZone(host.zone)
	}
}
