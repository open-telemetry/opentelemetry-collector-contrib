// Copyright  OpenTelemetry Authors
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

package awscontainerinsightreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver"

import (
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
)

// Config defines configuration for aws ecs container metrics receiver.
type Config struct {
	// AWSSessionSettings contains the common configuration options
	// for creating AWS session to communicate with backend
	awsutil.AWSSessionSettings `mapstructure:",squash"`

	// CollectionInterval is the interval at which metrics should be collected. The default is 60 second.
	CollectionInterval time.Duration `mapstructure:"collection_interval"`

	// ContainerOrchestrator is the type of container orchestration service, e.g. eks or ecs. The default is eks.
	ContainerOrchestrator string `mapstructure:"container_orchestrator"`

	// Whether to add the associated service name as attribute. The default is true
	TagService bool `mapstructure:"add_service_as_attribute"`

	// The "PodName" attribute is set based on the name of the relevant controllers like Daemonset, Job, ReplicaSet, ReplicationController, ...
	// If it can not be set that way and PrefFullPodName is true, the "PodName" attribute is set to the pod's own name.
	// The default value is false
	PrefFullPodName bool `mapstructure:"prefer_full_pod_name"`

	// The "FullPodName" attribute is the pod name including suffix
	// If false FullPodName label is not added
	// The default value is false
	AddFullPodNameMetricLabel bool `mapstructure:"add_full_pod_name_metric_label"`

	// The "ContainerName" attribute is the name of the container
	// If false ContainerName label is not added
	// The default value is false
	AddContainerNameMetricLabel bool `mapstructure:"add_container_name_metric_label"`

	// ClusterName can be used to explicitly provide the Cluster's Name for scenarios where it's not
	// possible to auto-detect it using EC2 tags.
	ClusterName string `mapstructure:"cluster_name"`

	// LeaderLockName is an optional attribute to override the name of the locking resource (e.g. config map) used during the leader
	// election process for EKS Container Insights. The elected leader is responsible for scraping cluster level metrics.
	// The default value is "otel-container-insight-clusterleader".
	LeaderLockName string `mapstructure:"leader_lock_name"`

	// LeaderLockUsingConfigMapOnly is an optional attribute to override the default behavior when obtaining a locking resource to be used during the leader
	// election process for EKS Container Insights. By default, the leader election logic prefers a Lease and alternatively the combination of Lease & ConfigMap.
	// When this flag is set to true, the leader election logic will be forced to use ConfigMap only. This flag mainly exists for backwards compatibility.
	// The default value is false.
	LeaderLockUsingConfigMapOnly bool `mapstructure:"leader_lock_using_config_map_only"`
}
