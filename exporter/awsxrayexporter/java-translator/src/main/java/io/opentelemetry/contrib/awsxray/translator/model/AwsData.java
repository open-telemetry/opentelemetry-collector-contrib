// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package io.opentelemetry.contrib.awsxray.translator.model;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;
import java.util.List;

@JsonInclude(JsonInclude.Include.NON_NULL)
public class AwsData {

    @JsonProperty("account_id")
    private String accountId;

    @JsonProperty("operation")
    private String operation;

    @JsonProperty("region")
    private String remoteRegion;

    @JsonProperty("request_id")
    private String requestId;

    @JsonProperty("queue_url")
    private String queueUrl;

    @JsonProperty("table_name")
    private String tableName;

    @JsonProperty("table_names")
    private List<String> tableNames;

    @JsonProperty("retries")
    private Long retries;

    @JsonProperty("elastic_beanstalk")
    private BeanstalkMetadata beanstalk;

    @JsonProperty("cloudwatch_logs")
    private List<LogGroupMetadata> cwLogs;

    @JsonProperty("ecs")
    private EcsMetadata ecs;

    @JsonProperty("ec2")
    private Ec2Metadata ec2;

    @JsonProperty("eks")
    private EksMetadata eks;

    @JsonProperty("xray")
    private XRayMetadata xray;

    public String getAccountId() { return accountId; }
    public void setAccountId(String accountId) { this.accountId = accountId; }

    public String getOperation() { return operation; }
    public void setOperation(String operation) { this.operation = operation; }

    public String getRemoteRegion() { return remoteRegion; }
    public void setRemoteRegion(String remoteRegion) { this.remoteRegion = remoteRegion; }

    public String getRequestId() { return requestId; }
    public void setRequestId(String requestId) { this.requestId = requestId; }

    public String getQueueUrl() { return queueUrl; }
    public void setQueueUrl(String queueUrl) { this.queueUrl = queueUrl; }

    public String getTableName() { return tableName; }
    public void setTableName(String tableName) { this.tableName = tableName; }

    public List<String> getTableNames() { return tableNames; }
    public void setTableNames(List<String> tableNames) { this.tableNames = tableNames; }

    public Long getRetries() { return retries; }
    public void setRetries(Long retries) { this.retries = retries; }

    public BeanstalkMetadata getBeanstalk() { return beanstalk; }
    public void setBeanstalk(BeanstalkMetadata beanstalk) { this.beanstalk = beanstalk; }

    public List<LogGroupMetadata> getCwLogs() { return cwLogs; }
    public void setCwLogs(List<LogGroupMetadata> cwLogs) { this.cwLogs = cwLogs; }

    public EcsMetadata getEcs() { return ecs; }
    public void setEcs(EcsMetadata ecs) { this.ecs = ecs; }

    public Ec2Metadata getEc2() { return ec2; }
    public void setEc2(Ec2Metadata ec2) { this.ec2 = ec2; }

    public EksMetadata getEks() { return eks; }
    public void setEks(EksMetadata eks) { this.eks = eks; }

    public XRayMetadata getXray() { return xray; }
    public void setXray(XRayMetadata xray) { this.xray = xray; }

    @JsonInclude(JsonInclude.Include.NON_NULL)
    public static class Ec2Metadata {
        @JsonProperty("instance_id")
        private String instanceId;

        @JsonProperty("availability_zone")
        private String availabilityZone;

        @JsonProperty("instance_size")
        private String instanceSize;

        @JsonProperty("ami_id")
        private String amiId;

        public String getInstanceId() { return instanceId; }
        public void setInstanceId(String instanceId) { this.instanceId = instanceId; }

        public String getAvailabilityZone() { return availabilityZone; }
        public void setAvailabilityZone(String availabilityZone) { this.availabilityZone = availabilityZone; }

        public String getInstanceSize() { return instanceSize; }
        public void setInstanceSize(String instanceSize) { this.instanceSize = instanceSize; }

        public String getAmiId() { return amiId; }
        public void setAmiId(String amiId) { this.amiId = amiId; }
    }

    @JsonInclude(JsonInclude.Include.NON_NULL)
    public static class EcsMetadata {
        @JsonProperty("container")
        private String containerName;

        @JsonProperty("container_id")
        private String containerId;

        @JsonProperty("task_arn")
        private String taskArn;

        @JsonProperty("task_family")
        private String taskFamily;

        @JsonProperty("cluster_arn")
        private String clusterArn;

        @JsonProperty("container_arn")
        private String containerArn;

        @JsonProperty("availability_zone")
        private String availabilityZone;

        @JsonProperty("launch_type")
        private String launchType;

        public String getContainerName() { return containerName; }
        public void setContainerName(String containerName) { this.containerName = containerName; }

        public String getContainerId() { return containerId; }
        public void setContainerId(String containerId) { this.containerId = containerId; }

        public String getTaskArn() { return taskArn; }
        public void setTaskArn(String taskArn) { this.taskArn = taskArn; }

        public String getTaskFamily() { return taskFamily; }
        public void setTaskFamily(String taskFamily) { this.taskFamily = taskFamily; }

        public String getClusterArn() { return clusterArn; }
        public void setClusterArn(String clusterArn) { this.clusterArn = clusterArn; }

        public String getContainerArn() { return containerArn; }
        public void setContainerArn(String containerArn) { this.containerArn = containerArn; }

        public String getAvailabilityZone() { return availabilityZone; }
        public void setAvailabilityZone(String availabilityZone) { this.availabilityZone = availabilityZone; }

        public String getLaunchType() { return launchType; }
        public void setLaunchType(String launchType) { this.launchType = launchType; }
    }

    @JsonInclude(JsonInclude.Include.NON_NULL)
    public static class BeanstalkMetadata {
        @JsonProperty("environment_name")
        private String environment;

        @JsonProperty("version_label")
        private String versionLabel;

        @JsonProperty("deployment_id")
        private Long deploymentId;

        public String getEnvironment() { return environment; }
        public void setEnvironment(String environment) { this.environment = environment; }

        public String getVersionLabel() { return versionLabel; }
        public void setVersionLabel(String versionLabel) { this.versionLabel = versionLabel; }

        public Long getDeploymentId() { return deploymentId; }
        public void setDeploymentId(Long deploymentId) { this.deploymentId = deploymentId; }
    }

    @JsonInclude(JsonInclude.Include.NON_NULL)
    public static class EksMetadata {
        @JsonProperty("cluster_name")
        private String clusterName;

        @JsonProperty("pod")
        private String pod;

        @JsonProperty("container_id")
        private String containerId;

        public String getClusterName() { return clusterName; }
        public void setClusterName(String clusterName) { this.clusterName = clusterName; }

        public String getPod() { return pod; }
        public void setPod(String pod) { this.pod = pod; }

        public String getContainerId() { return containerId; }
        public void setContainerId(String containerId) { this.containerId = containerId; }
    }

    @JsonInclude(JsonInclude.Include.NON_NULL)
    public static class XRayMetadata {
        @JsonProperty("sdk")
        private String sdk;

        @JsonProperty("sdk_version")
        private String sdkVersion;

        @JsonProperty("auto_instrumentation")
        private Boolean autoInstrumentation;

        public String getSdk() { return sdk; }
        public void setSdk(String sdk) { this.sdk = sdk; }

        public String getSdkVersion() { return sdkVersion; }
        public void setSdkVersion(String sdkVersion) { this.sdkVersion = sdkVersion; }

        public Boolean getAutoInstrumentation() { return autoInstrumentation; }
        public void setAutoInstrumentation(Boolean autoInstrumentation) { this.autoInstrumentation = autoInstrumentation; }
    }

    @JsonInclude(JsonInclude.Include.NON_NULL)
    public static class LogGroupMetadata {
        @JsonProperty("log_group")
        private String logGroup;

        @JsonProperty("arn")
        private String arn;

        public String getLogGroup() { return logGroup; }
        public void setLogGroup(String logGroup) { this.logGroup = logGroup; }

        public String getArn() { return arn; }
        public void setArn(String arn) { this.arn = arn; }
    }
}
