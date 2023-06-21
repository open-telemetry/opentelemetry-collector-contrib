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

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter/internal/translator"

import (
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"

	awsxray "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"
)

func makeAws(attributes map[string]pcommon.Value, resource pcommon.Resource, logGroupNames []string) (map[string]pcommon.Value, *awsxray.AWSData) {
	var (
		cloud        string
		service      string
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
		tableNames   []string
		sdk          string
		sdkName      string
		sdkLanguage  string
		sdkVersion   string
		autoVersion  string
		containerID  string
		clusterName  string
		podUID       string
		clusterArn   string
		containerArn string
		taskArn      string
		taskFamily   string
		launchType   string
		logGroups    pcommon.Slice
		logGroupArns pcommon.Slice
		cwl          []awsxray.LogGroupMetadata
		ec2          *awsxray.EC2Metadata
		ecs          *awsxray.ECSMetadata
		ebs          *awsxray.BeanstalkMetadata
		eks          *awsxray.EKSMetadata
	)

	filtered := make(map[string]pcommon.Value)
	resource.Attributes().Range(func(key string, value pcommon.Value) bool {
		switch key {
		case conventions.AttributeCloudProvider:
			cloud = value.Str()
		case conventions.AttributeCloudPlatform:
			service = value.Str()
		case conventions.AttributeCloudAccountID:
			account = value.Str()
		case conventions.AttributeCloudAvailabilityZone:
			zone = value.Str()
		case conventions.AttributeHostID:
			hostID = value.Str()
		case conventions.AttributeHostType:
			hostType = value.Str()
		case conventions.AttributeHostImageID:
			amiID = value.Str()
		case conventions.AttributeContainerName:
			if container == "" {
				container = value.Str()
			}
		case conventions.AttributeK8SPodName:
			podUID = value.Str()
		case conventions.AttributeServiceNamespace:
			namespace = value.Str()
		case conventions.AttributeServiceInstanceID:
			deployID = value.Str()
		case conventions.AttributeServiceVersion:
			versionLabel = value.Str()
		case conventions.AttributeTelemetrySDKName:
			sdkName = value.Str()
		case conventions.AttributeTelemetrySDKLanguage:
			sdkLanguage = value.Str()
		case conventions.AttributeTelemetrySDKVersion:
			sdkVersion = value.Str()
		case conventions.AttributeTelemetryAutoVersion:
			autoVersion = value.Str()
		case conventions.AttributeContainerID:
			containerID = value.Str()
		case conventions.AttributeK8SClusterName:
			clusterName = value.Str()
		case conventions.AttributeAWSECSClusterARN:
			clusterArn = value.Str()
		case conventions.AttributeAWSECSContainerARN:
			containerArn = value.Str()
		case conventions.AttributeAWSECSTaskARN:
			taskArn = value.Str()
		case conventions.AttributeAWSECSTaskFamily:
			taskFamily = value.Str()
		case conventions.AttributeAWSECSLaunchtype:
			launchType = value.Str()
		case conventions.AttributeAWSLogGroupNames:
			logGroups = normalizeToSlice(value)
		case conventions.AttributeAWSLogGroupARNs:
			logGroupArns = normalizeToSlice(value)
		}
		return true
	})

	if awsOperation, ok := attributes[awsxray.AWSOperationAttribute]; ok {
		operation = awsOperation.Str()
	} else if rpcMethod, ok := attributes[conventions.AttributeRPCMethod]; ok {
		operation = rpcMethod.Str()
	}

	for key, value := range attributes {
		switch key {
		case conventions.AttributeRPCMethod:
			// Determinstically handled with if else above
		case awsxray.AWSOperationAttribute:
			// Determinstically handled with if else above
		case awsxray.AWSAccountAttribute:
			if value.Type() != pcommon.ValueTypeEmpty {
				account = value.Str()
			}
		case awsxray.AWSRegionAttribute:
			remoteRegion = value.Str()
		case awsxray.AWSRequestIDAttribute:
			fallthrough
		case awsxray.AWSRequestIDAttribute2:
			requestID = value.Str()
		case awsxray.AWSQueueURLAttribute:
			fallthrough
		case awsxray.AWSQueueURLAttribute2:
			queueURL = value.Str()
		case awsxray.AWSTableNameAttribute:
			fallthrough
		case awsxray.AWSTableNameAttribute2:
			tableName = value.Str()
		default:
			filtered[key] = value
		}
	}
	if cloud != conventions.AttributeCloudProviderAWS && cloud != "" {
		return filtered, nil // not AWS so return nil
	}

	// Favor Semantic Conventions for specific SQS and DynamoDB attributes.
	if value, ok := attributes[conventions.AttributeMessagingURL]; ok {
		queueURL = value.Str()
	}
	if value, ok := attributes[conventions.AttributeAWSDynamoDBTableNames]; ok {
		if value.Slice().Len() == 1 {
			tableName = value.Slice().At(0).Str()
		} else if value.Slice().Len() > 1 {
			tableName = ""
			tableNames = []string{}
			for i := 0; i < value.Slice().Len(); i++ {
				tableNames = append(tableNames, value.Slice().At(i).Str())
			}
		}
	}

	// EC2 - add ec2 metadata to xray request if
	//       1. cloud.platfrom is set to "aws_ec2" or
	//       2. there is an non-blank host/instance id found
	if service == conventions.AttributeCloudPlatformAWSEC2 || hostID != "" {
		ec2 = &awsxray.EC2Metadata{
			InstanceID:       awsxray.String(hostID),
			AvailabilityZone: awsxray.String(zone),
			InstanceSize:     awsxray.String(hostType),
			AmiID:            awsxray.String(amiID),
		}
	}

	// ECS
	if service == conventions.AttributeCloudPlatformAWSECS {
		ecs = &awsxray.ECSMetadata{
			ContainerName:    awsxray.String(container),
			ContainerID:      awsxray.String(containerID),
			AvailabilityZone: awsxray.String(zone),
			ContainerArn:     awsxray.String(containerArn),
			ClusterArn:       awsxray.String(clusterArn),
			TaskArn:          awsxray.String(taskArn),
			TaskFamily:       awsxray.String(taskFamily),
			LaunchType:       awsxray.String(launchType),
		}
	}

	// Beanstalk
	if service == conventions.AttributeCloudPlatformAWSElasticBeanstalk && deployID != "" {
		deployNum, err := strconv.ParseInt(deployID, 10, 64)
		if err != nil {
			deployNum = 0
		}
		ebs = &awsxray.BeanstalkMetadata{
			Environment:  awsxray.String(namespace),
			DeploymentID: aws.Int64(deployNum),
			VersionLabel: awsxray.String(versionLabel),
		}
	}

	// EKS or native Kubernetes
	if service == conventions.AttributeCloudPlatformAWSEKS || clusterName != "" {
		eks = &awsxray.EKSMetadata{
			ClusterName: awsxray.String(clusterName),
			Pod:         awsxray.String(podUID),
			ContainerID: awsxray.String(containerID),
		}
	}

	// Since we must couple log group ARNs and Log Group Names in the same CWLogs object, we first try to derive the
	// names from the ARN, then fall back to recording the names, if they do not exist in the resource
	// then pull from them from config.
	switch {
	case logGroupArns != (pcommon.Slice{}) && logGroupArns.Len() > 0:
		cwl = getLogGroupMetadata(logGroupArns, true)
	case logGroups != (pcommon.Slice{}) && logGroups.Len() > 0:
		cwl = getLogGroupMetadata(logGroups, false)
	case logGroupNames != nil:
		var configSlice = pcommon.NewSlice()
		configSlice.EnsureCapacity(len(logGroupNames))

		for _, s := range logGroupNames {
			configSlice.AppendEmpty().SetStr(s)
		}

		cwl = getLogGroupMetadata(configSlice, false)
	}

	sdkLanguageSuffix := " for " + sdkLanguage
	sdk = sdkName
	if sdkName != "" && sdkLanguage != "" && !strings.HasSuffix(strings.ToLower(sdkName), strings.ToLower(sdkLanguageSuffix)) {
		// Convention for SDK name for xray SDK information is e.g., `X-Ray SDK for Java`, `X-Ray for Go`.
		// We fill in with e.g, `opentelemetry for java` by using the conventions
		sdk += sdkLanguageSuffix
	}

	xray := &awsxray.XRayMetaData{
		SDK:                 awsxray.String(sdk),
		SDKVersion:          awsxray.String(sdkVersion),
		AutoInstrumentation: aws.Bool(autoVersion != ""),
	}

	awsData := &awsxray.AWSData{
		AccountID:    awsxray.String(account),
		Beanstalk:    ebs,
		CWLogs:       cwl,
		ECS:          ecs,
		EC2:          ec2,
		EKS:          eks,
		XRay:         xray,
		Operation:    awsxray.String(operation),
		RemoteRegion: awsxray.String(remoteRegion),
		RequestID:    awsxray.String(requestID),
		QueueURL:     awsxray.String(queueURL),
		TableName:    awsxray.String(tableName),
		TableNames:   tableNames,
	}
	return filtered, awsData
}

// Normalize value to slice.
// 1. String values are converted to a slice of size 1 so that we can also handle resource
// attributes that are set using the OTEL_RESOURCE_ATTRIBUTES
// 2. Slices are kept as they are
// 3. Other types will result in a empty slice so that we avoid panic.
func normalizeToSlice(v pcommon.Value) pcommon.Slice {
	switch v.Type() {
	case pcommon.ValueTypeStr:
		s := pcommon.NewSlice()
		s.AppendEmpty().SetStr(v.Str())
		return s
	case pcommon.ValueTypeSlice:
		return v.Slice()
	default:
		return pcommon.NewSlice()
	}
}

// Given an array of log group ARNs, create a corresponding amount of LogGroupMetadata objects with log_group and arn
// populated, or given an array of just log group names, create the LogGroupMetadata objects with arn omitted
func getLogGroupMetadata(logGroups pcommon.Slice, isArn bool) []awsxray.LogGroupMetadata {
	var lgm []awsxray.LogGroupMetadata
	for i := 0; i < logGroups.Len(); i++ {
		if isArn {
			lgm = append(lgm, awsxray.LogGroupMetadata{
				Arn:      awsxray.String(logGroups.At(i).Str()),
				LogGroup: awsxray.String(parseLogGroup(logGroups.At(i).Str())),
			})
		} else {
			lgm = append(lgm, awsxray.LogGroupMetadata{
				LogGroup: awsxray.String(logGroups.At(i).Str()),
			})
		}
	}

	return lgm
}

// Log group name will always be in the 7th position of the ARN
// https://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/iam-access-control-overview-cwl.html#CWL_ARN_Format
func parseLogGroup(arn string) string {
	parts := strings.Split(arn, ":")
	if len(parts) >= 7 {
		return parts[6]
	}

	return arn
}
