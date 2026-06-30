// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package io.opentelemetry.contrib.awsxray.translator;

import io.opentelemetry.api.common.AttributeKey;
import io.opentelemetry.api.common.Attributes;
import io.opentelemetry.contrib.awsxray.translator.model.AwsData;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;

import static io.opentelemetry.contrib.awsxray.translator.AttributeConstants.*;

final class AwsTranslator {

    static class AwsResult {
        final Map<String, Object> filteredAttributes;
        final AwsData awsData;

        AwsResult(Map<String, Object> filteredAttributes, AwsData awsData) {
            this.filteredAttributes = filteredAttributes;
            this.awsData = awsData;
        }
    }

    static AwsResult makeAws(Map<String, Object> attributes, Attributes resourceAttributes, List<String> logGroupNames) {
        String cloud = "";
        String service = "";
        String account = "";
        String zone = "";
        String hostId = "";
        String hostType = "";
        String amiId = "";
        String container = "";
        String namespace = "";
        String deployId = "";
        String versionLabel = "";
        String operation = "";
        String remoteRegion = "";
        String requestId = "";
        String queueUrl = "";
        String tableName = "";
        List<String> tableNames = null;
        String sdk = "";
        String sdkName = "";
        String sdkLanguage = "";
        String sdkVersion = "";
        String autoVersion = "";
        String containerId = "";
        String clusterName = "";
        String podUid = "";
        String clusterArn = "";
        String containerArn = "";
        String taskArn = "";
        String taskFamily = "";
        String launchType = "";
        List<String> logGroups = null;
        List<String> logGroupArns = null;

        Map<String, Object> filtered = new HashMap<>();

        // Process resource attributes
        for (Map.Entry<AttributeKey<?>, Object> entry : resourceAttributes.asMap().entrySet()) {
            String key = entry.getKey().getKey();
            Object value = entry.getValue();
            switch (key) {
                case CLOUD_PROVIDER: cloud = String.valueOf(value); break;
                case CLOUD_PLATFORM: service = String.valueOf(value); break;
                case CLOUD_ACCOUNT_ID: account = String.valueOf(value); break;
                case CLOUD_AVAILABILITY_ZONE: zone = String.valueOf(value); break;
                case HOST_ID: hostId = String.valueOf(value); break;
                case HOST_TYPE: hostType = String.valueOf(value); break;
                case HOST_IMAGE_ID: amiId = String.valueOf(value); break;
                case CONTAINER_NAME:
                    if (container.isEmpty()) container = String.valueOf(value);
                    break;
                case K8S_POD_NAME: podUid = String.valueOf(value); break;
                case SERVICE_NAMESPACE: namespace = String.valueOf(value); break;
                case SERVICE_INSTANCE_ID: deployId = String.valueOf(value); break;
                case SERVICE_VERSION: versionLabel = String.valueOf(value); break;
                case TELEMETRY_SDK_NAME: sdkName = String.valueOf(value); break;
                case TELEMETRY_SDK_LANGUAGE: sdkLanguage = String.valueOf(value); break;
                case TELEMETRY_SDK_VERSION: sdkVersion = String.valueOf(value); break;
                case TELEMETRY_AUTO_VERSION:
                case TELEMETRY_DISTRO_VERSION:
                    autoVersion = String.valueOf(value); break;
                case CONTAINER_ID: containerId = String.valueOf(value); break;
                case K8S_CLUSTER_NAME: clusterName = String.valueOf(value); break;
                case AWS_ECS_CLUSTER_ARN: clusterArn = String.valueOf(value); break;
                case AWS_ECS_CONTAINER_ARN: containerArn = String.valueOf(value); break;
                case AWS_ECS_TASK_ARN: taskArn = String.valueOf(value); break;
                case AWS_ECS_TASK_FAMILY: taskFamily = String.valueOf(value); break;
                case AWS_ECS_LAUNCHTYPE: launchType = String.valueOf(value); break;
                case AWS_LOG_GROUP_NAMES:
                    logGroups = toStringList(value);
                    break;
                case AWS_LOG_GROUP_ARNS:
                    logGroupArns = toStringList(value);
                    break;
                default: break;
            }
        }

        // Process span attributes
        Object awsOp = attributes.get(AWS_OPERATION);
        if (awsOp != null) {
            operation = String.valueOf(awsOp);
        } else {
            Object rpcMethod = attributes.get(RPC_METHOD);
            if (rpcMethod != null) {
                operation = String.valueOf(rpcMethod);
            }
        }

        for (Map.Entry<String, Object> entry : attributes.entrySet()) {
            String key = entry.getKey();
            Object value = entry.getValue();
            switch (key) {
                case RPC_METHOD:
                case AWS_OPERATION:
                    break;
                case AWS_ACCOUNT:
                    if (value != null) account = String.valueOf(value);
                    break;
                case AWS_REGION:
                    remoteRegion = String.valueOf(value);
                    break;
                case AWS_REQUEST_ID:
                case AWS_REQUEST_ID_2:
                    requestId = String.valueOf(value);
                    break;
                case AWS_QUEUE_URL:
                case AWS_QUEUE_URL_2:
                    queueUrl = String.valueOf(value);
                    break;
                case AWS_TABLE_NAME:
                case AWS_TABLE_NAME_2:
                    tableName = String.valueOf(value);
                    break;
                default:
                    filtered.put(key, value);
                    break;
            }
        }

        if (!cloud.isEmpty() && !CLOUD_PROVIDER_AWS.equals(cloud)) {
            return new AwsResult(filtered, null);
        }

        // Favor semantic conventions for SQS and DynamoDB
        Object messagingUrl = attributes.get(MESSAGING_URL);
        if (messagingUrl != null) {
            queueUrl = String.valueOf(messagingUrl);
        }
        Object dynamoTableNames = attributes.get(AWS_DYNAMODB_TABLE_NAMES);
        if (dynamoTableNames instanceof List) {
            @SuppressWarnings("unchecked")
            List<String> tableNamesList = (List<String>) dynamoTableNames;
            if (tableNamesList.size() == 1) {
                tableName = tableNamesList.get(0);
            } else if (tableNamesList.size() > 1) {
                tableName = "";
                tableNames = tableNamesList;
            }
        }

        AwsData awsData = new AwsData();
        awsData.setAccountId(emptyToNull(account));
        awsData.setOperation(emptyToNull(operation));
        awsData.setRemoteRegion(emptyToNull(remoteRegion));
        awsData.setRequestId(emptyToNull(requestId));
        awsData.setQueueUrl(emptyToNull(queueUrl));
        awsData.setTableName(emptyToNull(tableName));
        awsData.setTableNames(tableNames);

        // EC2
        if (CLOUD_PLATFORM_AWS_EC2.equals(service) || !hostId.isEmpty()) {
            AwsData.Ec2Metadata ec2 = new AwsData.Ec2Metadata();
            ec2.setInstanceId(emptyToNull(hostId));
            ec2.setAvailabilityZone(emptyToNull(zone));
            ec2.setInstanceSize(emptyToNull(hostType));
            ec2.setAmiId(emptyToNull(amiId));
            awsData.setEc2(ec2);
        }

        // ECS
        if (CLOUD_PLATFORM_AWS_ECS.equals(service)) {
            AwsData.EcsMetadata ecs = new AwsData.EcsMetadata();
            ecs.setContainerName(emptyToNull(container));
            ecs.setContainerId(emptyToNull(containerId));
            ecs.setAvailabilityZone(emptyToNull(zone));
            ecs.setContainerArn(emptyToNull(containerArn));
            ecs.setClusterArn(emptyToNull(clusterArn));
            ecs.setTaskArn(emptyToNull(taskArn));
            ecs.setTaskFamily(emptyToNull(taskFamily));
            ecs.setLaunchType(emptyToNull(launchType));
            awsData.setEcs(ecs);
        }

        // Beanstalk
        if (CLOUD_PLATFORM_AWS_ELASTIC_BEANSTALK.equals(service) && !deployId.isEmpty()) {
            AwsData.BeanstalkMetadata beanstalk = new AwsData.BeanstalkMetadata();
            beanstalk.setEnvironment(emptyToNull(namespace));
            beanstalk.setVersionLabel(emptyToNull(versionLabel));
            try {
                beanstalk.setDeploymentId(Long.parseLong(deployId));
            } catch (NumberFormatException e) {
                beanstalk.setDeploymentId(0L);
            }
            awsData.setBeanstalk(beanstalk);
        }

        // EKS
        if (CLOUD_PLATFORM_AWS_EKS.equals(service) || !clusterName.isEmpty()) {
            AwsData.EksMetadata eks = new AwsData.EksMetadata();
            eks.setClusterName(emptyToNull(clusterName));
            eks.setPod(emptyToNull(podUid));
            eks.setContainerId(emptyToNull(containerId));
            awsData.setEks(eks);
        }

        // CloudWatch Logs
        List<AwsData.LogGroupMetadata> cwl = null;
        if (logGroupArns != null && !logGroupArns.isEmpty()) {
            cwl = getLogGroupMetadata(logGroupArns, true);
        } else if (logGroups != null && !logGroups.isEmpty()) {
            cwl = getLogGroupMetadata(logGroups, false);
        } else if (logGroupNames != null && !logGroupNames.isEmpty()) {
            cwl = getLogGroupMetadata(logGroupNames, false);
        }
        awsData.setCwLogs(cwl);

        // SDK info
        if (!sdkName.isEmpty() && !sdkLanguage.isEmpty()) {
            sdk = sdkName + " for " + sdkLanguage;
        } else {
            sdk = sdkName;
        }

        AwsData.XRayMetadata xray = new AwsData.XRayMetadata();
        xray.setSdk(emptyToNull(sdk));
        xray.setSdkVersion(emptyToNull(sdkVersion));
        xray.setAutoInstrumentation(!autoVersion.isEmpty());
        awsData.setXray(xray);

        return new AwsResult(filtered, awsData);
    }

    private static List<AwsData.LogGroupMetadata> getLogGroupMetadata(List<String> logGroups, boolean isArn) {
        List<AwsData.LogGroupMetadata> result = new ArrayList<>();
        for (String entry : logGroups) {
            AwsData.LogGroupMetadata lgm = new AwsData.LogGroupMetadata();
            if (isArn) {
                lgm.setArn(entry);
                lgm.setLogGroup(parseLogGroup(entry));
            } else {
                lgm.setLogGroup(entry);
            }
            result.add(lgm);
        }
        return result;
    }

    private static String parseLogGroup(String arn) {
        String[] parts = arn.split(":");
        if (parts.length >= 7) {
            return parts[6];
        }
        return arn;
    }

    @SuppressWarnings("unchecked")
    private static List<String> toStringList(Object value) {
        if (value instanceof List) {
            List<String> result = new ArrayList<>();
            for (Object item : (List<?>) value) {
                result.add(String.valueOf(item));
            }
            return result;
        } else if (value instanceof String) {
            String str = (String) value;
            List<String> result = new ArrayList<>();
            for (String part : str.split("&")) {
                if (!part.isEmpty()) {
                    result.add(part);
                }
            }
            return result;
        }
        return null;
    }

    private static String emptyToNull(String s) {
        return (s == null || s.isEmpty()) ? null : s;
    }

    private AwsTranslator() {}
}
