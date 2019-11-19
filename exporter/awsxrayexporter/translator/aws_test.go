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
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestAwsFromEc2Resource(t *testing.T) {
	instanceId := "i-00f7c0bcb26da2a99"
	labels := make(map[string]string)
	labels[CloudProviderAttribute] = "aws"
	labels[CloudAccountAttribute] = "123456789"
	labels[CloudZoneAttribute] = "us-east-1c"
	labels[HostIdAttribute] = instanceId
	labels[HostTypeAttribute] = "m5.xlarge"
	resource := &resourcepb.Resource{
		Type:   "vm",
		Labels: labels,
	}
	attributes := make(map[string]string)

	filtered, awsData := makeAws(attributes, resource)

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.NotNil(t, awsData.EC2Metadata)
	assert.Nil(t, awsData.ECSMetadata)
	assert.Nil(t, awsData.BeanstalkMetadata)
	w := borrow()
	if err := w.Encode(awsData); err != nil {
		assert.Fail(t, "invalid json")
	}
	jsonStr := w.String()
	release(w)
	assert.True(t, strings.Contains(jsonStr, instanceId))
}

func TestAwsFromEcsResource(t *testing.T) {
	instanceId := "i-00f7c0bcb26da2a99"
	containerId := "signup_aggregator-x82ufje83"
	labels := make(map[string]string)
	labels[CloudProviderAttribute] = "aws"
	labels[CloudAccountAttribute] = "123456789"
	labels[CloudZoneAttribute] = "us-east-1c"
	labels[ContainerNameAttribute] = "signup_aggregator"
	labels[ContainerImageAttribute] = "otel/signupaggregator"
	labels[ContainerTagAttribute] = "v1"
	labels[K8sClusterAttribute] = "production"
	labels[K8sNamespaceAttribute] = "default"
	labels[K8sDeploymentAttribute] = "signup_aggregator"
	labels[K8sPodAttribute] = containerId
	labels[HostIdAttribute] = instanceId
	labels[HostTypeAttribute] = "m5.xlarge"
	resource := &resourcepb.Resource{
		Type:   "container",
		Labels: labels,
	}
	attributes := make(map[string]string)

	filtered, awsData := makeAws(attributes, resource)

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.NotNil(t, awsData.EC2Metadata)
	assert.NotNil(t, awsData.ECSMetadata)
	assert.Nil(t, awsData.BeanstalkMetadata)
	w := borrow()
	if err := w.Encode(awsData); err != nil {
		assert.Fail(t, "invalid json")
	}
	jsonStr := w.String()
	release(w)
	assert.True(t, strings.Contains(jsonStr, containerId))
}

func TestAwsFromBeanstalkResource(t *testing.T) {
	deployId := "232"
	labels := make(map[string]string)
	labels[CloudProviderAttribute] = "aws"
	labels[CloudAccountAttribute] = "123456789"
	labels[CloudZoneAttribute] = "us-east-1c"
	labels[ServiceVersionAttribute] = "2.1.4"
	labels[ServiceNamespaceAttribute] = "production"
	labels[ServiceInstanceAttribute] = deployId
	resource := &resourcepb.Resource{
		Type:   "vm",
		Labels: labels,
	}
	attributes := make(map[string]string)

	filtered, awsData := makeAws(attributes, resource)

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.Nil(t, awsData.EC2Metadata)
	assert.Nil(t, awsData.ECSMetadata)
	assert.NotNil(t, awsData.BeanstalkMetadata)
	w := borrow()
	if err := w.Encode(awsData); err != nil {
		assert.Fail(t, "invalid json")
	}
	jsonStr := w.String()
	release(w)
	assert.True(t, strings.Contains(jsonStr, deployId))
}

func TestAwsWithAwsSqsResources(t *testing.T) {
	instanceId := "i-00f7c0bcb26da2a99"
	containerId := "signup_aggregator-x82ufje83"
	labels := make(map[string]string)
	labels[CloudProviderAttribute] = "aws"
	labels[CloudAccountAttribute] = "123456789"
	labels[CloudZoneAttribute] = "us-east-1c"
	labels[ContainerNameAttribute] = "signup_aggregator"
	labels[ContainerImageAttribute] = "otel/signupaggregator"
	labels[ContainerTagAttribute] = "v1"
	labels[K8sClusterAttribute] = "production"
	labels[K8sNamespaceAttribute] = "default"
	labels[K8sDeploymentAttribute] = "signup_aggregator"
	labels[K8sPodAttribute] = containerId
	labels[HostIdAttribute] = instanceId
	labels[HostTypeAttribute] = "m5.xlarge"
	resource := &resourcepb.Resource{
		Type:   "container",
		Labels: labels,
	}
	queueUrl := "https://sqs.use1.amazonaws.com/Meltdown-Alerts"
	attributes := make(map[string]string)
	attributes[AwsOperationAttribute] = "SendMessage"
	attributes[AwsAccountAttribute] = "987654321"
	attributes[AwsRegionAttribute] = "us-east-2"
	attributes[AwsQueueUrlAttribute] = queueUrl
	attributes["employee.id"] = "XB477"

	filtered, awsData := makeAws(attributes, resource)

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.NotNil(t, awsData.EC2Metadata)
	assert.NotNil(t, awsData.ECSMetadata)
	assert.Nil(t, awsData.BeanstalkMetadata)
	w := borrow()
	if err := w.Encode(awsData); err != nil {
		assert.Fail(t, "invalid json")
	}
	jsonStr := w.String()
	release(w)
	assert.True(t, strings.Contains(jsonStr, containerId))
	assert.True(t, strings.Contains(jsonStr, queueUrl))
}

func TestAwsWithAwsDynamoDbResources(t *testing.T) {
	instanceId := "i-00f7c0bcb26da2a99"
	containerId := "signup_aggregator-x82ufje83"
	labels := make(map[string]string)
	labels[CloudProviderAttribute] = "aws"
	labels[CloudAccountAttribute] = "123456789"
	labels[CloudZoneAttribute] = "us-east-1c"
	labels[ContainerNameAttribute] = "signup_aggregator"
	labels[ContainerImageAttribute] = "otel/signupaggregator"
	labels[ContainerTagAttribute] = "v1"
	labels[K8sClusterAttribute] = "production"
	labels[K8sNamespaceAttribute] = "default"
	labels[K8sDeploymentAttribute] = "signup_aggregator"
	labels[K8sPodAttribute] = containerId
	labels[HostIdAttribute] = instanceId
	labels[HostTypeAttribute] = "m5.xlarge"
	resource := &resourcepb.Resource{
		Type:   "container",
		Labels: labels,
	}
	tableName := "WIDGET_TYPES"
	attributes := make(map[string]string)
	attributes[AwsOperationAttribute] = "PutItem"
	attributes[AwsRequestIdAttribute] = "75107C82-EC8A-4F75-883F-4440B491B0AB"
	attributes[AwsTableNameAttribute] = tableName

	filtered, awsData := makeAws(attributes, resource)

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.NotNil(t, awsData.EC2Metadata)
	assert.NotNil(t, awsData.ECSMetadata)
	assert.Nil(t, awsData.BeanstalkMetadata)
	w := borrow()
	if err := w.Encode(awsData); err != nil {
		assert.Fail(t, "invalid json")
	}
	jsonStr := w.String()
	release(w)
	assert.True(t, strings.Contains(jsonStr, containerId))
	assert.True(t, strings.Contains(jsonStr, tableName))
}
