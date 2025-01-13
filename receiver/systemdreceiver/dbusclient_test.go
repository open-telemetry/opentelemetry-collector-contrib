// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package systemdreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/systemdreceiver"

import (
	"context"
	"errors"

	"github.com/coreos/go-systemd/v22/dbus"
)

type testDbusClient struct {
	connected func() bool
	close     func()

	listUnitsByNamesContext func(context.Context, []string) ([]dbus.UnitStatus, error)
	getAllPropertiesContext func(context.Context, string) (map[string]interface{}, error)
	getManagerProperty      func(string) (string, error)
}

func (t testDbusClient) Connected() bool {
	if t.connected == nil {
		return false
	}
	return t.connected()
}

func (t testDbusClient) Close() {
	if t.close == nil {
		return
	}
	t.close()
}

func (t testDbusClient) GetManagerProperty(property string) (string, error) {
	if t.getManagerProperty == nil {
		return "", errors.New("GetManagerProperty is not defined")
	}
	return t.getManagerProperty(property)
}

func (t testDbusClient) ListUnitsByNamesContext(ctx context.Context, units []string) ([]dbus.UnitStatus, error) {
	if t.listUnitsByNamesContext == nil {
		return []dbus.UnitStatus{}, errors.New("ListUnitsByNamesContexts is not defined")
	}
	return t.listUnitsByNamesContext(ctx, units)
}

func (t testDbusClient) GetAllPropertiesContext(ctx context.Context, unit string) (map[string]interface{}, error) {
	if t.getAllPropertiesContext == nil {
		return map[string]interface{}{}, errors.New("GetAllPropertiesContext is not defined")
	}
	return t.getAllPropertiesContext(ctx, unit)
}
