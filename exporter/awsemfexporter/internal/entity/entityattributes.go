// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package entity // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter/internal/entity"

const (
	// Entity resource attributes in OTLP payload
	AWSEntityPrefix                      = "com.amazonaws.cloudwatch.entity.internal."
	AttributeEntityServiceName           = AWSEntityPrefix + "service.name"
	AttributeEntityDeploymentEnvironment = AWSEntityPrefix + "deployment.environment"
	AttributeEntityK8sClusterName        = AWSEntityPrefix + "k8s.cluster.name"
	AttributeEntityK8sNamespaceName      = AWSEntityPrefix + "k8s.namespace.name"
	AttributeEntityK8sWorkloadName       = AWSEntityPrefix + "k8s.workload.name"
	AttributeEntityK8sNodeName           = AWSEntityPrefix + "k8s.node.name"
	AttributeEntityServiceNameSource     = AWSEntityPrefix + "service.name.source"
	AttributeEntityPlatformType          = AWSEntityPrefix + "platform.type"
	AttributeEntityInstanceID            = AWSEntityPrefix + "instance.id"
	AttributeEntityAutoScalingGroup      = AWSEntityPrefix + "auto.scaling.group"

	// Entity fields in EMF log
	Service              = "Service"
	Environment          = "Environment"
	EksCluster           = "EKS.Cluster"
	K8sCluster           = "K8s.Cluster"
	K8sNamespace         = "K8s.Namespace"
	K8sWorkload          = "K8s.Workload"
	K8sNode              = "K8s.Node"
	AWSServiceNameSource = "AWS.ServiceNameSource"
	PlatformType         = "PlatformType"
	InstanceID           = "EC2.InstanceId"
	AutoscalingGroup     = "EC2.AutoScalingGroup"

	// Possible values for PlatformType
	AttributeEntityEKSPlatform = "AWS::EKS"
	AttributeEntityK8sPlatform = "K8s"
)

// attributeEntityToFieldMap maps attribute entity resource attributes to entity fields
var attributeEntityToFieldMap = map[string]string{
	AttributeEntityServiceName:           Service,
	AttributeEntityDeploymentEnvironment: Environment,
	AttributeEntityK8sNamespaceName:      K8sNamespace,
	AttributeEntityK8sWorkloadName:       K8sWorkload,
	AttributeEntityK8sNodeName:           K8sNode,
	AttributeEntityPlatformType:          PlatformType,
	AttributeEntityInstanceID:            InstanceID,
	AttributeEntityAutoScalingGroup:      AutoscalingGroup,
	AttributeEntityServiceNameSource:     AWSServiceNameSource,
}

// GetEntityField returns entity field for provided attribute
func GetEntityField(attribute string, platform string) string {
	if attribute == AttributeEntityK8sClusterName {
		switch platform {
		case AttributeEntityEKSPlatform:
			return EksCluster
		case AttributeEntityK8sPlatform:
			return K8sCluster
		}
	}

	if field, ok := attributeEntityToFieldMap[attribute]; ok {
		return field
	}

	return ""
}
