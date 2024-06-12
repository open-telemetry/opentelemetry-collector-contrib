// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/awsapplicationsignalsprocessor/internal/common"

const (
	MetricAttributeLocalService             = "Service"
	MetricAttributeLocalOperation           = "Operation"
	MetricAttributeEnvironment              = "Environment"
	MetricAttributeRemoteService            = "RemoteService"
	MetricAttributeRemoteEnvironment        = "RemoteEnvironment"
	MetricAttributeRemoteOperation          = "RemoteOperation"
	MetricAttributeRemoteResourceIdentifier = "RemoteResourceIdentifier"
	MetricAttributeRemoteResourceType       = "RemoteResourceType"
)

const (
	AttributeEKSClusterName      = "EKS.Cluster"
	AttributeK8SClusterName      = "K8s.Cluster"
	AttributeK8SNamespace        = "K8s.Namespace"
	AttributeEC2AutoScalingGroup = "EC2.AutoScalingGroup"
	AttributeEC2InstanceID       = "EC2.InstanceId"
	AttributeHost                = "Host"
	AttributePlatformType        = "PlatformType"
	AttributeTelemetrySDK        = "Telemetry.SDK"
	AttributeTelemetryAgent      = "Telemetry.Agent"
	AttributeTelemetrySource     = "Telemetry.Source"
)

const (
	AttributeTmpReserved = "aws.tmp.reserved"
)

var IndexableMetricAttributes = []string{
	MetricAttributeLocalService,
	MetricAttributeLocalOperation,
	MetricAttributeEnvironment,
	MetricAttributeRemoteService,
	MetricAttributeRemoteEnvironment,
	MetricAttributeRemoteOperation,
	MetricAttributeRemoteResourceIdentifier,
	MetricAttributeRemoteResourceType,
}
