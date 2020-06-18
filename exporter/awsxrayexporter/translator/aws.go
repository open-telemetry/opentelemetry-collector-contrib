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

	"go.opentelemetry.io/collector/consumer/pdata"
	semconventions "go.opentelemetry.io/collector/translator/conventions"
)

// AWS-specific OpenTelemetry attribute names
const (
	AWSOperationAttribute = "aws.operation"
	AWSAccountAttribute   = "aws.account_id"
	AWSRegionAttribute    = "aws.region"
	AWSRequestIDAttribute = "aws.request_id"
	// Currently different instrumentation uses different tag formats.
	// TODO(anuraaga): Find current instrumentation and consolidate.
	AWSRequestIDAttribute2 = "aws.requestId"
	AWSQueueURLAttribute   = "aws.queue_url"
	AWSQueueURLAttribute2  = "aws.queue.url"
	AWSServiceAttribute    = "aws.service"
	AWSTableNameAttribute  = "aws.table_name"
	AWSTableNameAttribute2 = "aws.table.name"
)

// AWSData provides the shape for unmarshalling AWS resource data.
type AWSData struct {
	AccountID         string             `json:"account_id,omitempty"`
	BeanstalkMetadata *BeanstalkMetadata `json:"elastic_beanstalk,omitempty"`
	ECSMetadata       *ECSMetadata       `json:"ecs,omitempty"`
	EC2Metadata       *EC2Metadata       `json:"ec2,omitempty"`
	Operation         string             `json:"operation,omitempty"`
	RemoteRegion      string             `json:"region,omitempty"`
	RequestID         string             `json:"request_id,omitempty"`
	QueueURL          string             `json:"queue_url,omitempty"`
	TableName         string             `json:"table_name,omitempty"`
}

// EC2Metadata provides the shape for unmarshalling EC2 metadata.
type EC2Metadata struct {
	InstanceID       string `json:"instance_id"`
	AvailabilityZone string `json:"availability_zone"`
	InstanceSize     string `json:"instance_size"`
	AmiID            string `json:"ami_id"`
}

// ECSMetadata provides the shape for unmarshalling ECS metadata.
type ECSMetadata struct {
	ContainerName string `json:"container"`
}

// BeanstalkMetadata provides the shape for unmarshalling Elastic Beanstalk environment metadata.
type BeanstalkMetadata struct {
	Environment  string `json:"environment_name"`
	VersionLabel string `json:"version_label"`
	DeploymentID int64  `json:"deployment_id"`
}

func makeAws(attributes map[string]string, resource pdata.Resource) (map[string]string, *AWSData) {
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
		ec2          *EC2Metadata
		ecs          *ECSMetadata
		ebs          *BeanstalkMetadata
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
		case AWSOperationAttribute:
			operation = value
		case AWSAccountAttribute:
			if value != "" {
				account = value
			}
		case AWSRegionAttribute:
			remoteRegion = value
		case AWSRequestIDAttribute:
			fallthrough
		case AWSRequestIDAttribute2:
			requestID = value
		case AWSQueueURLAttribute:
			fallthrough
		case AWSQueueURLAttribute2:
			queueURL = value
		case AWSTableNameAttribute:
			fallthrough
		case AWSTableNameAttribute2:
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
		ec2 = &EC2Metadata{
			InstanceID:       hostID,
			AvailabilityZone: zone,
			InstanceSize:     hostType,
			AmiID:            amiID,
		}
	}
	if container != "" {
		ecs = &ECSMetadata{
			ContainerName: container,
		}
	}
	if deployID != "" {
		deployNum, err := strconv.ParseInt(deployID, 10, 64)
		if err != nil {
			deployNum = 0
		}
		ebs = &BeanstalkMetadata{
			Environment:  namespace,
			DeploymentID: deployNum,
			VersionLabel: versionLabel,
		}
	}
	awsData := &AWSData{
		AccountID:         account,
		BeanstalkMetadata: ebs,
		ECSMetadata:       ecs,
		EC2Metadata:       ec2,
		Operation:         operation,
		RemoteRegion:      remoteRegion,
		RequestID:         requestID,
		QueueURL:          queueURL,
		TableName:         tableName,
	}
	return filtered, awsData
}
