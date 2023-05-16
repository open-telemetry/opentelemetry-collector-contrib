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

package gvk // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver/internal/gvk"

import "k8s.io/apimachinery/pkg/runtime/schema"

// Kubernetes group version kinds
var (
	Pod                         = schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}
	Node                        = schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Node"}
	Namespace                   = schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"}
	ReplicationController       = schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ReplicationController"}
	ResourceQuota               = schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ResourceQuota"}
	Service                     = schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Service"}
	DaemonSet                   = schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "DaemonSet"}
	Deployment                  = schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}
	ReplicaSet                  = schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "ReplicaSet"}
	StatefulSet                 = schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"}
	Job                         = schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"}
	CronJob                     = schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "CronJob"}
	CronJobBeta                 = schema.GroupVersionKind{Group: "batch", Version: "v1beta1", Kind: "CronJob"}
	HorizontalPodAutoscaler     = schema.GroupVersionKind{Group: "autoscaling", Version: "v2", Kind: "HorizontalPodAutoscaler"}
	HorizontalPodAutoscalerBeta = schema.GroupVersionKind{Group: "autoscaling", Version: "v2beta2", Kind: "HorizontalPodAutoscaler"}
	ClusterResourceQuota        = schema.GroupVersionKind{Group: "quota", Version: "v1", Kind: "ClusterResourceQuota"}
)
