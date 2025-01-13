// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//go:build !linux

package systemdreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/systemdreceiver"

import (
	"context"

	"github.com/coreos/go-systemd/v22/dbus"
)

type emptyDbusClient struct{}

func (e emptyDbusClient) Connected() bool { return false }
func (e emptyDbusClient) Close()          { return }
func (e emptyDbusClient) ListUnitsByNamesContext(context.Context, []string) ([]dbus.UnitStatus, error) {
	return nil, nil
}
func (e emptyDbusClient) GetAllPropertiesContext(context.Context, string) (map[string]interface{}, error) {
	return nil, nil
}
func (e emptyDbusClient) GetManagerProperty(string) (string, error) { return "", nil }

var _ dbusClient = new(emptyDbusClient)

func newDbusClient(_ context.Context) (dbusClient, error) {
	client := new(emptyDbusClient)
	return client, nil
}
