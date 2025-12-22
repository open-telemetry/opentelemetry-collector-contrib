// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package systemdreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/systemdreceiver"

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/containerd/cgroups/v3"
	"github.com/containerd/cgroups/v3/cgroup2"
	"github.com/godbus/dbus/v5"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/scraper/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/systemdreceiver/internal/metadata"
)

var errNotUnifiedCGroup = errors.New("not using unified cgroups")

// A connection to dbus.
//
// This has the same interface as [dbus.Conn], but allows us to mock it out for testing.
type dbusConnection interface {
	Object(dest string, path dbus.ObjectPath) dbus.BusObject
	Close() error
}

type systemdScraper struct {
	conn       dbusConnection
	cfg        *Config
	settings   component.TelemetrySettings
	mb         *metadata.MetricsBuilder
	cgroupOpts []cgroup2.InitOpts
}

func (s *systemdScraper) start(ctx context.Context, _ component.Host) (err error) {
	var conn *dbus.Conn
	switch s.cfg.Scope {
	case "system":
		conn, err = dbus.ConnectSystemBus(dbus.WithContext(ctx))
	case "user":
		conn, err = dbus.ConnectSessionBus(dbus.WithContext(ctx))
	default:
		return errInvalidScope
	}

	if err != nil {
		return err
	}

	s.conn = conn

	return err
}

func (s *systemdScraper) shutdown(_ context.Context) (err error) {
	if s.conn != nil {
		err = s.conn.Close()
	}

	return err
}

type unitTuple struct {
	Name        string
	Description string
	LoadState   string
	ActiveState string
	SubState    string
	Following   string
	Path        dbus.ObjectPath
	JobID       uint32
	JobType     string
	JobPath     dbus.ObjectPath
}

// Find the active cgroup for each service and scrape statistics from it.
func (s *systemdScraper) scrapeServiceCgroup(now pcommon.Timestamp, unit *unitTuple) error {
	if cgroups.Mode() != cgroups.Unified {
		return errNotUnifiedCGroup
	}

	cgroupVariant, err := s.conn.Object("org.freedesktop.systemd1", unit.Path).GetProperty("org.freedesktop.systemd1.Service.ControlGroup")
	if err != nil {
		return err
	}

	var cgroupPath string
	if err2 := cgroupVariant.Store(&cgroupPath); err2 != nil {
		return err2
	} else if cgroupPath == "" {
		return nil
	}

	cgroup, err := cgroup2.Load(cgroupPath, s.cgroupOpts...)
	if err != nil {
		return err
	}

	stats, err := cgroup.Stat()
	if err != nil {
		return err
	}

	// See https://www.kernel.org/doc/html/v4.18/admin-guide/cgroup-v2.html for more information on the various
	// controllers.

	if stats.CPU != nil {
		s.mb.RecordSystemdServiceCPUTimeDataPoint(now, int64(stats.CPU.SystemUsec), metadata.AttributeCPUModeSystem)
		s.mb.RecordSystemdServiceCPUTimeDataPoint(now, int64(stats.CPU.UserUsec), metadata.AttributeCPUModeUser)
	}

	return nil
}

// Are any of our cgroup requiring metrics available
func (s *systemdScraper) hasCgroupMetrics() bool {
	return s.cfg.Metrics.SystemdServiceCPUTime.Enabled
}

func (s *systemdScraper) scrapeRestartCount(now pcommon.Timestamp, unit *unitTuple) error {
	restartVariant, err := s.conn.Object("org.freedesktop.systemd1", unit.Path).GetProperty("org.freedesktop.systemd1.Service.NRestarts")
	if err != nil {
		return err
	}

	var restarts int64
	if err2 := restartVariant.Store(&restarts); err2 != nil {
		return err2
	}

	s.mb.RecordSystemdServiceRestartsDataPoint(now, restarts)

	return nil
}

func (s *systemdScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	now := pcommon.NewTimestampFromTime(time.Now())

	var units []unitTuple
	if err := s.conn.Object("org.freedesktop.systemd1", "/org/freedesktop/systemd1").CallWithContext(ctx, "org.freedesktop.systemd1.Manager.ListUnitsByPatterns", 0, []string{}, s.cfg.Units).Store(&units); err != nil {
		return pmetric.NewMetrics(), err
	}

	var errs scrapererror.ScrapeErrors
	for i := range units {
		unit := &units[i]
		for stateName, state := range metadata.MapAttributeSystemdUnitActiveState {
			if unit.ActiveState == stateName {
				s.mb.RecordSystemdUnitStateDataPoint(now, int64(1), state)
			} else {
				s.mb.RecordSystemdUnitStateDataPoint(now, int64(0), state)
			}
		}

		if strings.HasSuffix(unit.Name, ".service") {
			if s.hasCgroupMetrics() {
				err := s.scrapeServiceCgroup(now, unit)
				if err != nil {
					errs.AddPartial(1, err)
				}
			}

			if s.cfg.Metrics.SystemdServiceRestarts.Enabled {
				err := s.scrapeRestartCount(now, unit)
				if err != nil {
					errs.AddPartial(1, err)
				}
			}
		}

		resource := s.mb.NewResourceBuilder()
		resource.SetSystemdUnitName(unit.Name)
		s.mb.EmitForResource(metadata.WithResource(resource.Emit()))
	}

	metrics := s.mb.Emit()
	return metrics, errs.Combine()
}

func newScraper(conf *Config, settings receiver.Settings) *systemdScraper {
	return &systemdScraper{
		cfg:        conf,
		settings:   settings.TelemetrySettings,
		mb:         metadata.NewMetricsBuilder(conf.MetricsBuilderConfig, settings),
		cgroupOpts: []cgroup2.InitOpts{},
	}
}
