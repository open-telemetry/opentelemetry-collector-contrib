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

func (*k8slogReceiver) Start(context.Context, component.Host) error {
	return nil
}

func (*k8slogReceiver) Shutdown(context.Context) error {
	return nil
}

func newReceiver(
	receiver.Settings,
	*Config,
	consumer.Logs,
) (receiver.Logs, error) {
	return &k8slogReceiver{}, nil
}
