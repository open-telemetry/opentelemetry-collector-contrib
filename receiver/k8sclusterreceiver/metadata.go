// Copyright 2020 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package k8sclusterreceiver

import (
	"fmt"
	"strings"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// Keys for K8s properties
	k8sKeyWorkLoad     = "k8s.workload"
	k8sKeyWorkLoadName = "k8s.workload.name"
)

// KubernetesMetadata currently provides a reasonable way to store metadata
// and may changes depending on how we decide to handle property updates.
type KubernetesMetadata struct {
	resourceIDKey string
	resourceID    string
	properties    map[string]string
}

// getGenericMetadata is responsible for collecting metadata from K8s resources that
// live on v1.ObjectMeta.
func getGenericMetadata(om *v1.ObjectMeta, resourceType string) *KubernetesMetadata {
	properties := om.Labels

	properties[k8sKeyWorkLoad] = resourceType
	properties[k8sKeyWorkLoadName] = om.Name
	properties[fmt.Sprintf("%s.creation_timestamp",
		resourceType)] = om.GetCreationTimestamp().Format(time.RFC3339)

	for _, or := range om.OwnerReferences {
		properties[strings.ToLower(or.Kind)] = or.Name
		properties[strings.ToLower(or.Kind)+"_uid"] = string(or.UID)
	}

	return &KubernetesMetadata{
		resourceIDKey: fmt.Sprintf("k8s.%s.uid", resourceType),
		resourceID:    string(om.UID),
		properties:    properties,
	}
}
