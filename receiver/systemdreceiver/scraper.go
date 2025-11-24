// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package systemdreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/systemdreceiver"

import (
	"context"
	"time"

	"github.com/godbus/dbus/v5"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/systemdreceiver/internal/metadata"
)

// A connection to dbus.
//
// This has the same interface as [dbus.Conn], but allows us to mock it out for testing.
type dbusConnection interface {
	Object(dest string, path dbus.ObjectPath) dbus.BusObject
	Close() error
}

type systemdScraper struct {
	conn     dbusConnection
	cfg      *Config
	settings component.TelemetrySettings
	mb       *metadata.MetricsBuilder
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

func (s *systemdScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	now := pcommon.NewTimestampFromTime(time.Now())

	var units []unitTuple
	if err := s.conn.Object("org.freedesktop.systemd1", "/org/freedesktop/systemd1").CallWithContext(ctx, "org.freedesktop.systemd1.Manager.ListUnitsByPatterns", 0, []string{}, s.cfg.Units).Store(&units); err != nil {
		return pmetric.NewMetrics(), err
	}

	for i := range units {
		unit := &units[i]
		for stateName, state := range metadata.MapAttributeSystemdUnitActiveState {
			if unit.ActiveState == stateName {
				s.mb.RecordSystemdUnitStateDataPoint(now, int64(1), state)
			} else {
				s.mb.RecordSystemdUnitStateDataPoint(now, int64(0), state)
			}
		}

		resource := s.mb.NewResourceBuilder()
		resource.SetSystemdUnitName(unit.Name)
		s.mb.EmitForResource(metadata.WithResource(resource.Emit()))
	}

	metrics := s.mb.Emit()
	return metrics, nil
}

func newScraper(conf *Config, settings receiver.Settings) *systemdScraper {
	return &systemdScraper{
		cfg:      conf,
		settings: settings.TelemetrySettings,
		mb:       metadata.NewMetricsBuilder(conf.MetricsBuilderConfig, settings),
	}
}
