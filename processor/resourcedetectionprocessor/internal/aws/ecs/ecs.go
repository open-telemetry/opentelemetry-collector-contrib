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

package ecs // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/ecs"

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil/endpoints"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

const (
	// TypeStr is type of detector.
	TypeStr = "ecs"
)

var _ internal.Detector = (*Detector)(nil)

type Detector struct {
	provider ecsutil.MetadataProvider
}

func NewDetector(params component.ProcessorCreateSettings, _ internal.DetectorConfig) (internal.Detector, error) {
	provider, err := ecsutil.NewDetectedTaskMetadataProvider(params.TelemetrySettings)
	if err != nil {
		// Allow metadata provider to be created in incompatible environments and just have a noop Detect()
		if _, ok := err.(endpoints.ErrNoTaskMetadataEndpointDetected); ok {
			return &Detector{provider: nil}, nil
		}
		return nil, fmt.Errorf("unable to create task metadata provider: %w", err)
	}
	return &Detector{provider: provider}, nil
}

// Detect records metadata retrieved from the ECS Task Metadata Endpoint (TMDE) as resource attributes
// TODO(willarmiros): Replace all attribute fields and enums with values defined in "conventions" once they exist
func (d *Detector) Detect(context.Context) (resource pcommon.Resource, schemaURL string, err error) {
	res := pcommon.NewResource()

	// don't attempt to fetch metadata if there's no provider (incompatible env)
	if d.provider == nil {
		return res, "", nil
	}

	tmdeResp, err := d.provider.FetchTaskMetadata()

	if err != nil || tmdeResp == nil {
		return res, "", fmt.Errorf("unable to fetch task metadata: %w", err)
	}

	attr := res.Attributes()
	attr.InsertString(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	attr.InsertString(conventions.AttributeCloudPlatform, conventions.AttributeCloudPlatformAWSECS)
	attr.InsertString(conventions.AttributeAWSECSTaskARN, tmdeResp.TaskARN)
	attr.InsertString(conventions.AttributeAWSECSTaskFamily, tmdeResp.Family)
	attr.InsertString(conventions.AttributeAWSECSTaskRevision, tmdeResp.Revision)

	region, account := parseRegionAndAccount(tmdeResp.TaskARN)
	if account != "" {
		attr.InsertString(conventions.AttributeCloudAccountID, account)
	}

	if region != "" {
		attr.InsertString(conventions.AttributeCloudRegion, region)
	}

	// TMDE returns the the cluster short name or ARN, so we need to construct the ARN if necessary
	attr.InsertString(conventions.AttributeAWSECSClusterARN, constructClusterArn(tmdeResp.Cluster, region, account))

	// The Availability Zone is not available in all Fargate runtimes
	if tmdeResp.AvailabilityZone != "" {
		attr.InsertString(conventions.AttributeCloudAvailabilityZone, tmdeResp.AvailabilityZone)
	}

	// The launch type and log data attributes are only available in TMDE v4
	switch lt := strings.ToLower(tmdeResp.LaunchType); lt {
	case "ec2":
		attr.InsertString(conventions.AttributeAWSECSLaunchtype, "ec2")

	case "fargate":
		attr.InsertString(conventions.AttributeAWSECSLaunchtype, "fargate")
	}

	selfMetaData, err := d.provider.FetchContainerMetadata()

	if err != nil || selfMetaData == nil {
		return res, "", err
	}

	logAttributes := [4]string{
		conventions.AttributeAWSLogGroupNames,
		conventions.AttributeAWSLogGroupARNs,
		conventions.AttributeAWSLogStreamNames,
		conventions.AttributeAWSLogStreamARNs,
	}

	for i, attribVal := range getValidLogData(tmdeResp.Containers, selfMetaData, account) {
		if attribVal.SliceVal().Len() > 0 {
			attr.Insert(logAttributes[i], attribVal)
		}
	}

	return res, conventions.SchemaURL, nil
}

func constructClusterArn(cluster, region, account string) string {
	// If cluster is already an ARN, return it
	if bytes.IndexByte([]byte(cluster), byte(':')) != -1 {
		return cluster
	}

	return fmt.Sprintf("arn:aws:ecs:%s:%s:cluster/%s", region, account, cluster)
}

// Parses the AWS Account ID and AWS Region from a task ARN
// See: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs-account-settings.html#ecs-resource-ids
func parseRegionAndAccount(taskARN string) (region string, account string) {
	parts := strings.Split(taskARN, ":")
	if len(parts) >= 5 {
		return parts[3], parts[4]
	}

	return "", ""
}

// Filter out non-normal containers, our own container since we assume the collector is run as a sidecar,
// "init" containers which only run at startup then shutdown (as indicated by the "KnownStatus" attribute),
// containers not using AWS Logs, and those without log group metadata to get the final lists of valid log data
// See: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-metadata-endpoint-v4.html#task-metadata-endpoint-v4-response
func getValidLogData(containers []ecsutil.ContainerMetadata, self *ecsutil.ContainerMetadata, account string) [4]pcommon.Value {
	logGroupNames := pcommon.NewValueSlice()
	logGroupArns := pcommon.NewValueSlice()
	logStreamNames := pcommon.NewValueSlice()
	logStreamArns := pcommon.NewValueSlice()

	for _, container := range containers {
		logData := container.LogOptions
		if container.Type == "NORMAL" &&
			container.KnownStatus == "RUNNING" &&
			container.LogDriver == "awslogs" &&
			self.DockerID != container.DockerID &&
			logData != (ecsutil.LogOptions{}) {

			logGroupNames.SliceVal().AppendEmpty().SetStringVal(logData.LogGroup)
			logGroupArns.SliceVal().AppendEmpty().SetStringVal(constructLogGroupArn(logData.Region, account, logData.LogGroup))
			logStreamNames.SliceVal().AppendEmpty().SetStringVal(logData.Stream)
			logStreamArns.SliceVal().AppendEmpty().SetStringVal(constructLogStreamArn(logData.Region, account, logData.LogGroup, logData.Stream))
		}
	}

	return [4]pcommon.Value{logGroupNames, logGroupArns, logStreamNames, logStreamArns}
}

func constructLogGroupArn(region, account, group string) string {
	return fmt.Sprintf("arn:aws:logs:%s:%s:log-group:%s", region, account, group)
}

func constructLogStreamArn(region, account, group, stream string) string {
	return fmt.Sprintf("%s:log-stream:%s", constructLogGroupArn(region, account, group), stream)
}
