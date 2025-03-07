// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//go:build linux

package systemdreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/systemdreceiver"

import (
	"context"

	"github.com/coreos/go-systemd/v22/dbus"
)

func newDbusClient(ctx context.Context) (dbusClient, error) {
	return dbus.NewSystemConnectionContext(ctx)
}
