// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver/internal/translator"

import (
	"strconv"

	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"

	awsxray "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"
)

func addAWSToResource(aws *awsxray.AWSData, attrs pcommon.Map) {
	if aws == nil {
		// https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/c615d2db351929b99e46f7b427f39c12afe15b54/exporter/awsxrayexporter/translator/aws.go#L121
		// this implies that the current segment being processed is not generated
		// by an AWS entity.
		attrs.PutStr(string(conventions.CloudProviderKey), "unknown")
		return
	}

	attrs.PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAWS.Value.AsString())
	addString(aws.AccountID, string(conventions.CloudAccountIDKey), attrs)

	// based on https://docs.aws.amazon.com/xray/latest/devguide/xray-api-segmentdocuments.html#api-segmentdocuments-aws
	// it's possible to have all cloudwatch_logs, ec2, ecs and beanstalk fields at the same time.
	if cwl := aws.CWLogs; cwl != nil {
		for _, logGroupMetaData := range cwl {
			addStringSlice(logGroupMetaData.Arn, string(conventions.AWSLogGroupARNsKey), attrs)
			addStringSlice(logGroupMetaData.LogGroup, string(conventions.AWSLogGroupNamesKey), attrs)
		}
	}

	if ec2 := aws.EC2; ec2 != nil {
		addString(ec2.AvailabilityZone, string(conventions.CloudAvailabilityZoneKey), attrs)
		addString(ec2.InstanceID, string(conventions.HostIDKey), attrs)
		addString(ec2.InstanceSize, string(conventions.HostTypeKey), attrs)
		addString(ec2.AmiID, string(conventions.HostImageIDKey), attrs)
	}

	if ecs := aws.ECS; ecs != nil {
		addString(ecs.ContainerName, string(conventions.ContainerNameKey), attrs)
		addString(ecs.AvailabilityZone, string(conventions.CloudAvailabilityZoneKey), attrs)
		addString(ecs.ContainerID, string(conventions.ContainerIDKey), attrs)
	}

	if bs := aws.Beanstalk; bs != nil {
		addString(bs.Environment, string(conventions.ServiceNamespaceKey), attrs)
		if bs.DeploymentID != nil {
			attrs.PutStr(string(conventions.ServiceInstanceIDKey), strconv.FormatInt(*bs.DeploymentID, 10))
		}
		addString(bs.VersionLabel, string(conventions.ServiceVersionKey), attrs)
	}

	if eks := aws.EKS; eks != nil {
		addString(eks.ContainerID, string(conventions.ContainerIDKey), attrs)
		addString(eks.ClusterName, string(conventions.K8SClusterNameKey), attrs)
		addString(eks.Pod, string(conventions.K8SPodNameKey), attrs)
	}
}

func addAWSToSpan(aws *awsxray.AWSData, attrs pcommon.Map) {
	if aws != nil {
		addString(aws.AccountID, awsxray.AWSAccountAttribute, attrs)
		addString(aws.Operation, awsxray.AWSOperationAttribute, attrs)
		addString(aws.RemoteRegion, awsxray.AWSRegionAttribute, attrs)
		addString(aws.RequestID, awsxray.AWSRequestIDAttribute, attrs)
		addString(aws.QueueURL, awsxray.AWSQueueURLAttribute, attrs)
		addString(aws.TableName, awsxray.AWSTableNameAttribute, attrs)
		addInt64(aws.Retries, awsxray.AWSXrayRetriesAttribute, attrs)
	}
}
