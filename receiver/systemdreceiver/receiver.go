// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package systemdreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/systemdreceiver"
import (
	"context"

	"go.opentelemetry.io/collector/component"
)

type systemdReceiver struct{}

func (systemdReceiver) Start(context.Context, component.Host) error {
	return nil
}

func (systemdReceiver) Shutdown(context.Context) error {
	return nil
}
