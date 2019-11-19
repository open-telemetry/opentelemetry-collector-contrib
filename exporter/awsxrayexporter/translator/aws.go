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
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"strconv"
)

const (
	AwsOperationAttribute = "aws.operation"
	AwsAccountAttribute   = "aws.account_id"
	AwsRegionAttribute    = "aws.region"
	AwsRequestIdAttribute = "aws.request_id"
	AwsQueueUrlAttribute  = "aws.queue_url"
	AwsTableNameAttribute = "aws.table_name"
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

func makeAws(attributes map[string]string, resource *resourcepb.Resource) (map[string]string, *AWSData) {
	var (
		cloud        string
		account      string
		zone         string
		hostId       string
		container    string
		namespace    string
		deployId     string
		ver          string
		origin       string
		operation    string
		remoteRegion string
		requestId    string
		queueUrl     string
		tableName    string
		ec2          *EC2Metadata
		ecs          *ECSMetadata
		ebs          *BeanstalkMetadata
		awsData      *AWSData
		filtered     map[string]string
	)
	if resource == nil {
		return attributes, awsData
	}
	filtered = make(map[string]string)
	for key, value := range resource.Labels {
		switch key {
		case CloudProviderAttribute:
			cloud = value
		case CloudAccountAttribute:
			account = value
		case CloudZoneAttribute:
			zone = value
		case HostIdAttribute:
			hostId = value
		case ContainerNameAttribute:
			if container == "" {
				container = value
			}
		case K8sPodAttribute:
			container = value
		case ServiceNamespaceAttribute:
			namespace = value
		case ServiceInstanceAttribute:
			deployId = value
		case ServiceVersionAttribute:
			ver = value
		}
	}
	for key, value := range attributes {
		switch key {
		case AwsOperationAttribute:
			operation = value
		case AwsAccountAttribute:
			if value != "" {
				account = value
			}
		case AwsRegionAttribute:
			remoteRegion = value
		case AwsRequestIdAttribute:
			requestId = value
		case AwsQueueUrlAttribute:
			queueUrl = value
		case AwsTableNameAttribute:
			tableName = value
		default:
			filtered[key] = value
		}
	}
	if cloud != "aws" && cloud != "" {
		return filtered, awsData // not AWS so return nil
	}
	// progress from least specific to most specific origin so most specific ends up as origin
	// as per X-Ray docs
	if hostId != "" {
		origin = OriginEC2
		ec2 = &EC2Metadata{
			InstanceID:       hostId,
			AvailabilityZone: zone,
		}
	}
	if container != "" {
		origin = OriginECS
		ecs = &ECSMetadata{
			ContainerName: container,
		}
	}
	if deployId != "" {
		origin = OriginEB
		deployNum, err := strconv.ParseInt(deployId, 10, 64)
		if err != nil {
			deployNum = 0
		}
		ebs = &BeanstalkMetadata{
			Environment:  namespace,
			VersionLabel: ver,
			DeploymentID: deployNum,
		}
	}
	if origin == "" {
		return filtered, awsData
	}
	awsData = &AWSData{
		AccountID:         account,
		BeanstalkMetadata: ebs,
		ECSMetadata:       ecs,
		EC2Metadata:       ec2,
		Operation:         operation,
		RemoteRegion:      remoteRegion,
		RequestID:         requestId,
		QueueURL:          queueUrl,
		TableName:         tableName,
	}
	return filtered, awsData
}
