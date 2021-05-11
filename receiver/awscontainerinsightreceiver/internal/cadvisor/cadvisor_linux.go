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

// +build linux

package cadvisor

import (
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/host"
)

// TODO: add proper field for Cadvisor
type Cadvisor struct {
}

// New creates a Cadvisor struct which can generate metrics from embedded cadvisor lib
func New(containerOrchestrator string, machineInfo *host.MachineInfo, logger *zap.Logger) *Cadvisor {
	// TODO: initialize the cadvisor
	return &Cadvisor{}
}

// GetMetrics generates metrics from cadvisor
func (c *Cadvisor) GetMetrics() []pdata.Metrics {
	// TODO: add the logic to generate the metrics from cadvisor
	return []pdata.Metrics{}
}
