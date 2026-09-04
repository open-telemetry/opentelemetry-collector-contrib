// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package io.opentelemetry.contrib.awsxray.translator;

public final class AttributeConstants {

    // AWS X-Ray specific span attributes
    public static final String AWS_LOCAL_SERVICE = "aws.local.service";
    public static final String AWS_REMOTE_SERVICE = "aws.remote.service";
    public static final String AWS_LOCAL_OPERATION = "aws.local.operation";
    public static final String AWS_REMOTE_OPERATION = "aws.remote.operation";
    public static final String REMOTE_TARGET = "remoteTarget";
    public static final String AWS_SPAN_KIND = "aws.span.kind";
    public static final String K8S_REMOTE_NAMESPACE = "K8s.RemoteNamespace";
    public static final String LOCAL_ROOT = "LOCAL_ROOT";

    // AWS operation/service attributes
    public static final String AWS_OPERATION = "aws.operation";
    public static final String AWS_ACCOUNT = "aws.account_id";
    public static final String AWS_REGION = "aws.region";
    public static final String AWS_REQUEST_ID = "aws.request_id";
    public static final String AWS_REQUEST_ID_2 = "aws.requestId";
    public static final String AWS_QUEUE_URL = "aws.queue_url";
    public static final String AWS_QUEUE_URL_2 = "aws.queue.url";
    public static final String AWS_SERVICE = "aws.service";
    public static final String AWS_TABLE_NAME = "aws.table_name";
    public static final String AWS_TABLE_NAME_2 = "aws.table.name";

    // X-Ray specific attributes
    public static final String AWS_XRAY_INPROGRESS = "aws.xray.inprogress";
    public static final String AWS_XRAY_ANNOTATIONS = "aws.xray.annotations";
    public static final String AWS_XRAY_METADATA_PREFIX = "aws.xray.metadata.";

    // Semantic convention keys
    public static final String CLOUD_PROVIDER = "cloud.provider";
    public static final String CLOUD_PLATFORM = "cloud.platform";
    public static final String CLOUD_ACCOUNT_ID = "cloud.account.id";
    public static final String CLOUD_AVAILABILITY_ZONE = "cloud.availability_zone";

    public static final String HOST_ID = "host.id";
    public static final String HOST_TYPE = "host.type";
    public static final String HOST_IMAGE_ID = "host.image.id";
    public static final String HOST_NAME = "host.name";

    public static final String CONTAINER_NAME = "container.name";
    public static final String CONTAINER_ID = "container.id";
    public static final String CONTAINER_IMAGE_TAGS = "container.image.tags";
    public static final String CONTAINER_IMAGE_TAG = "container.image.tag";

    public static final String K8S_POD_NAME = "k8s.pod.name";
    public static final String K8S_CLUSTER_NAME = "k8s.cluster.name";

    public static final String SERVICE_NAME = "service.name";
    public static final String SERVICE_NAMESPACE = "service.namespace";
    public static final String SERVICE_INSTANCE_ID = "service.instance.id";
    public static final String SERVICE_VERSION = "service.version";

    public static final String TELEMETRY_SDK_NAME = "telemetry.sdk.name";
    public static final String TELEMETRY_SDK_LANGUAGE = "telemetry.sdk.language";
    public static final String TELEMETRY_SDK_VERSION = "telemetry.sdk.version";
    public static final String TELEMETRY_AUTO_VERSION = "telemetry.auto.version";
    public static final String TELEMETRY_DISTRO_VERSION = "telemetry.distro.version";

    // AWS ECS
    public static final String AWS_ECS_CLUSTER_ARN = "aws.ecs.cluster.arn";
    public static final String AWS_ECS_CONTAINER_ARN = "aws.ecs.container.arn";
    public static final String AWS_ECS_TASK_ARN = "aws.ecs.task.arn";
    public static final String AWS_ECS_TASK_FAMILY = "aws.ecs.task.family";
    public static final String AWS_ECS_LAUNCHTYPE = "aws.ecs.launchtype";

    // AWS log groups
    public static final String AWS_LOG_GROUP_NAMES = "aws.log.group.names";
    public static final String AWS_LOG_GROUP_ARNS = "aws.log.group.arns";

    // HTTP
    public static final String HTTP_METHOD = "http.method";
    public static final String HTTP_REQUEST_METHOD = "http.request.method";
    public static final String HTTP_URL = "http.url";
    public static final String URL_FULL = "url.full";
    public static final String HTTP_SCHEME = "http.scheme";
    public static final String URL_SCHEME = "url.scheme";
    public static final String HTTP_HOST = "http.host";
    public static final String HTTP_TARGET = "http.target";
    public static final String HTTP_STATUS_CODE = "http.status_code";
    public static final String HTTP_RESPONSE_STATUS_CODE = "http.response.status_code";
    public static final String HTTP_SERVER_NAME = "http.server_name";
    public static final String HTTP_CLIENT_IP = "http.client_ip";
    public static final String HTTP_USER_AGENT = "http.user_agent";
    public static final String USER_AGENT_ORIGINAL = "user_agent.original";
    public static final String URL_PATH = "url.path";
    public static final String URL_QUERY = "url.query";
    public static final String SERVER_ADDRESS = "server.address";
    public static final String SERVER_PORT = "server.port";
    public static final String CLIENT_ADDRESS = "client.address";

    // Network
    public static final String NET_HOST_PORT = "net.host.port";
    public static final String NET_HOST_NAME = "net.host.name";
    public static final String NET_PEER_NAME = "net.peer.name";
    public static final String NET_PEER_PORT = "net.peer.port";
    public static final String NET_PEER_IP = "net.peer.ip";
    public static final String NETWORK_PEER_ADDRESS = "network.peer.address";

    // RPC
    public static final String RPC_SYSTEM = "rpc.system";
    public static final String RPC_SERVICE = "rpc.service";
    public static final String RPC_METHOD = "rpc.method";

    // Database
    public static final String DB_SYSTEM = "db.system";
    public static final String DB_NAME = "db.name";
    public static final String DB_STATEMENT = "db.statement";
    public static final String DB_USER = "db.user";
    public static final String DB_CONNECTION_STRING = "db.connection_string";

    // Messaging
    public static final String MESSAGING_URL = "messaging.url";

    // DynamoDB
    public static final String AWS_DYNAMODB_TABLE_NAMES = "aws.dynamodb.table_names";

    // Peer
    public static final String PEER_SERVICE = "peer.service";

    // Enduser
    public static final String ENDUSER_ID = "enduser.id";

    // Cloud platforms
    public static final String CLOUD_PROVIDER_AWS = "aws";
    public static final String CLOUD_PLATFORM_AWS_EC2 = "aws_ec2";
    public static final String CLOUD_PLATFORM_AWS_ECS = "aws_ecs";
    public static final String CLOUD_PLATFORM_AWS_EKS = "aws_eks";
    public static final String CLOUD_PLATFORM_AWS_ELASTIC_BEANSTALK = "aws_elastic_beanstalk";
    public static final String CLOUD_PLATFORM_AWS_APP_RUNNER = "aws_app_runner";

    // ECS launch types
    public static final String AWS_ECS_LAUNCHTYPE_EC2 = "ec2";
    public static final String AWS_ECS_LAUNCHTYPE_FARGATE = "fargate";

    // AWS API RPC system
    public static final String AWS_API_RPC_SYSTEM = "aws-api";

    // Exception event
    public static final String EXCEPTION_EVENT_NAME = "exception";
    public static final String EXCEPTION_TYPE = "exception.type";
    public static final String EXCEPTION_MESSAGE = "exception.message";
    public static final String EXCEPTION_STACKTRACE = "exception.stacktrace";

    // AWS HTTP error event
    public static final String AWS_INDIVIDUAL_HTTP_EVENT_NAME = "HTTP request failure";
    public static final String AWS_INDIVIDUAL_HTTP_ERROR_EVENT_TYPE = "aws.http.error.event";
    public static final String AWS_INDIVIDUAL_HTTP_ERROR_MSG_ATTR = "aws.http.error_message";

    private AttributeConstants() {}
}
