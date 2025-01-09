// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package systemdreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/systemdreceiver"

import (
	"context"
	"github.com/coreos/go-systemd/v22/dbus"
)

// This mimics part of the interface of the DBUS client we use, this allows us to mock a client
type dbusClient interface {
	Connected() bool
	Close()

	ListUnitsByNamesContext(context.Context, []string) ([]dbus.UnitStatus, error)
	GetAllPropertiesContext(context.Context, string) (map[string]interface{}, error)
}
