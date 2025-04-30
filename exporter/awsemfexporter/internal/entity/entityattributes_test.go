// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetEntityField(t *testing.T) {
	tests := []struct {
		name      string
		attribute string
		value     string
		want      string
	}{
		{
			name:      "AttributeEntityServiceName from map",
			attribute: AttributeEntityServiceName,
			value:     "",
			want:      Service,
		},
		{
			name:      "AttributeEntityDeploymentEnvironment from map",
			attribute: AttributeEntityDeploymentEnvironment,
			value:     "",
			want:      Environment,
		},
		{
			name:      "AttributeEntityK8sNamespaceName from map",
			attribute: AttributeEntityK8sNamespaceName,
			value:     "",
			want:      K8sNamespace,
		},
		{
			name:      "AttributeEntityK8sWorkloadName from map",
			attribute: AttributeEntityK8sWorkloadName,
			value:     "",
			want:      K8sWorkload,
		},
		{
			name:      "AttributeEntityK8sNodeName from map",
			attribute: AttributeEntityK8sNodeName,
			value:     "",
			want:      K8sNode,
		},
		{
			name:      "AttributeEntityPlatformType from map",
			attribute: AttributeEntityPlatformType,
			value:     "",
			want:      PlatformType,
		},
		{
			name:      "AttributeEntityInstanceID from map",
			attribute: AttributeEntityInstanceID,
			value:     "",
			want:      InstanceID,
		},
		{
			name:      "AttributeEntityAutoScalingGroup from map",
			attribute: AttributeEntityAutoScalingGroup,
			value:     "",
			want:      AutoscalingGroup,
		},
		{
			name:      "AttributeEntityServiceNameSource from map",
			attribute: AttributeEntityServiceNameSource,
			value:     "",
			want:      AWSServiceNameSource,
		},
		{
			name:      "K8sClusterName with EKSPlatform",
			attribute: AttributeEntityK8sClusterName,
			value:     AttributeEntityEKSPlatform,
			want:      EksCluster,
		},
		{
			name:      "K8sClusterName with K8sPlatform",
			attribute: AttributeEntityK8sClusterName,
			value:     AttributeEntityK8sPlatform,
			want:      K8sCluster,
		},
		{
			name:      "K8sClusterName with unknown platform",
			attribute: AttributeEntityK8sClusterName,
			value:     "unknown",
			want:      "",
		},
		{
			name:      "Unknown attribute",
			attribute: "unknown",
			value:     "",
			want:      "",
		},
		{
			name:      "K8sClusterName with no values provided",
			attribute: AttributeEntityK8sClusterName,
			value:     "",
			want:      "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := GetEntityField(tc.attribute, tc.value)
			assert.Equalf(t, tc.want, got,
				"GetEntityField(%q, %v) = %q; want %q",
				tc.attribute, tc.value, got, tc.want)
		})
	}
}
