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
	"bytes"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"go.opentelemetry.io/collector/model/pdata"
	semconventions "go.opentelemetry.io/collector/translator/conventions"

	awsxray "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"
)

const (
	attributeInfrastructureService = "cloud.platform"
	awsEcsClusterArn               = "aws.ecs.cluster.arn"
	awsEcsContainerArn             = "aws.ecs.container.arn"
	awsEcsTaskArn                  = "aws.ecs.task.arn"
	awsEcsTaskFamily               = "aws.ecs.task.family"
	awsEcsLaunchType               = "aws.ecs.launchtype"
	awsLogGroupNames               = "aws.log.group.names"
	awsLogGroupArns                = "aws.log.group.arns"
)

func makeAws(attributes map[string]pdata.AttributeValue, resource pdata.Resource) (map[string]pdata.AttributeValue, *awsxray.AWSData) {
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
		logGroups    pdata.AnyValueArray
		logGroupArns pdata.AnyValueArray
		cwl          []awsxray.LogGroupMetadata
		ec2          *awsxray.EC2Metadata
		ecs          *awsxray.ECSMetadata
		ebs          *awsxray.BeanstalkMetadata
		eks          *awsxray.EKSMetadata
	)

	filtered := make(map[string]pdata.AttributeValue)
	resource.Attributes().Range(func(key string, value pdata.AttributeValue) bool {
		switch key {
		case semconventions.AttributeCloudProvider:
			cloud = value.StringVal()
		case attributeInfrastructureService:
			service = value.StringVal()
		case semconventions.AttributeCloudAccount:
			account = value.StringVal()
		case semconventions.AttributeCloudAvailabilityZone:
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
			podUID = value.StringVal()
		case semconventions.AttributeServiceNamespace:
			namespace = value.StringVal()
		case semconventions.AttributeServiceInstance:
			deployID = value.StringVal()
		case semconventions.AttributeServiceVersion:
			versionLabel = value.StringVal()
		case semconventions.AttributeTelemetrySDKName:
			sdkName = value.StringVal()
		case semconventions.AttributeTelemetrySDKLanguage:
			sdkLanguage = value.StringVal()
		case semconventions.AttributeTelemetrySDKVersion:
			sdkVersion = value.StringVal()
		case semconventions.AttributeTelemetryAutoVersion:
			autoVersion = value.StringVal()
		case semconventions.AttributeContainerID:
			containerID = value.StringVal()
		case semconventions.AttributeK8sCluster:
			clusterName = value.StringVal()
		case awsEcsClusterArn:
			clusterArn = value.StringVal()
		case awsEcsContainerArn:
			containerArn = value.StringVal()
		case awsEcsTaskArn:
			taskArn = value.StringVal()
		case awsEcsTaskFamily:
			taskFamily = value.StringVal()
		case awsEcsLaunchType:
			launchType = value.StringVal()
		case awsLogGroupNames:
			logGroups = value.ArrayVal()
		case awsLogGroupArns:
			logGroupArns = value.ArrayVal()
		}
		return true
	})

	for key, value := range attributes {
		switch key {
		case awsxray.AWSOperationAttribute:
			operation = value.StringVal()
		case awsxray.AWSAccountAttribute:
			if value.Type() != pdata.AttributeValueTypeNull {
				account = value.StringVal()
			}
		case awsxray.AWSRegionAttribute:
			remoteRegion = value.StringVal()
		case awsxray.AWSRequestIDAttribute:
			fallthrough
		case awsxray.AWSRequestIDAttribute2:
			requestID = value.StringVal()
		case awsxray.AWSQueueURLAttribute:
			fallthrough
		case awsxray.AWSQueueURLAttribute2:
			queueURL = value.StringVal()
		case awsxray.AWSTableNameAttribute:
			fallthrough
		case awsxray.AWSTableNameAttribute2:
			tableName = value.StringVal()
		default:
			filtered[key] = value
		}
	}
	if cloud != semconventions.AttributeCloudProviderAWS && cloud != "" {
		return filtered, nil // not AWS so return nil
	}

	if service == "EC2" || hostID != "" {
		ec2 = &awsxray.EC2Metadata{
			InstanceID:       awsxray.String(hostID),
			AvailabilityZone: awsxray.String(zone),
			InstanceSize:     awsxray.String(hostType),
			AmiID:            awsxray.String(amiID),
		}
	}
	if service == "ECS" || container != "" {
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

	// TODO(willarmiros): Add infrastructure_service checks once their resource detectors are implemented
	if deployID != "" {
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
	if clusterName != "" {
		eks = &awsxray.EKSMetadata{
			ClusterName: awsxray.String(clusterName),
			Pod:         awsxray.String(podUID),
			ContainerID: awsxray.String(containerID),
		}
	}

	// Since we must couple log group ARNs and Log Group Names in the same CWLogs object, we first try to derive the
	// names from the ARN, then fall back to just recording the names
	if logGroupArns != (pdata.AnyValueArray{}) && logGroupArns.Len() > 0 {
		cwl = getLogGroupMetadata(logGroupArns, true)
	} else if logGroups != (pdata.AnyValueArray{}) && logGroups.Len() > 0 {
		cwl = getLogGroupMetadata(logGroups, false)
	}

	if sdkName != "" && sdkLanguage != "" {
		// Convention for SDK name for xray SDK information is e.g., `X-Ray SDK for Java`, `X-Ray for Go`.
		// We fill in with e.g, `opentelemetry for java` by using the conventions
		sdk = sdkName + " for " + sdkLanguage
	} else {
		sdk = sdkName
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
	}
	return filtered, awsData
}

// Given an array of log group ARNs, create a corresponding amount of LogGroupMetadata objects with log_group and arn
// populated, or given an array of just log group names, create the LogGroupMetadata objects with arn omitted
func getLogGroupMetadata(logGroups pdata.AnyValueArray, isArn bool) []awsxray.LogGroupMetadata {
	var lgm []awsxray.LogGroupMetadata
	for i := 0; i < logGroups.Len(); i++ {
		if isArn {
			lgm = append(lgm, awsxray.LogGroupMetadata{
				Arn:      awsxray.String(logGroups.At(i).StringVal()),
				LogGroup: awsxray.String(parseLogGroup(logGroups.At(i).StringVal())),
			})
		} else {
			lgm = append(lgm, awsxray.LogGroupMetadata{
				LogGroup: awsxray.String(logGroups.At(i).StringVal()),
			})
		}
	}

	return lgm
}

func parseLogGroup(arn string) string {
	i := bytes.LastIndexByte([]byte(arn), byte(':'))
	if i != -1 {
		return arn[i+1:]
	}

	return arn
}
