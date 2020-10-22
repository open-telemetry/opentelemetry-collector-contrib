// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package translator

import (
	"strconv"

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/awsxray"
)

func addAWSToResource(aws *awsxray.AWSData, attrs *pdata.AttributeMap) {
	if aws == nil {
		// https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/c615d2db351929b99e46f7b427f39c12afe15b54/exporter/awsxrayexporter/translator/aws.go#L121
		// this implies that the current segment being processed is not generated
		// by an AWS entity.
		attrs.UpsertString(conventions.AttributeCloudProvider, "unknown")
		return
	}

	attrs.UpsertString(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	addString(aws.AccountID, conventions.AttributeCloudAccount, attrs)

	// based on https://docs.aws.amazon.com/xray/latest/devguide/xray-api-segmentdocuments.html#api-segmentdocuments-aws
	// it's possible to have all ec2, ecs and beanstalk fields at the same time.
	if ec2 := aws.EC2; ec2 != nil {
		addString(ec2.AvailabilityZone, conventions.AttributeCloudZone, attrs)
		addString(ec2.InstanceID, conventions.AttributeHostID, attrs)
		addString(ec2.InstanceSize, conventions.AttributeHostType, attrs)
		addString(ec2.AmiID, conventions.AttributeHostImageID, attrs)
	}

	if ecs := aws.ECS; ecs != nil {
		addString(ecs.ContainerName, conventions.AttributeContainerName, attrs)
	}

	if bs := aws.Beanstalk; bs != nil {
		addString(bs.Environment, conventions.AttributeServiceNamespace, attrs)
		if bs.DeploymentID != nil {
			attrs.UpsertString(conventions.AttributeServiceInstance, strconv.FormatInt(*bs.DeploymentID, 10))
		}
		addString(bs.VersionLabel, conventions.AttributeServiceVersion, attrs)
	}
}

func addAWSToSpan(aws *awsxray.AWSData, attrs *pdata.AttributeMap) {
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
