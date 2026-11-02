// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !omit_detector_aws_ecs

package ecs

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
	"github.com/aws/smithy-go"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil"
)

type ecsAPI interface {
	DescribeTasks(context.Context, *ecs.DescribeTasksInput, ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error)
	DescribeContainerInstances(context.Context, *ecs.DescribeContainerInstancesInput, ...func(*ecs.Options)) (*ecs.DescribeContainerInstancesOutput, error)
}
type ec2API interface {
	DescribeInstances(context.Context, *ec2.DescribeInstancesInput, ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
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

func validResource(parts []string, cluster string) bool {
	return len(parts) == 2 && parts[1] != "" || len(parts) == 3 && parts[1] == cluster && parts[2] != ""
}

func parseARN(s, kind string) (arn.ARN, error) {
	a, err := arn.Parse(s)
	if err != nil || a.Service != "ecs" || a.Partition == "" || a.Region == "" || len(a.AccountID) != 12 || strings.IndexFunc(a.AccountID, func(r rune) bool { return r < '0' || r > '9' }) >= 0 || !strings.HasPrefix(a.Resource, kind+"/") {
		return arn.ARN{}, fmt.Errorf("invalid ECS %s ARN", kind)
	}
	return a, nil
}

func sameScope(a, b arn.ARN) bool {
	return a.Partition == b.Partition && a.Region == b.Region && a.AccountID == b.AccountID
}

func (d *Detector) enrichEC2(ctx context.Context, meta *ecsutil.TaskMetadata) error {
	task, err := parseARN(meta.TaskARN, "task")
	if err != nil {
		return err
	}
	cluster := meta.Cluster
	if cluster == "" {
		return errors.New("missing ECS cluster")
	}
	clusterName := cluster
	if strings.HasPrefix(cluster, "arn:") {
		a, e := parseARN(cluster, "cluster")
		if e != nil || !sameScope(task, a) {
			return errors.New("invalid ECS cluster identity")
		}
		clusterName = strings.TrimPrefix(a.Resource, "cluster/")
	}
	if clusterName == "" || strings.ContainsAny(clusterName, "/: \t\r\n") {
		return errors.New("invalid ECS cluster name")
	}
	parts := strings.Split(task.Resource, "/")
	if !validResource(parts, clusterName) {
		return errors.New("task and cluster identity mismatch")
	}
	ecsc, ec2c, err := d.clients(ctx, task.Region)
	if err != nil {
		return ec2MetadataError(ctx, "AWS configuration", err)
	}
	tasks, err := ecsc.DescribeTasks(ctx, &ecs.DescribeTasksInput{Cluster: aws.String(cluster), Tasks: []string{meta.TaskARN}})
	if err != nil {
		return ec2MetadataError(ctx, "DescribeTasks", err)
	}
	if tasks == nil || len(tasks.Failures) != 0 || len(tasks.Tasks) != 1 || aws.ToString(tasks.Tasks[0].TaskArn) != meta.TaskARN || aws.ToString(tasks.Tasks[0].ClusterArn) == "" {
		return errors.New("DescribeTasks returned missing or mismatched task")
	}
	ca, e := parseARN(aws.ToString(tasks.Tasks[0].ClusterArn), "cluster")
	if e != nil || !sameScope(task, ca) || ca.Resource != "cluster/"+clusterName {
		return errors.New("DescribeTasks returned mismatched cluster")
	}
	ci := aws.ToString(tasks.Tasks[0].ContainerInstanceArn)
	cia, e := parseARN(ci, "container-instance")
	if e != nil || !sameScope(task, cia) {
		return errors.New("invalid container instance identity")
	}
	cp := strings.Split(cia.Resource, "/")
	if !validResource(cp, clusterName) {
		return errors.New("container instance cluster mismatch")
	}
	cis, err := ecsc.DescribeContainerInstances(ctx, &ecs.DescribeContainerInstancesInput{Cluster: aws.String(cluster), ContainerInstances: []string{ci}})
	if err != nil {
		return ec2MetadataError(ctx, "DescribeContainerInstances", err)
	}
	if cis == nil || len(cis.Failures) != 0 || len(cis.ContainerInstances) != 1 || aws.ToString(cis.ContainerInstances[0].ContainerInstanceArn) != ci {
		return errors.New("DescribeContainerInstances returned missing or mismatched identity")
	}
	id := aws.ToString(cis.ContainerInstances[0].Ec2InstanceId)
	if id == "" {
		return errors.New("missing EC2 instance ID")
	}
	instances, err := ec2c.DescribeInstances(ctx, &ec2.DescribeInstancesInput{InstanceIds: []string{id}})
	if err != nil {
		return ec2MetadataError(ctx, "DescribeInstances", err)
	}
	if instances == nil || len(instances.Reservations) != 1 || aws.ToString(instances.Reservations[0].OwnerId) != task.AccountID || len(instances.Reservations[0].Instances) != 1 {
		return errors.New("DescribeInstances returned missing, ambiguous or mismatched owner")
	}
	instance := instances.Reservations[0].Instances[0]
	if aws.ToString(instance.InstanceId) != id {
		return errors.New("DescribeInstances returned mismatched instance")
	}
	zone := ""
	if instance.Placement != nil {
		zone = aws.ToString(instance.Placement.AvailabilityZone)
	}
	// Apply host metadata only after the complete identity chain and fields have
	// been validated, so partial or ambiguous API responses cannot invent a host.
	name, typ, image := aws.ToString(instance.PrivateDnsName), string(instance.InstanceType), aws.ToString(instance.ImageId)
	if strings.TrimSpace(name) == "" || typ == "" || strings.TrimSpace(image) == "" || strings.TrimSpace(zone) == "" {
		return errors.New("DescribeInstances returned incomplete host metadata")
	}
	d.rb.SetHostID(id)
	d.rb.SetHostName(name)
	d.rb.SetHostType(typ)
	d.rb.SetHostImageID(image)
	d.rb.SetCloudAccountID(task.AccountID)
	d.rb.SetCloudRegion(task.Region)
	d.rb.SetCloudAvailabilityZone(zone)
	return nil
}

// Log operation names and API error codes without dumping request or credential
// provider errors, which can contain endpoint URLs or response bodies.
func ec2MetadataError(ctx context.Context, operation string, err error) error {
	if ctx.Err() != nil {
		return fmt.Errorf("%s: %w", operation, ctx.Err())
	}
	if api, ok := errors.AsType[smithy.APIError](err); ok {
		code := api.ErrorCode()
		if code != "" && strings.IndexFunc(code, func(r rune) bool {
			return !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-", r)
		}) < 0 {
			return fmt.Errorf("%s: %s", operation, code)
		}
	}
	return fmt.Errorf("%s failed", operation)
}
