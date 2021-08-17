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

package awscontainerinsightreceiver

import (
	"time"

	"go.opentelemetry.io/collector/config"
)

// Config defines configuration for aws ecs container metrics receiver.
type Config struct {
	config.ReceiverSettings `mapstructure:",squash"`

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
}
