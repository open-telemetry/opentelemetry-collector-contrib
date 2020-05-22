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
	"testing"

	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"github.com/stretchr/testify/assert"
	semconventions "go.opentelemetry.io/collector/translator/conventions"
)

func TestAwsFromEc2Resource(t *testing.T) {
	instanceID := "i-00f7c0bcb26da2a99"
	labels := make(map[string]string)
	labels[semconventions.AttributeCloudProvider] = "aws"
	labels[semconventions.AttributeCloudAccount] = "123456789"
	labels[semconventions.AttributeCloudZone] = "us-east-1c"
	labels[semconventions.AttributeHostID] = instanceID
	labels[semconventions.AttributeHostType] = "m5.xlarge"
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
	assert.Equal(t, "123456789", awsData.AccountID)
	assert.Equal(t, &EC2Metadata{
		InstanceID:       instanceID,
		AvailabilityZone: "us-east-1c",
	}, awsData.EC2Metadata)
}

func TestAwsFromEcsResource(t *testing.T) {
	instanceID := "i-00f7c0bcb26da2a99"
	containerID := "signup_aggregator-x82ufje83"
	labels := make(map[string]string)
	labels[semconventions.AttributeCloudProvider] = "aws"
	labels[semconventions.AttributeCloudAccount] = "123456789"
	labels[semconventions.AttributeCloudZone] = "us-east-1c"
	labels[semconventions.AttributeContainerName] = "signup_aggregator"
	labels[semconventions.AttributeContainerImage] = "otel/signupaggregator"
	labels[semconventions.AttributeContainerTag] = "v1"
	labels[semconventions.AttributeK8sCluster] = "production"
	labels[semconventions.AttributeK8sNamespace] = "default"
	labels[semconventions.AttributeK8sDeployment] = "signup_aggregator"
	labels[semconventions.AttributeK8sPod] = containerID
	labels[semconventions.AttributeHostID] = instanceID
	labels[semconventions.AttributeHostType] = "m5.xlarge"
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
	assert.Equal(t, &ECSMetadata{
		ContainerName: containerID,
	}, awsData.ECSMetadata)
}

func TestAwsFromBeanstalkResource(t *testing.T) {
	deployID := "232"
	labels := make(map[string]string)
	labels[semconventions.AttributeCloudProvider] = "aws"
	labels[semconventions.AttributeCloudAccount] = "123456789"
	labels[semconventions.AttributeCloudZone] = "us-east-1c"
	labels[semconventions.AttributeServiceNamespace] = "production"
	labels[semconventions.AttributeServiceInstance] = deployID
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
	assert.Equal(t, &BeanstalkMetadata{
		Environment:  "production",
		VersionLabel: "",
		DeploymentID: 232,
	}, awsData.BeanstalkMetadata)
}

func TestAwsWithAwsSqsResources(t *testing.T) {
	instanceID := "i-00f7c0bcb26da2a99"
	containerID := "signup_aggregator-x82ufje83"
	labels := make(map[string]string)
	labels[semconventions.AttributeCloudProvider] = "aws"
	labels[semconventions.AttributeCloudAccount] = "123456789"
	labels[semconventions.AttributeCloudZone] = "us-east-1c"
	labels[semconventions.AttributeContainerName] = "signup_aggregator"
	labels[semconventions.AttributeContainerImage] = "otel/signupaggregator"
	labels[semconventions.AttributeContainerTag] = "v1"
	labels[semconventions.AttributeK8sCluster] = "production"
	labels[semconventions.AttributeK8sNamespace] = "default"
	labels[semconventions.AttributeK8sDeployment] = "signup_aggregator"
	labels[semconventions.AttributeK8sPod] = containerID
	labels[semconventions.AttributeHostID] = instanceID
	labels[semconventions.AttributeHostType] = "m5.xlarge"
	resource := &resourcepb.Resource{
		Type:   "container",
		Labels: labels,
	}
	queueURL := "https://sqs.use1.amazonaws.com/Meltdown-Alerts"
	attributes := make(map[string]string)
	attributes[AWSOperationAttribute] = "SendMessage"
	attributes[AWSAccountAttribute] = "987654321"
	attributes[AWSRegionAttribute] = "us-east-2"
	attributes[AWSQueueURLAttribute] = queueURL
	attributes["employee.id"] = "XB477"

	filtered, awsData := makeAws(attributes, resource)

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.Equal(t, queueURL, awsData.QueueURL)
	assert.Equal(t, "us-east-2", awsData.RemoteRegion)
}

func TestAwsWithSqsAlternateAttribute(t *testing.T) {
	queueURL := "https://sqs.use1.amazonaws.com/Meltdown-Alerts"
	attributes := make(map[string]string)
	attributes[AWSQueueURLAttribute2] = queueURL

	filtered, awsData := makeAws(attributes, nil)

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.Equal(t, queueURL, awsData.QueueURL)
}

func TestAwsWithAwsDynamoDbResources(t *testing.T) {
	instanceID := "i-00f7c0bcb26da2a99"
	containerID := "signup_aggregator-x82ufje83"
	labels := make(map[string]string)
	labels[semconventions.AttributeCloudProvider] = "aws"
	labels[semconventions.AttributeCloudAccount] = "123456789"
	labels[semconventions.AttributeCloudZone] = "us-east-1c"
	labels[semconventions.AttributeContainerName] = "signup_aggregator"
	labels[semconventions.AttributeContainerImage] = "otel/signupaggregator"
	labels[semconventions.AttributeContainerTag] = "v1"
	labels[semconventions.AttributeK8sCluster] = "production"
	labels[semconventions.AttributeK8sNamespace] = "default"
	labels[semconventions.AttributeK8sDeployment] = "signup_aggregator"
	labels[semconventions.AttributeK8sPod] = containerID
	labels[semconventions.AttributeHostID] = instanceID
	labels[semconventions.AttributeHostType] = "m5.xlarge"
	resource := &resourcepb.Resource{
		Type:   "container",
		Labels: labels,
	}
	tableName := "WIDGET_TYPES"
	attributes := make(map[string]string)
	attributes[AWSOperationAttribute] = "PutItem"
	attributes[AWSRequestIDAttribute] = "75107C82-EC8A-4F75-883F-4440B491B0AB"
	attributes[AWSTableNameAttribute] = tableName

	filtered, awsData := makeAws(attributes, resource)

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.Equal(t, "PutItem", awsData.Operation)
	assert.Equal(t, "75107C82-EC8A-4F75-883F-4440B491B0AB", awsData.RequestID)
	assert.Equal(t, tableName, awsData.TableName)
}

func TestAwsWithDynamoDbAlternateAttribute(t *testing.T) {
	tableName := "MyTable"
	attributes := make(map[string]string)
	attributes[AWSTableNameAttribute2] = tableName

	filtered, awsData := makeAws(attributes, nil)

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.Equal(t, tableName, awsData.TableName)
}

func TestAwsWithRequestIdAlternateAttribute(t *testing.T) {
	requestid := "12345-request"
	attributes := make(map[string]string)
	attributes[AWSRequestIDAttribute2] = requestid

	filtered, awsData := makeAws(attributes, nil)

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.Equal(t, requestid, awsData.RequestID)
}
