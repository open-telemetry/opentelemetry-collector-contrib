// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecs // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/ecs"

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	conventions "go.opentelemetry.io/otel/semconv/v1.39.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil/endpoints"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/ecs/internal/metadata"
)

const (
	// TypeStr is type of detector.
	TypeStr = "ecs"
)

var _ internal.Detector = (*Detector)(nil)

type Detector struct {
	provider ecsutil.MetadataProvider
	rb       *metadata.ResourceBuilder
}

func NewDetector(params processor.Settings, dcfg internal.DetectorConfig) (internal.Detector, error) {
	cfg := dcfg.(Config)
	provider, err := ecsutil.NewDetectedTaskMetadataProvider(params.TelemetrySettings)
	if err != nil {
		// Allow metadata provider to be created in incompatible environments and just have a noop Detect()
		var errNTMED endpoints.ErrNoTaskMetadataEndpointDetected
		if errors.As(err, &errNTMED) {
			return &Detector{provider: nil}, nil
		}
		return nil, fmt.Errorf("unable to create task metadata provider: %w", err)
	}
	return &Detector{provider: provider, rb: metadata.NewResourceBuilder(cfg.ResourceAttributes)}, nil
}

// Detect records metadata retrieved from the ECS Task Metadata Endpoint (TMDE) as resource attributes
// TODO(willarmiros): Replace all attribute fields and enums with values defined in "conventions" once they exist
func (d *Detector) Detect(context.Context) (resource pcommon.Resource, schemaURL string, err error) {
	// don't attempt to fetch metadata if there's no provider (incompatible env)
	if d.provider == nil {
		return pcommon.NewResource(), "", nil
	}

	tmdeResp, err := d.provider.FetchTaskMetadata()

	if err != nil || tmdeResp == nil {
		return pcommon.NewResource(), "", fmt.Errorf("unable to fetch task metadata: %w", err)
	}

	d.rb.SetCloudProvider(conventions.CloudProviderAWS.Value.AsString())
	d.rb.SetCloudPlatform(conventions.CloudPlatformAWSECS.Value.AsString())
	d.rb.SetAwsEcsTaskArn(tmdeResp.TaskARN)
	d.rb.SetAwsEcsTaskFamily(tmdeResp.Family)
	d.rb.SetAwsEcsTaskRevision(tmdeResp.Revision)

	region, account, taskID := parseTaskARN(tmdeResp.TaskARN)
	if account != "" {
		d.rb.SetCloudAccountID(account)
	}

	if region != "" {
		d.rb.SetCloudRegion(region)
	}

	if taskID != "" {
		d.rb.SetAwsEcsTaskID(taskID)
	}

	// TMDE returns the cluster short name or ARN, so we need to construct the ARN if necessary
	d.rb.SetAwsEcsClusterArn(constructClusterArn(tmdeResp.Cluster, region, account))

	if tmdeResp.AvailabilityZone != "" {
		d.rb.SetCloudAvailabilityZone(tmdeResp.AvailabilityZone)
	}

	// The launch type and log data attributes are only available in TMDE v4
	switch lt := strings.ToLower(tmdeResp.LaunchType); lt {
	case "ec2":
		d.rb.SetAwsEcsLaunchtype("ec2")
	case "fargate":
		d.rb.SetAwsEcsLaunchtype("fargate")
	}

	selfMetaData, err := d.provider.FetchContainerMetadata()

	if err != nil || selfMetaData == nil {
		return d.rb.Emit(), "", err
	}

	addValidLogData(tmdeResp.Containers, selfMetaData, account, d.rb)

	return d.rb.Emit(), conventions.SchemaURL, nil
}

func constructClusterArn(cluster, region, account string) string {
	// If cluster is already an ARN, return it
	if bytes.IndexByte([]byte(cluster), byte(':')) != -1 {
		return cluster
	}

	return fmt.Sprintf("arn:aws:ecs:%s:%s:cluster/%s", region, account, cluster)
}

// Parses ECS Task ARN into subcomponents according to its spec
// See: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs-account-settings.html#ecs-resource-ids
func parseTaskARN(taskARN string) (region, account, taskID string) {
	parts := strings.Split(taskARN, ":")
	if len(parts) >= 5 {
		region := parts[3]
		account := parts[4]

		// ECS Task ARNs come in two versions. In the old one, the last part of the ARN contains
		// only the "task/<task-id>". In the new one, it contains "task/cluster-name/task-id".
		// This handles both cases.
		taskInfo := parts[5]
		taskInfoParts := strings.Split(taskInfo, "/")
		taskID := taskInfoParts[len(taskInfoParts)-1]

		return region, account, taskID
	}

	return "", "", ""
}

// Filter out non-normal containers, our own container since we assume the collector is run as a sidecar,
// "init" containers which only run at startup then shutdown (as indicated by the "KnownStatus" attribute),
// containers not using AWS Logs, and those without log group metadata to get the final lists of valid log data
// See: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-metadata-endpoint-v4.html#task-metadata-endpoint-v4-response
func addValidLogData(containers []ecsutil.ContainerMetadata, self *ecsutil.ContainerMetadata, account string, rb *metadata.ResourceBuilder) {
	logGroupNames := make([]any, 0, len(containers))
	logGroupArns := make([]any, 0, len(containers))
	logStreamNames := make([]any, 0, len(containers))
	logStreamArns := make([]any, 0, len(containers))
	containerFound := false

	for i := range containers {
		container := &containers[i]
		logData := container.LogOptions
		if container.Type == "NORMAL" &&
			container.KnownStatus == "RUNNING" &&
			container.LogDriver == "awslogs" &&
			self.DockerID != container.DockerID &&
			logData != (ecsutil.LogOptions{}) {
			containerFound = true
			logGroupNames = append(logGroupNames, logData.LogGroup)
			logGroupArns = append(logGroupArns, constructLogGroupArn(logData.Region, account, logData.LogGroup))
			logStreamNames = append(logStreamNames, logData.Stream)
			logStreamArns = append(logStreamArns, constructLogStreamArn(logData.Region, account, logData.LogGroup, logData.Stream))
		}
	}

	if containerFound {
		rb.SetAwsLogGroupNames(logGroupNames)
		rb.SetAwsLogGroupArns(logGroupArns)
		rb.SetAwsLogStreamNames(logStreamNames)
		rb.SetAwsLogStreamArns(logStreamArns)
	}
}

func constructLogGroupArn(region, account, group string) string {
	return fmt.Sprintf("arn:aws:logs:%s:%s:log-group:%s", region, account, group)
}

func constructLogStreamArn(region, account, group, stream string) string {
	return fmt.Sprintf("%s:log-stream:%s", constructLogGroupArn(region, account, group), stream)
}
