// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package systemdreceiver

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/containerd/cgroups/v3/cgroup2"
	"github.com/godbus/dbus/v5"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/systemdreceiver/internal/metadata"
)

var errUnimplementedMethod = errors.New("unimplemented test method")

type testDbusObject struct {
	destination string
	path        dbus.ObjectPath
	methods     map[string][]any
	properties  map[string]dbus.Variant
}

func (s *testDbusObject) Call(method string, _ dbus.Flags, _ ...any) *dbus.Call {
	if value, exists := s.methods[method]; exists {
		return &dbus.Call{Body: value}
	}
	return &dbus.Call{Err: fmt.Errorf("no such method %s", method)}
}

func (s *testDbusObject) CallWithContext(_ context.Context, method string, flags dbus.Flags, args ...any) *dbus.Call {
	return s.Call(method, flags, args)
}

func (*testDbusObject) Go(_ string, _ dbus.Flags, _ chan *dbus.Call, _ ...any) *dbus.Call {
	return &dbus.Call{Err: errUnimplementedMethod}
}

func (*testDbusObject) GoWithContext(_ context.Context, _ string, _ dbus.Flags, _ chan *dbus.Call, _ ...any) *dbus.Call {
	return &dbus.Call{Err: errUnimplementedMethod}
}

func (*testDbusObject) AddMatchSignal(_, _ string, _ ...dbus.MatchOption) *dbus.Call {
	return &dbus.Call{Err: errUnimplementedMethod}
}

func (*testDbusObject) RemoveMatchSignal(_, _ string, _ ...dbus.MatchOption) *dbus.Call {
	return &dbus.Call{Err: errUnimplementedMethod}
}

func (s *testDbusObject) GetProperty(p string) (dbus.Variant, error) {
	if value, exists := s.properties[p]; exists {
		return value, nil
	}

	return dbus.Variant{}, errors.New("no such property")
}

func (*testDbusObject) StoreProperty(_ string, _ any) error {
	return errUnimplementedMethod
}

func (*testDbusObject) SetProperty(_ string, _ any) error {
	return errUnimplementedMethod
}

func (s *testDbusObject) Destination() string {
	return s.destination
}

func (s *testDbusObject) Path() dbus.ObjectPath {
	return s.path
}

type testDbusConnection struct {
	isOpen bool
	units  []unitTuple
}

func (s *testDbusConnection) Close() error {
	if !s.isOpen {
		return errors.New("connection already closed")
	}

	s.isOpen = false
	return nil
}

func (s *testDbusConnection) Object(dest string, path dbus.ObjectPath) dbus.BusObject {
	if dest == "org.freedesktop.systemd1" {
		switch path {
		case "/org/freedesktop/systemd1":
			return &testDbusObject{
				destination: dest,
				path:        path,
				methods: map[string][]any{
					"org.freedesktop.systemd1.Manager.ListUnitsByPatterns": {s.units},
				},
			}
		case "/org/freedesktop/systemd1/unit/nginx_2eservice":
			return &testDbusObject{
				destination: dest,
				path:        path,
				properties: map[string]dbus.Variant{
					"org.freedesktop.systemd1.Service.ControlGroup": dbus.MakeVariant("/system.slice/nginx.service"),
					"org.freedesktop.systemd1.Service.NRestarts":    dbus.MakeVariant(3),
				},
			}
		}
	}

	panic(fmt.Sprintf("unsupported object %s %s", dest, path))
}

func newTestScraper(conf *Config, units []unitTuple) *systemdScraper {
	scraper := newScraper(conf, receivertest.NewNopSettings(metadata.Type))
	scraper.conn = &testDbusConnection{isOpen: true, units: units}
	return scraper
}

func TestScraperScrape(t *testing.T) {
	nginxService := unitTuple{
		Name:        "nginx.service",
		Description: "A high performance web server and a reverse proxy server",
		LoadState:   "loaded",
		ActiveState: "active",
		SubState:    "plugged",
		Following:   "",
		Path:        "/org/freedesktop/systemd1/unit/nginx_2eservice",
		JobID:       uint32(0),
		JobType:     "",
		JobPath:     "/",
	}
	testCases := []struct {
		desc        string
		config      func() *Config
		units       []unitTuple
		goldenName  string
		expectedErr error
	}{
		{
			desc: "Basic scrape",
			config: func() *Config {
				cfg := createDefaultDisabledConfig()
				cfg.Metrics.SystemdUnitState.Enabled = true
				return cfg
			},
			units: []unitTuple{
				nginxService,
				{
					Name:        "rsyslog.service",
					Description: "Advanced key-value store",
					LoadState:   "loaded",
					ActiveState: "failed",
					SubState:    "failed",
					Following:   "",
					Path:        "/org/freedesktop/systemd1/unit/rsyslog_2eservice",
					JobID:       uint32(0),
					JobType:     "",
					JobPath:     "/",
				},
			},
			goldenName:  "basic-scrape",
			expectedErr: nil,
		},
		{
			desc: "With cgroups",
			config: func() *Config {
				cfg := createDefaultDisabledConfig()
				cfg.Metrics.SystemdServiceCPUTime.Enabled = true
				return cfg
			},
			units: []unitTuple{
				nginxService,
				{
					Name:        "dbus.socket",
					Description: "D-Bus System Message Bus Socket",
					LoadState:   "loaded",
					ActiveState: "active",
					SubState:    "plugged",
					Following:   "",
					Path:        "/org/freedesktop/systemd1/unit/dbus_2esocket",
					JobID:       uint32(0),
					JobType:     "",
					JobPath:     "/",
				},
			},
			goldenName:  "cgroups",
			expectedErr: nil,
		},
		{
			desc: "Service restarts",
			config: func() *Config {
				cfg := createDefaultDisabledConfig()
				cfg.Metrics.SystemdServiceRestarts.Enabled = true
				return cfg
			},
			units:       []unitTuple{nginxService},
			goldenName:  "restarts",
			expectedErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			scraper := newTestScraper(tc.config(), tc.units)
			scraper.cgroupOpts = []cgroup2.InitOpts{cgroup2.WithMountpoint(filepath.Join("testdata", "cgroup"))}

			actualMetrics, err := scraper.scrape(t.Context())
			if tc.expectedErr == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.expectedErr.Error())
			}

			goldenPath := filepath.Join("testdata", "expected_metrics", tc.goldenName+".yaml")
			// golden.WriteMetrics(t, goldenPath, actualMetrics)

			expectedMetrics, err := golden.ReadMetrics(goldenPath)
			require.NoError(t, err)
			require.NoError(t, pmetrictest.CompareMetrics(
				expectedMetrics, actualMetrics,
				pmetrictest.IgnoreMetricDataPointsOrder(),
				pmetrictest.IgnoreStartTimestamp(),
				pmetrictest.IgnoreTimestamp(),
			))
		})
	}
}

// Create a config where all metrics are disabled
func createDefaultDisabledConfig() *Config {
	cfg := createDefaultConfig().(*Config)
	cfg.Metrics.SystemdUnitState.Enabled = false
	cfg.Metrics.SystemdServiceCPUTime.Enabled = false
	return cfg
}
