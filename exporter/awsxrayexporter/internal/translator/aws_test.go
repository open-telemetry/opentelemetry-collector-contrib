// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventionsv112 "go.opentelemetry.io/collector/semconv/v1.12.0"
	conventions "go.opentelemetry.io/collector/semconv/v1.27.0"

	awsxray "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"
)

func TestAwsFromEc2Resource(t *testing.T) {
	instanceID := "i-00f7c0bcb26da2a99"
	hostType := "m5.xlarge"
	imageID := "ami-0123456789"
	resource := pcommon.NewResource()
	attrs := pcommon.NewMap()
	attrs.PutStr(conventionsv112.AttributeCloudProvider, conventionsv112.AttributeCloudProviderAWS)
	attrs.PutStr(conventionsv112.AttributeCloudPlatform, conventionsv112.AttributeCloudPlatformAWSEC2)
	attrs.PutStr(conventionsv112.AttributeCloudAccountID, "123456789")
	attrs.PutStr(conventionsv112.AttributeCloudAvailabilityZone, "us-east-1c")
	attrs.PutStr(conventionsv112.AttributeHostID, instanceID)
	attrs.PutStr(conventionsv112.AttributeHostType, hostType)
	attrs.PutStr(conventionsv112.AttributeHostImageID, imageID)
	attrs.CopyTo(resource.Attributes())

	attributes := make(map[string]pcommon.Value)

	filtered, awsData := makeAws(attributes, resource, nil)

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
	az := "us-east-1c"
	launchType := "fargate"
	family := "family"
	taskArn := "arn:aws:ecs:us-west-2:123456789123:task/123"
	clusterArn := "arn:aws:ecs:us-west-2:123456789123:cluster/my-cluster"
	containerArn := "arn:aws:ecs:us-west-2:123456789123:container-instance/123"
	resource := pcommon.NewResource()
	attrs := pcommon.NewMap()
	attrs.PutStr(conventionsv112.AttributeCloudProvider, conventionsv112.AttributeCloudProviderAWS)
	attrs.PutStr(conventionsv112.AttributeCloudPlatform, conventionsv112.AttributeCloudPlatformAWSECS)
	attrs.PutStr(conventionsv112.AttributeCloudAccountID, "123456789")
	attrs.PutStr(conventionsv112.AttributeCloudAvailabilityZone, az)
	attrs.PutStr(conventionsv112.AttributeContainerImageName, "otel/signupaggregator")
	attrs.PutStr(conventionsv112.AttributeContainerImageTag, "v1")
	attrs.PutStr(conventionsv112.AttributeContainerName, containerName)
	attrs.PutStr(conventionsv112.AttributeContainerID, containerID)
	attrs.PutStr(conventionsv112.AttributeHostID, instanceID)
	attrs.PutStr(conventionsv112.AttributeAWSECSClusterARN, clusterArn)
	attrs.PutStr(conventionsv112.AttributeAWSECSContainerARN, containerArn)
	attrs.PutStr(conventionsv112.AttributeAWSECSTaskARN, taskArn)
	attrs.PutStr(conventionsv112.AttributeAWSECSTaskFamily, family)
	attrs.PutStr(conventionsv112.AttributeAWSECSLaunchtype, launchType)
	attrs.PutStr(conventionsv112.AttributeHostType, "m5.xlarge")

	attrs.CopyTo(resource.Attributes())

	attributes := make(map[string]pcommon.Value)

	filtered, awsData := makeAws(attributes, resource, nil)

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.NotNil(t, awsData.ECS)
	assert.NotNil(t, awsData.EC2)
	assert.Nil(t, awsData.Beanstalk)
	assert.Nil(t, awsData.EKS)
	assert.Equal(t, &awsxray.ECSMetadata{
		ContainerName:    aws.String(containerName),
		ContainerID:      aws.String(containerID),
		AvailabilityZone: aws.String(az),
		ClusterArn:       aws.String(clusterArn),
		ContainerArn:     aws.String(containerArn),
		TaskArn:          aws.String(taskArn),
		TaskFamily:       aws.String(family),
		LaunchType:       aws.String(launchType),
	}, awsData.ECS)
}

func TestAwsFromBeanstalkResource(t *testing.T) {
	deployID := "232"
	versionLabel := "4"
	resource := pcommon.NewResource()
	attrs := pcommon.NewMap()
	attrs.PutStr(conventionsv112.AttributeCloudProvider, conventionsv112.AttributeCloudProviderAWS)
	attrs.PutStr(conventionsv112.AttributeCloudPlatform, conventionsv112.AttributeCloudPlatformAWSElasticBeanstalk)
	attrs.PutStr(conventionsv112.AttributeCloudAccountID, "123456789")
	attrs.PutStr(conventionsv112.AttributeCloudAvailabilityZone, "us-east-1c")
	attrs.PutStr(conventionsv112.AttributeServiceNamespace, "production")
	attrs.PutStr(conventionsv112.AttributeServiceInstanceID, deployID)
	attrs.PutStr(conventionsv112.AttributeServiceVersion, versionLabel)
	attrs.CopyTo(resource.Attributes())

	attributes := make(map[string]pcommon.Value)

	filtered, awsData := makeAws(attributes, resource, nil)

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
	resource := pcommon.NewResource()
	attrs := pcommon.NewMap()
	attrs.PutStr(conventionsv112.AttributeCloudProvider, conventionsv112.AttributeCloudProviderAWS)
	attrs.PutStr(conventionsv112.AttributeCloudPlatform, conventionsv112.AttributeCloudPlatformAWSEKS)
	attrs.PutStr(conventionsv112.AttributeCloudAccountID, "123456789")
	attrs.PutStr(conventionsv112.AttributeCloudAvailabilityZone, "us-east-1c")
	attrs.PutStr(conventionsv112.AttributeContainerImageName, "otel/signupaggregator")
	attrs.PutStr(conventionsv112.AttributeContainerImageTag, "v1")
	attrs.PutStr(conventionsv112.AttributeK8SClusterName, "production")
	attrs.PutStr(conventionsv112.AttributeK8SNamespaceName, "default")
	attrs.PutStr(conventionsv112.AttributeK8SDeploymentName, "signup_aggregator")
	attrs.PutStr(conventionsv112.AttributeK8SPodName, "my-deployment-65dcf7d447-ddjnl")
	attrs.PutStr(conventionsv112.AttributeContainerName, containerName)
	attrs.PutStr(conventionsv112.AttributeContainerID, containerID)
	attrs.PutStr(conventionsv112.AttributeHostID, instanceID)
	attrs.PutStr(conventionsv112.AttributeHostType, "m5.xlarge")
	attrs.CopyTo(resource.Attributes())

	attributes := make(map[string]pcommon.Value)

	filtered, awsData := makeAws(attributes, resource, nil)

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.NotNil(t, awsData.EKS)
	assert.NotNil(t, awsData.EC2)
	assert.Nil(t, awsData.ECS)
	assert.Nil(t, awsData.Beanstalk)
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
	resource := pcommon.NewResource()
	attrs := pcommon.NewMap()
	attrs.PutStr(conventionsv112.AttributeCloudProvider, conventionsv112.AttributeCloudProviderAWS)
	attrs.PutStr(conventionsv112.AttributeCloudAccountID, "123456789")
	attrs.PutStr(conventionsv112.AttributeCloudAvailabilityZone, "us-east-1c")
	attrs.PutStr(conventionsv112.AttributeContainerName, containerName)
	attrs.PutStr(conventionsv112.AttributeContainerImageName, "otel/signupaggregator")
	attrs.PutStr(conventionsv112.AttributeContainerImageTag, "v1")
	attrs.PutStr(conventionsv112.AttributeK8SClusterName, "production")
	attrs.PutStr(conventionsv112.AttributeK8SNamespaceName, "default")
	attrs.PutStr(conventionsv112.AttributeK8SDeploymentName, "signup_aggregator")
	attrs.PutStr(conventionsv112.AttributeK8SPodName, "my-deployment-65dcf7d447-ddjnl")
	attrs.PutStr(conventionsv112.AttributeContainerName, containerName)
	attrs.PutStr(conventionsv112.AttributeContainerID, containerID)
	attrs.PutStr(conventionsv112.AttributeHostID, instanceID)
	attrs.PutStr(conventionsv112.AttributeHostType, "m5.xlarge")

	queueURL := "https://sqs.use1.amazonaws.com/Meltdown-Alerts"
	attributes := make(map[string]pcommon.Value)
	attributes[awsxray.AWSOperationAttribute] = pcommon.NewValueStr("SendMessage")
	attributes[awsxray.AWSAccountAttribute] = pcommon.NewValueStr("987654321")
	attributes[awsxray.AWSRegionAttribute] = pcommon.NewValueStr("us-east-2")
	attributes[awsxray.AWSQueueURLAttribute] = pcommon.NewValueStr(queueURL)
	attributes["employee.id"] = pcommon.NewValueStr("XB477")

	filtered, awsData := makeAws(attributes, resource, nil)

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.Equal(t, queueURL, *awsData.QueueURL)
	assert.Equal(t, "us-east-2", *awsData.RemoteRegion)
}

func TestAwsWithRpcAttributes(t *testing.T) {
	resource := pcommon.NewResource()
	attributes := make(map[string]pcommon.Value)
	attributes[conventionsv112.AttributeRPCMethod] = pcommon.NewValueStr("ListBuckets")

	_, awsData := makeAws(attributes, resource, nil)

	assert.NotNil(t, awsData)
	assert.Equal(t, "ListBuckets", *awsData.Operation)
}

func TestAwsWithSqsAlternateAttribute(t *testing.T) {
	queueURL := "https://sqs.use1.amazonaws.com/Meltdown-Alerts"
	attributes := make(map[string]pcommon.Value)
	attributes[awsxray.AWSQueueURLAttribute2] = pcommon.NewValueStr(queueURL)

	filtered, awsData := makeAws(attributes, pcommon.NewResource(), nil)

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.Equal(t, queueURL, *awsData.QueueURL)
}

func TestAwsWithAwsSqsSemConvAttributes(t *testing.T) {
	queueURL := "https://sqs.use1.amazonaws.com/Meltdown-Alerts"
	attributes := make(map[string]pcommon.Value)
	attributes[conventionsv112.AttributeMessagingURL] = pcommon.NewValueStr(queueURL)

	filtered, awsData := makeAws(attributes, pcommon.NewResource(), nil)

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.Equal(t, queueURL, *awsData.QueueURL)
}

func TestAwsWithAwsDynamoDbResources(t *testing.T) {
	instanceID := "i-00f7c0bcb26da2a99"
	containerName := "signup_aggregator-x82ufje83"
	containerID := "0123456789A"
	resource := pcommon.NewResource()
	attrs := pcommon.NewMap()
	attrs.PutStr(conventionsv112.AttributeCloudProvider, conventionsv112.AttributeCloudProviderAWS)
	attrs.PutStr(conventionsv112.AttributeCloudAccountID, "123456789")
	attrs.PutStr(conventionsv112.AttributeCloudAvailabilityZone, "us-east-1c")
	attrs.PutStr(conventionsv112.AttributeContainerName, "signup_aggregator")
	attrs.PutStr(conventionsv112.AttributeContainerImageName, "otel/signupaggregator")
	attrs.PutStr(conventionsv112.AttributeContainerImageTag, "v1")
	attrs.PutStr(conventionsv112.AttributeK8SClusterName, "production")
	attrs.PutStr(conventionsv112.AttributeK8SNamespaceName, "default")
	attrs.PutStr(conventionsv112.AttributeK8SDeploymentName, "signup_aggregator")
	attrs.PutStr(conventionsv112.AttributeK8SPodName, "my-deployment-65dcf7d447-ddjnl")
	attrs.PutStr(conventionsv112.AttributeContainerName, containerName)
	attrs.PutStr(conventionsv112.AttributeContainerID, containerID)
	attrs.PutStr(conventionsv112.AttributeHostID, instanceID)
	attrs.PutStr(conventionsv112.AttributeHostType, "m5.xlarge")

	tableName := "WIDGET_TYPES"
	attributes := make(map[string]pcommon.Value)
	attributes[conventionsv112.AttributeRPCMethod] = pcommon.NewValueStr("IncorrectAWSSDKOperation")
	attributes[awsxray.AWSOperationAttribute] = pcommon.NewValueStr("PutItem")
	attributes[awsxray.AWSRequestIDAttribute] = pcommon.NewValueStr("75107C82-EC8A-4F75-883F-4440B491B0AB")
	attributes[awsxray.AWSTableNameAttribute] = pcommon.NewValueStr(tableName)

	filtered, awsData := makeAws(attributes, resource, nil)

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.Equal(t, "PutItem", *awsData.Operation)
	assert.Equal(t, "75107C82-EC8A-4F75-883F-4440B491B0AB", *awsData.RequestID)
	assert.Equal(t, tableName, *awsData.TableName)
}

func TestAwsWithDynamoDbAlternateAttribute(t *testing.T) {
	tableName := "MyTable"
	attributes := make(map[string]pcommon.Value)
	attributes[awsxray.AWSTableNameAttribute2] = pcommon.NewValueStr(tableName)

	filtered, awsData := makeAws(attributes, pcommon.NewResource(), nil)

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.Equal(t, tableName, *awsData.TableName)
}

func TestAwsWithDynamoDbSemConvAttributes(t *testing.T) {
	tableName := "MyTable"
	attributes := make(map[string]pcommon.Value)
	attributes[conventionsv112.AttributeAWSDynamoDBTableNames] = pcommon.NewValueSlice()
	attributes[conventionsv112.AttributeAWSDynamoDBTableNames].Slice().AppendEmpty().SetStr(tableName)

	filtered, awsData := makeAws(attributes, pcommon.NewResource(), nil)

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.Equal(t, tableName, *awsData.TableName)
}

func TestAwsWithDynamoDbSemConvAttributesString(t *testing.T) {
	tableName := "MyTable"
	attributes := make(map[string]pcommon.Value)
	attributes[conventionsv112.AttributeAWSDynamoDBTableNames] = pcommon.NewValueStr(tableName)

	filtered, awsData := makeAws(attributes, pcommon.NewResource(), nil)

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.Equal(t, tableName, *awsData.TableName)
}

func TestAwsWithRequestIdAlternateAttribute(t *testing.T) {
	requestid := "12345-request"
	attributes := make(map[string]pcommon.Value)
	attributes[awsxray.AWSRequestIDAttribute2] = pcommon.NewValueStr(requestid)

	filtered, awsData := makeAws(attributes, pcommon.NewResource(), nil)

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.Equal(t, requestid, *awsData.RequestID)
}

func TestJavaSDK(t *testing.T) {
	attributes := make(map[string]pcommon.Value)
	resource := pcommon.NewResource()
	resource.Attributes().PutStr(conventionsv112.AttributeTelemetrySDKName, "opentelemetry")
	resource.Attributes().PutStr(conventionsv112.AttributeTelemetrySDKLanguage, "java")
	resource.Attributes().PutStr(conventionsv112.AttributeTelemetrySDKVersion, "1.2.3")

	filtered, awsData := makeAws(attributes, resource, nil)

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.Equal(t, "opentelemetry for java", *awsData.XRay.SDK)
	assert.Equal(t, "1.2.3", *awsData.XRay.SDKVersion)
}

func TestJavaAutoInstrumentation(t *testing.T) {
	attributes := make(map[string]pcommon.Value)
	resource := pcommon.NewResource()
	resource.Attributes().PutStr(conventionsv112.AttributeTelemetrySDKName, "opentelemetry")
	resource.Attributes().PutStr(conventionsv112.AttributeTelemetrySDKLanguage, "java")
	resource.Attributes().PutStr(conventionsv112.AttributeTelemetrySDKVersion, "1.2.3")
	resource.Attributes().PutStr(conventionsv112.AttributeTelemetryAutoVersion, "3.4.5")

	filtered, awsData := makeAws(attributes, resource, nil)

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.Equal(t, "opentelemetry for java", *awsData.XRay.SDK)
	assert.Equal(t, "1.2.3", *awsData.XRay.SDKVersion)
	assert.True(t, *awsData.XRay.AutoInstrumentation)
}

func TestJavaAutoInstrumentationStable(t *testing.T) {
	attributes := make(map[string]pcommon.Value)
	resource := pcommon.NewResource()
	resource.Attributes().PutStr(conventions.AttributeTelemetrySDKName, "opentelemetry")
	resource.Attributes().PutStr(conventions.AttributeTelemetrySDKLanguage, "java")
	resource.Attributes().PutStr(conventions.AttributeTelemetrySDKVersion, "1.2.3")
	resource.Attributes().PutStr(conventions.AttributeTelemetryDistroVersion, "3.4.5")

	filtered, awsData := makeAws(attributes, resource, nil)

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.Equal(t, "opentelemetry for java", *awsData.XRay.SDK)
	assert.Equal(t, "1.2.3", *awsData.XRay.SDKVersion)
	assert.True(t, *awsData.XRay.AutoInstrumentation)
}

func TestGoSDK(t *testing.T) {
	attributes := make(map[string]pcommon.Value)
	resource := pcommon.NewResource()
	resource.Attributes().PutStr(conventionsv112.AttributeTelemetrySDKName, "opentelemetry")
	resource.Attributes().PutStr(conventionsv112.AttributeTelemetrySDKLanguage, "go")
	resource.Attributes().PutStr(conventionsv112.AttributeTelemetrySDKVersion, "2.0.3")

	filtered, awsData := makeAws(attributes, resource, nil)

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.Equal(t, "opentelemetry for go", *awsData.XRay.SDK)
	assert.Equal(t, "2.0.3", *awsData.XRay.SDKVersion)
}

func TestCustomSDK(t *testing.T) {
	attributes := make(map[string]pcommon.Value)
	resource := pcommon.NewResource()
	resource.Attributes().PutStr(conventionsv112.AttributeTelemetrySDKName, "opentracing")
	resource.Attributes().PutStr(conventionsv112.AttributeTelemetrySDKLanguage, "java")
	resource.Attributes().PutStr(conventionsv112.AttributeTelemetrySDKVersion, "2.0.3")

	filtered, awsData := makeAws(attributes, resource, nil)

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.Equal(t, "opentracing for java", *awsData.XRay.SDK)
	assert.Equal(t, "2.0.3", *awsData.XRay.SDKVersion)
}

func TestLogGroups(t *testing.T) {
	cwl1 := awsxray.LogGroupMetadata{
		LogGroup: awsxray.String("group1"),
	}
	cwl2 := awsxray.LogGroupMetadata{
		LogGroup: awsxray.String("group2"),
	}

	attributes := make(map[string]pcommon.Value)
	resource := pcommon.NewResource()
	ava := resource.Attributes().PutEmptySlice(conventionsv112.AttributeAWSLogGroupNames)
	ava.EnsureCapacity(2)
	ava.AppendEmpty().SetStr("group1")
	ava.AppendEmpty().SetStr("group2")

	filtered, awsData := makeAws(attributes, resource, nil)

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.Len(t, awsData.CWLogs, 2)
	assert.Contains(t, awsData.CWLogs, cwl1)
	assert.Contains(t, awsData.CWLogs, cwl2)
}

func TestLogGroupsFromArns(t *testing.T) {
	group1 := "arn:aws:logs:us-east-1:123456789123:log-group:group1"
	cwl1 := awsxray.LogGroupMetadata{
		LogGroup: awsxray.String("group1"),
		Arn:      awsxray.String(group1),
	}
	group2 := "arn:aws:logs:us-east-1:123456789123:log-group:group2:*"
	cwl2 := awsxray.LogGroupMetadata{
		LogGroup: awsxray.String("group2"),
		Arn:      awsxray.String(group2),
	}

	attributes := make(map[string]pcommon.Value)
	resource := pcommon.NewResource()
	ava := resource.Attributes().PutEmptySlice(conventionsv112.AttributeAWSLogGroupARNs)
	ava.EnsureCapacity(2)
	ava.AppendEmpty().SetStr(group1)
	ava.AppendEmpty().SetStr(group2)

	filtered, awsData := makeAws(attributes, resource, nil)

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.Len(t, awsData.CWLogs, 2)
	assert.Contains(t, awsData.CWLogs, cwl1)
	assert.Contains(t, awsData.CWLogs, cwl2)
}

// Simulate Log groups being set using OTEL_RESOURCE_ATTRIBUTES
func TestLogGroupsFromStringResourceAttribute(t *testing.T) {
	cwl1 := awsxray.LogGroupMetadata{
		LogGroup: awsxray.String("group1"),
	}

	attributes := make(map[string]pcommon.Value)
	resource := pcommon.NewResource()
	resource.Attributes().PutStr(conventionsv112.AttributeAWSLogGroupNames, "group1")

	filtered, awsData := makeAws(attributes, resource, nil)

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.Len(t, awsData.CWLogs, 1)
	assert.Contains(t, awsData.CWLogs, cwl1)
}

func TestLogGroupsWithAmpersandFromStringResourceAttribute(t *testing.T) {
	cwl1 := awsxray.LogGroupMetadata{
		LogGroup: awsxray.String("group1"),
	}
	cwl2 := awsxray.LogGroupMetadata{
		LogGroup: awsxray.String("group2"),
	}

	attributes := make(map[string]pcommon.Value)
	resource := pcommon.NewResource()

	// normal cases
	resource.Attributes().PutStr(conventionsv112.AttributeAWSLogGroupNames, "group1&group2")
	filtered, awsData := makeAws(attributes, resource, nil)
	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.Len(t, awsData.CWLogs, 2)
	assert.Contains(t, awsData.CWLogs, cwl1)
	assert.Contains(t, awsData.CWLogs, cwl2)

	// with extra & at end
	resource.Attributes().PutStr(conventionsv112.AttributeAWSLogGroupNames, "group1&group2&")
	filtered, awsData = makeAws(attributes, resource, nil)
	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.Len(t, awsData.CWLogs, 2)
	assert.Contains(t, awsData.CWLogs, cwl1)
	assert.Contains(t, awsData.CWLogs, cwl2)

	// with extra & in the middle
	resource.Attributes().PutStr(conventionsv112.AttributeAWSLogGroupNames, "group1&&group2")
	filtered, awsData = makeAws(attributes, resource, nil)
	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.Len(t, awsData.CWLogs, 2)
	assert.Contains(t, awsData.CWLogs, cwl1)
	assert.Contains(t, awsData.CWLogs, cwl2)

	// with extra & at the beginning
	resource.Attributes().PutStr(conventionsv112.AttributeAWSLogGroupNames, "&group1&group2")
	filtered, awsData = makeAws(attributes, resource, nil)
	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.Len(t, awsData.CWLogs, 2)
	assert.Contains(t, awsData.CWLogs, cwl1)
	assert.Contains(t, awsData.CWLogs, cwl2)

	// with only &
	resource.Attributes().PutStr(conventionsv112.AttributeAWSLogGroupNames, "&")
	filtered, awsData = makeAws(attributes, resource, nil)
	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.Empty(t, awsData.CWLogs)
}

func TestLogGroupsInvalidType(t *testing.T) {
	attributes := make(map[string]pcommon.Value)
	resource := pcommon.NewResource()
	resource.Attributes().PutInt(conventionsv112.AttributeAWSLogGroupNames, 1)

	filtered, awsData := makeAws(attributes, resource, nil)

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.Empty(t, awsData.CWLogs)
}

// Simulate Log groups arns being set using OTEL_RESOURCE_ATTRIBUTES
func TestLogGroupsArnsFromStringResourceAttributes(t *testing.T) {
	group1 := "arn:aws:logs:us-east-1:123456789123:log-group:group1"

	cwl1 := awsxray.LogGroupMetadata{
		LogGroup: awsxray.String("group1"),
		Arn:      awsxray.String(group1),
	}

	attributes := make(map[string]pcommon.Value)
	resource := pcommon.NewResource()
	resource.Attributes().PutStr(conventionsv112.AttributeAWSLogGroupARNs, group1)

	filtered, awsData := makeAws(attributes, resource, nil)

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.Len(t, awsData.CWLogs, 1)
	assert.Contains(t, awsData.CWLogs, cwl1)
}

func TestLogGroupsArnsWithAmpersandFromStringResourceAttributes(t *testing.T) {
	group1 := "arn:aws:logs:us-east-1:123456789123:log-group:group1"
	group2 := "arn:aws:logs:us-east-1:123456789123:log-group:group2"

	cwl1 := awsxray.LogGroupMetadata{
		LogGroup: awsxray.String("group1"),
		Arn:      awsxray.String(group1),
	}
	cwl2 := awsxray.LogGroupMetadata{
		LogGroup: awsxray.String("group2"),
		Arn:      awsxray.String(group2),
	}

	attributes := make(map[string]pcommon.Value)
	resource := pcommon.NewResource()
	resource.Attributes().PutStr(conventionsv112.AttributeAWSLogGroupARNs, "arn:aws:logs:us-east-1:123456789123:log-group:group1&arn:aws:logs:us-east-1:123456789123:log-group:group2")

	filtered, awsData := makeAws(attributes, resource, nil)

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.Len(t, awsData.CWLogs, 2)
	assert.Contains(t, awsData.CWLogs, cwl1)
	assert.Contains(t, awsData.CWLogs, cwl2)
}

func TestLogGroupsFromConfig(t *testing.T) {
	cwl1 := awsxray.LogGroupMetadata{
		LogGroup: awsxray.String("logGroup1"),
	}
	cwl2 := awsxray.LogGroupMetadata{
		LogGroup: awsxray.String("logGroup2"),
	}

	attributes := make(map[string]pcommon.Value)
	resource := pcommon.NewResource()

	filtered, awsData := makeAws(attributes, resource, []string{"logGroup1", "logGroup2"})

	assert.NotNil(t, filtered)
	assert.NotNil(t, awsData)
	assert.Len(t, awsData.CWLogs, 2)
	assert.Contains(t, awsData.CWLogs, cwl1)
	assert.Contains(t, awsData.CWLogs, cwl2)
}
