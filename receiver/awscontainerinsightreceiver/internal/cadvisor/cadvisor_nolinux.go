// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
	Shutdown() error
}

// Option is a function that can be used to configure Cadvisor struct
type Option func(*Cadvisor)

// WithDecorator constructs an option for configuring the metric decorator
func WithDecorator(_ any) Option {
	return func(c *Cadvisor) {
		// do nothing
	}
}

func WithECSInfoCreator(_ any) Option {
	return func(c *Cadvisor) {
		// do nothing
	}
}

// New is a dummy function to construct a dummy Cadvisor struct for windows
func New(_ string, _ HostInfo, _ *zap.Logger, _ ...Option) (*Cadvisor, error) {
	return &Cadvisor{}, nil
}

// GetMetrics is a dummy function that always returns empty metrics for windows
func (c *Cadvisor) GetMetrics() []pmetric.Metrics {
	return []pmetric.Metrics{}
}

func (c *Cadvisor) Shutdown() error {
	return nil
}
