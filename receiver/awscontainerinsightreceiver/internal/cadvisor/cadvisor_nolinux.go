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

// +build !linux

package cadvisor

import (
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

// cadvisor doesn't support windows, define the dummy functions

type hostInfo interface {
	GetNumCores() int64
	GetMemoryCapacity() int64
	GetClusterName() string
}

// Cadvisor is a dummy struct for windows
type Cadvisor struct {
}

// New is a dummy function to construct a dummy Cadvisor struct for windows
func New(containerOrchestrator string, hostInfo hostInfo, logger *zap.Logger) (*Cadvisor, error) {
	return &Cadvisor{}, nil
}

// GetMetrics is a dummy function that always returns empty metrics for windows
func (c *Cadvisor) GetMetrics() []pdata.Metrics {
	return []pdata.Metrics{}
}
