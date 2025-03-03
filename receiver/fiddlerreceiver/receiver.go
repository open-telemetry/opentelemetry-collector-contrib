// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fiddlerreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/fiddlerreceiver"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
)

type fiddlerReceiver struct{}

func newFiddlerReceiver(_ *Config, _ consumer.Metrics) *fiddlerReceiver {
	return &fiddlerReceiver{}
}

// Start begins collecting metrics from Fiddler API.
func (fr *fiddlerReceiver) Start(ctx context.Context, host component.Host) error {
	return nil
}

// Shutdown stops the Fiddler metrics receiver.
func (fr *fiddlerReceiver) Shutdown(ctx context.Context) error {
	return nil
}
