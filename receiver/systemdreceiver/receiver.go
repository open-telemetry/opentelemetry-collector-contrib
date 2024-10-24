// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package systemdreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/systemdreceiver"
import (
	"context"

	"go.opentelemetry.io/collector/component"
)

type systemdReceiver struct {
}

func (s systemdReceiver) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (s systemdReceiver) Shutdown(_ context.Context) error {
	return nil
}
