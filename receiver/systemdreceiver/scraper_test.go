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

func Test_scraper_scrape(t *testing.T) {
	unitName := "foo.service"

	receiverCfg := createDefaultConfig().(*Config)
	receiverCfg.Units = []string{unitName}

	client := testDbusClient{
		connected: func() bool { return true },
		getAllPropertiesContext: func(ctx context.Context, unit string) (map[string]interface{}, error) {
			if unit == unitName {
				return map[string]interface{}{
					"StatusErrno": int32(7),
					"NRestarts":   uint32(2),
				}, nil
			}
			return map[string]interface{}{}, errors.New("There's an error, where there should not be")
		},
		listUnitsByNamesContext: func(ctx context.Context, s []string) ([]dbus.UnitStatus, error) {
			return []dbus.UnitStatus{
				{
					Name:        unitName,
					LoadState:   "loaded",
					ActiveState: "active",
					SubState:    "running",
				},
			}, nil
		},
		getManagerProperty: func(property string) (string, error) {
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
		},
	}

	s := newSystemdReceiver(receivertest.NewNopSettings(), receiverCfg, client)

	m, err := s.scrape(context.Background())

	require.NoError(t, err)
	expectedFile := filepath.Join("testdata", "scrape", "expected.yaml")
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)
	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, m, pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreTimestamp(), pmetrictest.IgnoreResourceMetricsOrder()))
}
