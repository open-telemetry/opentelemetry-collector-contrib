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

	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/pdata"
	semconventions "go.opentelemetry.io/collector/translator/conventions"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/awsxray"
)

func TestAwsFromEc2Resource(t *testing.T) {
	instanceID := "i-00f7c0bcb26da2a99"
	hostType := "m5.xlarge"
	imageID := "ami-0123456789"
	resource := pdata.NewResource()
	resource.InitEmpty()
	attrs := pdata.NewAttributeMap()
	attrs.InsertString(semconventions.AttributeCloudProvider, semconventions.AttributeCloudProviderAWS)
	attrs.InsertString(semconventions.AttributeCloudAccount, "123456789")
	attrs.InsertString(semconventions.AttributeCloudZone, "us-east-1c")
	attrs.InsertString(semconventions.AttributeHostID, instanceID)
	attrs.InsertString(semconventions.AttributeHostType, hostType)
	attrs.InsertString(semconventions.AttributeHostImageID, imageID)
	attrs.CopyTo(resource.Attributes())

	attributes := make(map[string]string)

	filtered, awsData := makeAws(attributes, resource)

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.NotNil(t, awsData.EC2)
	assert.Nil(t, awsData.ECS)
	assert.Nil(t, awsData.Beanstalk)
	assert.Nil(t, awsData.EKS)
	assert.Equal(t, "123456789", *awsData.AccountID)
	assert.Equal(t, &awsxray.EC2Metadata{
		InstanceID:       aws.String(instanceID),
		AvailabilityZone: aws.String("us-east-1c"),
		InstanceSize:     aws.String(hostType),
		AmiID:            aws.String(imageID),
	}, awsData.EC2)
}

func TestAwsFromEcsResource(t *testing.T) {
	instanceID := "i-00f7c0bcb26da2a99"
	containerName := "signup_aggregator-x82ufje83"
	containerID := "0123456789A"
	resource := pdata.NewResource()
	resource.InitEmpty()
	attrs := pdata.NewAttributeMap()
	attrs.InsertString(semconventions.AttributeCloudProvider, semconventions.AttributeCloudProviderAWS)
	attrs.InsertString(semconventions.AttributeCloudAccount, "123456789")
	attrs.InsertString(semconventions.AttributeCloudZone, "us-east-1c")
	attrs.InsertString(semconventions.AttributeContainerImage, "otel/signupaggregator")
	attrs.InsertString(semconventions.AttributeContainerTag, "v1")
	attrs.InsertString(semconventions.AttributeK8sCluster, "production")
	attrs.InsertString(semconventions.AttributeK8sNamespace, "default")
	attrs.InsertString(semconventions.AttributeK8sDeployment, "signup_aggregator")
	attrs.InsertString(semconventions.AttributeK8sPod, "my-deployment-65dcf7d447-ddjnl")
	attrs.InsertString(semconventions.AttributeContainerName, containerName)
	attrs.InsertString(semconventions.AttributeContainerID, containerID)
	attrs.InsertString(semconventions.AttributeHostID, instanceID)
	attrs.InsertString(semconventions.AttributeHostType, "m5.xlarge")
	attrs.CopyTo(resource.Attributes())

	attributes := make(map[string]string)

	filtered, awsData := makeAws(attributes, resource)

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.NotNil(t, awsData.EC2)
	assert.NotNil(t, awsData.ECS)
	assert.Nil(t, awsData.Beanstalk)
	assert.NotNil(t, awsData.EKS)
	assert.Equal(t, &awsxray.ECSMetadata{
		ContainerName: aws.String(containerName),
		ContainerID:   aws.String(containerID),
	}, awsData.ECS)
}

func TestAwsFromBeanstalkResource(t *testing.T) {
	deployID := "232"
	versionLabel := "4"
	resource := pdata.NewResource()
	resource.InitEmpty()
	attrs := pdata.NewAttributeMap()
	attrs.InsertString(semconventions.AttributeCloudProvider, semconventions.AttributeCloudProviderAWS)
	attrs.InsertString(semconventions.AttributeCloudAccount, "123456789")
	attrs.InsertString(semconventions.AttributeCloudZone, "us-east-1c")
	attrs.InsertString(semconventions.AttributeServiceNamespace, "production")
	attrs.InsertString(semconventions.AttributeServiceInstance, deployID)
	attrs.InsertString(semconventions.AttributeServiceVersion, versionLabel)
	attrs.CopyTo(resource.Attributes())

	attributes := make(map[string]string)

	filtered, awsData := makeAws(attributes, resource)

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.Nil(t, awsData.EC2)
	assert.Nil(t, awsData.ECS)
	assert.NotNil(t, awsData.Beanstalk)
	assert.Nil(t, awsData.EKS)
	assert.Equal(t, &awsxray.BeanstalkMetadata{
		Environment:  aws.String("production"),
		VersionLabel: aws.String(versionLabel),
		DeploymentID: aws.Int64(232),
	}, awsData.Beanstalk)
}

func TestAwsFromEksResource(t *testing.T) {
	instanceID := "i-00f7c0bcb26da2a99"
	containerName := "signup_aggregator-x82ufje83"
	containerID := "0123456789A"
	resource := pdata.NewResource()
	resource.InitEmpty()
	attrs := pdata.NewAttributeMap()
	attrs.InsertString(semconventions.AttributeCloudProvider, semconventions.AttributeCloudProviderAWS)
	attrs.InsertString(semconventions.AttributeCloudAccount, "123456789")
	attrs.InsertString(semconventions.AttributeCloudZone, "us-east-1c")
	attrs.InsertString(semconventions.AttributeContainerImage, "otel/signupaggregator")
	attrs.InsertString(semconventions.AttributeContainerTag, "v1")
	attrs.InsertString(semconventions.AttributeK8sCluster, "production")
	attrs.InsertString(semconventions.AttributeK8sNamespace, "default")
	attrs.InsertString(semconventions.AttributeK8sDeployment, "signup_aggregator")
	attrs.InsertString(semconventions.AttributeK8sPod, "my-deployment-65dcf7d447-ddjnl")
	attrs.InsertString(semconventions.AttributeContainerName, containerName)
	attrs.InsertString(semconventions.AttributeContainerID, containerID)
	attrs.InsertString(semconventions.AttributeHostID, instanceID)
	attrs.InsertString(semconventions.AttributeHostType, "m5.xlarge")
	attrs.CopyTo(resource.Attributes())

	attributes := make(map[string]string)

	filtered, awsData := makeAws(attributes, resource)

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.NotNil(t, awsData.EC2)
	assert.NotNil(t, awsData.ECS)
	assert.Nil(t, awsData.Beanstalk)
	assert.NotNil(t, awsData.EKS)
	assert.Equal(t, &awsxray.EKSMetadata{
		ClusterName: aws.String("production"),
		Pod:         aws.String("my-deployment-65dcf7d447-ddjnl"),
		ContainerID: aws.String(containerID),
	}, awsData.EKS)
}

func TestAwsWithAwsSqsResources(t *testing.T) {
	instanceID := "i-00f7c0bcb26da2a99"
	containerName := "signup_aggregator-x82ufje83"
	containerID := "0123456789A"
	resource := pdata.NewResource()
	resource.InitEmpty()
	attrs := pdata.NewAttributeMap()
	attrs.InsertString(semconventions.AttributeCloudProvider, semconventions.AttributeCloudProviderAWS)
	attrs.InsertString(semconventions.AttributeCloudAccount, "123456789")
	attrs.InsertString(semconventions.AttributeCloudZone, "us-east-1c")
	attrs.InsertString(semconventions.AttributeContainerName, containerName)
	attrs.InsertString(semconventions.AttributeContainerImage, "otel/signupaggregator")
	attrs.InsertString(semconventions.AttributeContainerTag, "v1")
	attrs.InsertString(semconventions.AttributeK8sCluster, "production")
	attrs.InsertString(semconventions.AttributeK8sNamespace, "default")
	attrs.InsertString(semconventions.AttributeK8sDeployment, "signup_aggregator")
	attrs.InsertString(semconventions.AttributeK8sPod, "my-deployment-65dcf7d447-ddjnl")
	attrs.InsertString(semconventions.AttributeContainerName, containerName)
	attrs.InsertString(semconventions.AttributeContainerID, containerID)
	attrs.InsertString(semconventions.AttributeHostID, instanceID)
	attrs.InsertString(semconventions.AttributeHostType, "m5.xlarge")

	queueURL := "https://sqs.use1.amazonaws.com/Meltdown-Alerts"
	attributes := make(map[string]string)
	attributes[awsxray.AWSOperationAttribute] = "SendMessage"
	attributes[awsxray.AWSAccountAttribute] = "987654321"
	attributes[awsxray.AWSRegionAttribute] = "us-east-2"
	attributes[awsxray.AWSQueueURLAttribute] = queueURL
	attributes["employee.id"] = "XB477"

	filtered, awsData := makeAws(attributes, resource)

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.Equal(t, queueURL, *awsData.QueueURL)
	assert.Equal(t, "us-east-2", *awsData.RemoteRegion)
}

func TestAwsWithSqsAlternateAttribute(t *testing.T) {
	queueURL := "https://sqs.use1.amazonaws.com/Meltdown-Alerts"
	attributes := make(map[string]string)
	attributes[awsxray.AWSQueueURLAttribute2] = queueURL

	filtered, awsData := makeAws(attributes, pdata.NewResource())

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.Equal(t, queueURL, *awsData.QueueURL)
}

func TestAwsWithAwsDynamoDbResources(t *testing.T) {
	instanceID := "i-00f7c0bcb26da2a99"
	containerName := "signup_aggregator-x82ufje83"
	containerID := "0123456789A"
	resource := pdata.NewResource()
	resource.InitEmpty()
	attrs := pdata.NewAttributeMap()
	attrs.InsertString(semconventions.AttributeCloudProvider, semconventions.AttributeCloudProviderAWS)
	attrs.InsertString(semconventions.AttributeCloudAccount, "123456789")
	attrs.InsertString(semconventions.AttributeCloudZone, "us-east-1c")
	attrs.InsertString(semconventions.AttributeContainerName, "signup_aggregator")
	attrs.InsertString(semconventions.AttributeContainerImage, "otel/signupaggregator")
	attrs.InsertString(semconventions.AttributeContainerTag, "v1")
	attrs.InsertString(semconventions.AttributeK8sCluster, "production")
	attrs.InsertString(semconventions.AttributeK8sNamespace, "default")
	attrs.InsertString(semconventions.AttributeK8sDeployment, "signup_aggregator")
	attrs.InsertString(semconventions.AttributeK8sPod, "my-deployment-65dcf7d447-ddjnl")
	attrs.InsertString(semconventions.AttributeContainerName, containerName)
	attrs.InsertString(semconventions.AttributeContainerID, containerID)
	attrs.InsertString(semconventions.AttributeHostID, instanceID)
	attrs.InsertString(semconventions.AttributeHostType, "m5.xlarge")

	tableName := "WIDGET_TYPES"
	attributes := make(map[string]string)
	attributes[awsxray.AWSOperationAttribute] = "PutItem"
	attributes[awsxray.AWSRequestIDAttribute] = "75107C82-EC8A-4F75-883F-4440B491B0AB"
	attributes[awsxray.AWSTableNameAttribute] = tableName

	filtered, awsData := makeAws(attributes, resource)

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.Equal(t, "PutItem", *awsData.Operation)
	assert.Equal(t, "75107C82-EC8A-4F75-883F-4440B491B0AB", *awsData.RequestID)
	assert.Equal(t, tableName, *awsData.TableName)
}

func TestAwsWithDynamoDbAlternateAttribute(t *testing.T) {
	tableName := "MyTable"
	attributes := make(map[string]string)
	attributes[awsxray.AWSTableNameAttribute2] = tableName

	filtered, awsData := makeAws(attributes, pdata.NewResource())

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.Equal(t, tableName, *awsData.TableName)
}

func TestAwsWithRequestIdAlternateAttribute(t *testing.T) {
	requestid := "12345-request"
	attributes := make(map[string]string)
	attributes[awsxray.AWSRequestIDAttribute2] = requestid

	filtered, awsData := makeAws(attributes, pdata.NewResource())

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.Equal(t, requestid, *awsData.RequestID)
}

func TestJavaSDK(t *testing.T) {
	attributes := make(map[string]string)
	resource := pdata.NewResource()
	resource.InitEmpty()
	resource.Attributes().InsertString(semconventions.AttributeTelemetrySDKName, "opentelemetry")
	resource.Attributes().InsertString(semconventions.AttributeTelemetrySDKLanguage, "java")
	resource.Attributes().InsertString(semconventions.AttributeTelemetrySDKVersion, "1.2.3")

	filtered, awsData := makeAws(attributes, resource)

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.Equal(t, "opentelemetry for java", *awsData.XRay.SDK)
	assert.Equal(t, "1.2.3", *awsData.XRay.SDKVersion)
}

func TestJavaAutoInstrumentation(t *testing.T) {
	attributes := make(map[string]string)
	resource := pdata.NewResource()
	resource.InitEmpty()
	resource.Attributes().InsertString(semconventions.AttributeTelemetrySDKName, "opentelemetry")
	resource.Attributes().InsertString(semconventions.AttributeTelemetrySDKLanguage, "java")
	resource.Attributes().InsertString(semconventions.AttributeTelemetrySDKVersion, "1.2.3")
	resource.Attributes().InsertString(semconventions.AttributeTelemetryAutoVersion, "3.4.5")

	filtered, awsData := makeAws(attributes, resource)

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.Equal(t, "opentelemetry for java", *awsData.XRay.SDK)
	assert.Equal(t, "1.2.3", *awsData.XRay.SDKVersion)
	assert.True(t, *awsData.XRay.AutoInstrumentation)
}

func TestGoSDK(t *testing.T) {
	attributes := make(map[string]string)
	resource := pdata.NewResource()
	resource.InitEmpty()
	resource.Attributes().InsertString(semconventions.AttributeTelemetrySDKName, "opentelemetry")
	resource.Attributes().InsertString(semconventions.AttributeTelemetrySDKLanguage, "go")
	resource.Attributes().InsertString(semconventions.AttributeTelemetrySDKVersion, "2.0.3")

	filtered, awsData := makeAws(attributes, resource)

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.Equal(t, "opentelemetry for go", *awsData.XRay.SDK)
	assert.Equal(t, "2.0.3", *awsData.XRay.SDKVersion)
}

func TestCustomSDK(t *testing.T) {
	attributes := make(map[string]string)
	resource := pdata.NewResource()
	resource.InitEmpty()
	resource.Attributes().InsertString(semconventions.AttributeTelemetrySDKName, "opentracing")
	resource.Attributes().InsertString(semconventions.AttributeTelemetrySDKLanguage, "java")
	resource.Attributes().InsertString(semconventions.AttributeTelemetrySDKVersion, "2.0.3")

	filtered, awsData := makeAws(attributes, resource)

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.Equal(t, "opentracing for java", *awsData.XRay.SDK)
	assert.Equal(t, "2.0.3", *awsData.XRay.SDKVersion)
}
