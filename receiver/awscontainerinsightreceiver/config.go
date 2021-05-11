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
}
