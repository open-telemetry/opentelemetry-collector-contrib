// Copyright 2019, OpenTelemetry Authors
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

	"github.com/aws/aws-sdk-go/aws"
	"go.opentelemetry.io/collector/consumer/pdata"
	semconventions "go.opentelemetry.io/collector/translator/conventions"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/awsxray"
)

func makeAws(attributes map[string]string, resource pdata.Resource) (map[string]string, *awsxray.AWSData) {
	var (
		cloud        string
		account      string
		zone         string
		hostID       string
		hostType     string
		amiID        string
		container    string
		namespace    string
		deployID     string
		versionLabel string
		operation    string
		remoteRegion string
		requestID    string
		queueURL     string
		tableName    string
		ec2          *awsxray.EC2Metadata
		ecs          *awsxray.ECSMetadata
		ebs          *awsxray.BeanstalkMetadata
	)

	filtered := make(map[string]string)
	if !resource.IsNil() {
		resource.Attributes().ForEach(func(key string, value pdata.AttributeValue) {
			switch key {
			case semconventions.AttributeCloudProvider:
				cloud = value.StringVal()
			case semconventions.AttributeCloudAccount:
				account = value.StringVal()
			case semconventions.AttributeCloudZone:
				zone = value.StringVal()
			case semconventions.AttributeHostID:
				hostID = value.StringVal()
			case semconventions.AttributeHostType:
				hostType = value.StringVal()
			case semconventions.AttributeHostImageID:
				amiID = value.StringVal()
			case semconventions.AttributeContainerName:
				if container == "" {
					container = value.StringVal()
				}
			case semconventions.AttributeK8sPod:
				container = value.StringVal()
			case semconventions.AttributeServiceNamespace:
				namespace = value.StringVal()
			case semconventions.AttributeServiceInstance:
				deployID = value.StringVal()
			case semconventions.AttributeServiceVersion:
				versionLabel = value.StringVal()
			}
		})
	}

	for key, value := range attributes {
		switch key {
		case awsxray.AWSOperationAttribute:
			operation = value
		case awsxray.AWSAccountAttribute:
			if value != "" {
				account = value
			}
		case awsxray.AWSRegionAttribute:
			remoteRegion = value
		case awsxray.AWSRequestIDAttribute:
			fallthrough
		case awsxray.AWSRequestIDAttribute2:
			requestID = value
		case awsxray.AWSQueueURLAttribute:
			fallthrough
		case awsxray.AWSQueueURLAttribute2:
			queueURL = value
		case awsxray.AWSTableNameAttribute:
			fallthrough
		case awsxray.AWSTableNameAttribute2:
			tableName = value
		default:
			filtered[key] = value
		}
	}
	if cloud != "aws" && cloud != "" {
		return filtered, nil // not AWS so return nil
	}
	// progress from least specific to most specific origin so most specific ends up as origin
	// as per X-Ray docs
	if hostID != "" {
		ec2 = &awsxray.EC2Metadata{
			InstanceID:       aws.String(hostID),
			AvailabilityZone: aws.String(zone),
			InstanceSize:     aws.String(hostType),
			AmiID:            aws.String(amiID),
		}
	}
	if container != "" {
		ecs = &awsxray.ECSMetadata{
			ContainerName: aws.String(container),
		}
	}
	if deployID != "" {
		deployNum, err := strconv.ParseInt(deployID, 10, 64)
		if err != nil {
			deployNum = 0
		}
		ebs = &awsxray.BeanstalkMetadata{
			Environment:  aws.String(namespace),
			DeploymentID: aws.Int64(deployNum),
			VersionLabel: aws.String(versionLabel),
		}
	}
	awsData := &awsxray.AWSData{
		AccountID:    aws.String(account),
		Beanstalk:    ebs,
		ECS:          ecs,
		EC2:          ec2,
		Operation:    aws.String(operation),
		RemoteRegion: aws.String(remoteRegion),
		RequestID:    aws.String(requestID),
		QueueURL:     aws.String(queueURL),
		TableName:    aws.String(tableName),
	}
	return filtered, awsData
}
