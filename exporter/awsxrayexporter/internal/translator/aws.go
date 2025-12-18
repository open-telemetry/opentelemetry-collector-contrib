// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter/internal/translator"

import (
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventionsv112 "go.opentelemetry.io/otel/semconv/v1.12.0"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"

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
	for key, value := range resource.Attributes().All() {
		switch key {
		case string(conventionsv112.CloudProviderKey):
			cloud = value.Str()
		case string(conventionsv112.CloudPlatformKey):
			service = value.Str()
		case string(conventionsv112.CloudAccountIDKey):
			account = value.Str()
		case string(conventionsv112.CloudAvailabilityZoneKey):
			zone = value.Str()
		case string(conventionsv112.HostIDKey):
			hostID = value.Str()
		case string(conventionsv112.HostTypeKey):
			hostType = value.Str()
		case string(conventionsv112.HostImageIDKey):
			amiID = value.Str()
		case string(conventionsv112.ContainerNameKey):
			if container == "" {
				container = value.Str()
			}
		case string(conventionsv112.K8SPodNameKey):
			podUID = value.Str()
		case string(conventionsv112.ServiceNamespaceKey):
			namespace = value.Str()
		case string(conventionsv112.ServiceInstanceIDKey):
			deployID = value.Str()
		case string(conventionsv112.ServiceVersionKey):
			versionLabel = value.Str()
		case string(conventionsv112.TelemetrySDKNameKey):
			sdkName = value.Str()
		case string(conventionsv112.TelemetrySDKLanguageKey):
			sdkLanguage = value.Str()
		case string(conventionsv112.TelemetrySDKVersionKey):
			sdkVersion = value.Str()
		case string(conventionsv112.TelemetryAutoVersionKey), string(conventions.TelemetryDistroVersionKey):
			autoVersion = value.Str()
		case string(conventionsv112.ContainerIDKey):
			containerID = value.Str()
		case string(conventionsv112.K8SClusterNameKey):
			clusterName = value.Str()
		case string(conventionsv112.AWSECSClusterARNKey):
			clusterArn = value.Str()
		case string(conventionsv112.AWSECSContainerARNKey):
			containerArn = value.Str()
		case string(conventionsv112.AWSECSTaskARNKey):
			taskArn = value.Str()
		case string(conventionsv112.AWSECSTaskFamilyKey):
			taskFamily = value.Str()
		case string(conventionsv112.AWSECSLaunchtypeKey):
			launchType = value.Str()
		case string(conventionsv112.AWSLogGroupNamesKey):
			logGroups = normalizeToSlice(value)
		case string(conventionsv112.AWSLogGroupARNsKey):
			logGroupArns = normalizeToSlice(value)
		}
	}

	if awsOperation, ok := attributes[awsxray.AWSOperationAttribute]; ok {
		operation = awsOperation.Str()
	} else if rpcMethod, ok := attributes[string(conventionsv112.RPCMethodKey)]; ok {
		operation = rpcMethod.Str()
	}

	for key, value := range attributes {
		switch key {
		case string(conventionsv112.RPCMethodKey):
			// Deterministically handled with if else above
		case awsxray.AWSOperationAttribute:
			// Deterministically handled with if else above
		case awsxray.AWSAccountAttribute:
			if value.Type() != pcommon.ValueTypeEmpty {
				account = value.Str()
			}
		case awsxray.AWSRegionAttribute:
			remoteRegion = value.Str()
		case awsxray.AWSRequestIDAttribute,
			awsxray.AWSRequestIDAttribute2:
			requestID = value.Str()
		case awsxray.AWSQueueURLAttribute,
			awsxray.AWSQueueURLAttribute2:
			queueURL = value.Str()
		case awsxray.AWSTableNameAttribute,
			awsxray.AWSTableNameAttribute2:
			tableName = value.Str()
		default:
			filtered[key] = value
		}
	}
	if cloud != conventionsv112.CloudProviderAWS.Value.AsString() && cloud != "" {
		return filtered, nil // not AWS so return nil
	}

	// Favor Semantic Conventions for specific SQS and DynamoDB attributes.
	if value, ok := attributes[string(conventionsv112.MessagingURLKey)]; ok {
		queueURL = value.Str()
	}
	if value, ok := attributes[string(conventionsv112.AWSDynamoDBTableNamesKey)]; ok {
		switch value.Type() {
		case pcommon.ValueTypeSlice:
			if value.Slice().Len() == 1 {
				tableName = value.Slice().At(0).Str()
			} else if value.Slice().Len() > 1 {
				tableName = ""
				tableNames = []string{}
				for i := 0; i < value.Slice().Len(); i++ {
					tableNames = append(tableNames, value.Slice().At(i).Str())
				}
			}
		case pcommon.ValueTypeStr:
			tableName = value.Str()
		}
	}

	// EC2 - add ec2 metadata to xray request if
	//       1. cloud.platfrom is set to "aws_ec2" or
	//       2. there is an non-blank host/instance id found
	if service == conventionsv112.CloudPlatformAWSEC2.Value.AsString() || hostID != "" {
		ec2 = &awsxray.EC2Metadata{
			InstanceID:       awsxray.String(hostID),
			AvailabilityZone: awsxray.String(zone),
			InstanceSize:     awsxray.String(hostType),
			AmiID:            awsxray.String(amiID),
		}
	}

	// ECS
	if service == conventionsv112.CloudPlatformAWSECS.Value.AsString() {
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
	if service == conventionsv112.CloudPlatformAWSElasticBeanstalk.Value.AsString() && deployID != "" {
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
	if service == conventionsv112.CloudPlatformAWSEKS.Value.AsString() || clusterName != "" {
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
		configSlice := pcommon.NewSlice()
		configSlice.EnsureCapacity(len(logGroupNames))

		for _, s := range logGroupNames {
			configSlice.AppendEmpty().SetStr(s)
		}

		cwl = getLogGroupMetadata(configSlice, false)
	}

	if sdkName != "" && sdkLanguage != "" {
		// Convention for SDK name for xray SDK information is e.g., `X-Ray SDK for Java`, `X-Ray for Go`.
		// We fill in with e.g, `opentelemetry for java` by using the conventionsv112
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
		TableNames:   tableNames,
	}
	return filtered, awsData
}

func getLogGroupNamesOrArns(logGroupNamesOrArns string) []string {
	// Split the input string by '&'
	items := strings.Split(logGroupNamesOrArns, "&")

	// Filter out empty strings
	var result []string
	for _, item := range items {
		if item != "" {
			result = append(result, item)
		}
	}

	return result
}

// Normalize value to slice.
// 1. String values are converted to a slice so that we can also handle resource
// attributes that are set using the OTEL_RESOURCE_ATTRIBUTES
// (multiple log group names or arns are separate by & like this "log-group1&log-group2&log-group3")
// 2. Slices are kept as they are
// 3. Other types will result in a empty slice so that we avoid panic.
func normalizeToSlice(v pcommon.Value) pcommon.Slice {
	switch v.Type() {
	case pcommon.ValueTypeStr:
		s := pcommon.NewSlice()
		logGroupNamesOrArns := getLogGroupNamesOrArns(v.Str())
		for _, logGroupOrArn := range logGroupNamesOrArns {
			s.AppendEmpty().SetStr(logGroupOrArn)
		}
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
