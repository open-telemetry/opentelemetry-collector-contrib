// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8slogreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8slogreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

type k8slogReceiver struct{}

func (k *k8slogReceiver) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (k *k8slogReceiver) Shutdown(_ context.Context) error {
	return nil
}

func newReceiver(
	_ receiver.Settings,
	_ *Config,
	_ consumer.Logs,
) (receiver.Logs, error) {
	return &k8slogReceiver{}, nil
}
