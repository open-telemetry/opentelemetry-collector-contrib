// Copyright The OpenTelemetry Authors
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

//go:build !linux
// +build !linux

package cadvisor // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor"

import (
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor/extractors"
)

// cadvisor doesn't support windows, define the dummy functions

type HostInfo interface {
	GetNumCores() int64
	GetMemoryCapacity() int64
	GetClusterName() string
}

// Cadvisor is a dummy struct for windows
type Cadvisor struct {
}

type Decorator interface {
	Decorate(*extractors.CAdvisorMetric) *extractors.CAdvisorMetric
}

// Option is a function that can be used to configure Cadvisor struct
type Option func(*Cadvisor)

// WithDecorator constructs an option for configuring the metric decorator
func WithDecorator(d interface{}) Option {
	return func(c *Cadvisor) {
		// do nothing
	}
}

func WithECSInfoCreator(f interface{}) Option {
	return func(c *Cadvisor) {
		// do nothing
	}
}

// New is a dummy function to construct a dummy Cadvisor struct for windows
func New(containerOrchestrator string, hostInfo HostInfo, logger *zap.Logger, options ...Option) (*Cadvisor, error) {
	return &Cadvisor{}, nil
}

// GetMetrics is a dummy function that always returns empty metrics for windows
func (c *Cadvisor) GetMetrics() []pmetric.Metrics {
	return []pmetric.Metrics{}
}
