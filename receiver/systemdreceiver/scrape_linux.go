// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//go:build linux

package systemdreceiver

import (
	"context"
	"time"

	"github.com/coreos/go-systemd/v22/dbus"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/systemdreceiver/internal/metadata"
)

func (s *systemdReceiver) scrape(ctx context.Context) (pmetric.Metrics, error) {
	s.logger.Debug("starting scrape")

	conn, err := dbus.NewSystemdConnectionContext(ctx)
	if err != nil {
		return pmetric.NewMetrics(), err
	}
	defer conn.Close()

	now := pcommon.NewTimestampFromTime(time.Now())
	s.logger.Debug("Fetching units")
	units, err := conn.ListUnitsByNamesContext(ctx, s.config.Units)
	if err != nil {
		return pmetric.NewMetrics(), err
	}

	s.logger.Debug("Fetched units", zap.Any("units", units))

	// Create metrics for all units
	var errors []error
	for _, unit := range units {
		s.mb.RecordSystemdUnitLoadStateDataPoint(now, int64(metadata.MapAttributeLoadState[unit.LoadState]), metadata.MapAttributeLoadState[unit.LoadState])
		s.mb.RecordSystemdUnitActiveStateDataPoint(now, int64(metadata.MapAttributeActiveState[unit.ActiveState]), metadata.MapAttributeActiveState[unit.ActiveState])
		s.mb.RecordSystemdUnitSubStateDataPoint(now, int64(metadata.MapAttributeSubState[unit.SubState]), metadata.MapAttributeSubState[unit.SubState])
		s.mb.RecordSystemdUnitStateDataPoint(now, int64(metadata.MapAttributeSubState[unit.SubState]), metadata.MapAttributeLoadState[unit.LoadState], metadata.MapAttributeActiveState[unit.ActiveState], metadata.MapAttributeSubState[unit.SubState])

		props, err := conn.GetAllPropertiesContext(ctx, unit.Name)
		if err != nil {
			errors = append(errors, err)
		}

		s.logger.Debug("props", zap.Any("unit", unit.Name), zap.Any("properties", props))

		if exitcode, ok := props["StatusErrno"]; ok {
			s.mb.RecordSystemdUnitErrnoDataPoint(now, int64(exitcode.(int32)))
		}

		if restarts, ok := props["NRestarts"]; ok {
			s.mb.RecordSystemdUnitRestartsDataPoint(now, int64(restarts.(uint32)))
		}

		// Now add all resource attributes
		rb := s.mb.NewResourceBuilder()
		rb.SetSystemdUnitName(unit.Name)

		s.mb.EmitForResource(metadata.WithResource(rb.Emit()))
	}
	if len(errors) > 0 {
		return s.mb.Emit(), scrapererror.NewPartialScrapeError(multierr.Combine(errors...), len(errors))
	}
	return s.mb.Emit(), nil
}
