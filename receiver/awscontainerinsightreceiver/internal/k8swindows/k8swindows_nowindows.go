// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows
// +build !windows

package k8swindows // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/k8swindows"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/host"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"

	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type K8sWindows struct {
}

// New is a dummy function to construct a dummy K8sWindows struct for linux
func New(_ *zap.Logger, _ *stores.K8sDecorator, _ host.Info) (*K8sWindows, error) {
	return &K8sWindows{}, nil
}

// GetMetrics is a dummy function to always returns empty metrics for linux
func (k *K8sWindows) GetMetrics() []pmetric.Metrics {
	return []pmetric.Metrics{}
}

func (k *K8sWindows) Shutdown() error {
	return nil
}
