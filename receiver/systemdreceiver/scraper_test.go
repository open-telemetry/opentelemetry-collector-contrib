// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//go:build linux

package systemdreceiver

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

const unitName = "foo.service"

func defaultTestClient(_ context.Context) (dbusClient, error) {
	client := new(testDbusClient)
	client.connected = func() bool { return true }
	client.getAllPropertiesContext = func(ctx context.Context, unit string) (map[string]interface{}, error) {
		if unit == unitName {
			return map[string]interface{}{
				"StatusErrno": int32(7),
				"NRestarts":   uint32(2),
			}, nil
		}
		return map[string]interface{}{}, errors.New("There's an error, where there should not be")
	}
	client.listUnitsByNamesContext = func(ctx context.Context, s []string) ([]dbus.UnitStatus, error) {
		return []dbus.UnitStatus{
			{
				Name:        unitName,
				LoadState:   "loaded",
				ActiveState: "active",
				SubState:    "running",
			},
		}, nil
	}
	client.getManagerProperty = func(property string) (string, error) {
		switch property {
		case "Version":
			return "\"240\"", nil
		case "SystemState":
			return "\"running\"", nil
		case "Architecture":
			return "\"x86-64\"", nil
		case "Virtualization":
			return "\"kvm\"", nil
		case "NJobs":
			return "@u 0", nil
		case "NInstalledJobs":
			return "@u 3528", nil
		case "NFailedJobs":
			return "@u 44", nil
		}
		return "", nil
	}
	return client, nil
}

func Test_scraper_scrape(t *testing.T) {
	receiverCfg := createDefaultConfig().(*Config)
	receiverCfg.Units = []string{unitName}

	s := newSystemdReceiver(receivertest.NewNopSettings(), receiverCfg, defaultTestClient)

	m, err := s.scrape(context.Background())

	require.NoError(t, err)
	expectedFile := filepath.Join("testdata", "scrape", "expected.yaml")
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)
	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, m, pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreTimestamp(), pmetrictest.IgnoreResourceMetricsOrder()))
}

func Test_scraper_scrape_error_create(t *testing.T) {
	receiverCfg := createDefaultConfig().(*Config)

	newTestClient := func(_ context.Context) (dbusClient, error) {
		return nil, errors.New("Creation failed")
	}

	s := newSystemdReceiver(receivertest.NewNopSettings(), receiverCfg, newTestClient)
	_, err := s.scrape(context.Background())

	require.Error(t, err)
}

func Test_scraper_scrape_error_connected(t *testing.T) {
	receiverCfg := createDefaultConfig().(*Config)

	newCallCtr := 0

	newTestClient := func(_ context.Context) (dbusClient, error) {
		// First call, create a test client where Connected returns false
		// This will then force the creation of a new client, that should finish the scrape

		if newCallCtr == 0 {
			client := new(testDbusClient)
			client.connected = func() bool { return false }
			newCallCtr++
			return client, nil
		}
		if newCallCtr == 1 {
			newCallCtr++
			return defaultTestClient(context.TODO())
		}
		return nil, errors.New("Should not get here")
	}

	s := newSystemdReceiver(receivertest.NewNopSettings(), receiverCfg, newTestClient)

	m, err := s.scrape(context.Background())

	require.NoError(t, err)
	expectedFile := filepath.Join("testdata", "scrape", "expected.yaml")
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)
	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, m, pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreTimestamp(), pmetrictest.IgnoreResourceMetricsOrder()))
}
