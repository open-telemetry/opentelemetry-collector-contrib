// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscontainerinsightreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver"

import (
	"time"
)

// Config defines configuration for aws ecs container metrics receiver.
type Config struct {

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
}
