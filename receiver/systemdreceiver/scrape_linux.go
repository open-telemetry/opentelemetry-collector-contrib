// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//go:build linux

package systemdreceiver

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/systemdreceiver/internal/metadata"
)

func (s *systemdReceiver) scrapeSystemState(now pcommon.Timestamp) []error {
	var errs []error
	props := make(map[string]string)
	for _, prop := range []string{"SystemState", "Version", "Architecture", "Virtualization"} {
		propVal, err := s.client.GetManagerProperty(prop)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		props[prop] = strings.Trim(propVal, "\"")
	}

	s.mb.RecordSystemdSystemStateDataPoint(now,
		int64(metadata.MapAttributeSystemState[props["SystemState"]]),
		props["Version"],
		metadata.MapAttributeSystemState[props["SystemState"]],
		props["Architecture"],
		props["Virtualization"],
	)
	s.mb.EmitForResource(metadata.WithResource(s.mb.NewResourceBuilder().Emit()))

	return errs
}

func (s *systemdReceiver) scrapeJobs(now pcommon.Timestamp) []error {
	var errs []error
	for prop, recordFunc := range map[string]func(pcommon.Timestamp, int64){
		"NJobs":          s.mb.RecordSystemdJobsDataPoint,
		"NInstalledJobs": s.mb.RecordSystemdInstalledJobsDataPoint,
		"NFailedJobs":    s.mb.RecordSystemdFailedJobsDataPoint,
	} {
		propVal, err := s.client.GetManagerProperty(prop)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		// GetManagerProperty returns a *string* representation of the dbus variant.
		// For a uint32, this representation is "@u NNNNN" e.g. "@u 0"
		// See https://docs.gtk.org/glib/gvariant-format-strings.html and
		// https://docs.gtk.org/glib/gvariant-format-strings.html#numeric-types
		switch propVal[:3] {
		case "@u ":
			intPropVal, err := strconv.Atoi(propVal[3:])
			if err != nil {
				errs = append(errs, err)
				continue
			}
			recordFunc(now, int64(intPropVal))
		default:
			errs = append(errs, errors.New(fmt.Sprintf("Unknown variant: %s", propVal[:3])))
			continue
		}

		s.mb.EmitForResource(metadata.WithResource(s.mb.NewResourceBuilder().Emit()))
	}
	return errs
}

func (s *systemdReceiver) scrapeUnits(ctx context.Context, now pcommon.Timestamp) []error {
	var errs []error

	s.logger.Debug("Fetching units")
	units, err := s.client.ListUnitsByNamesContext(ctx, s.config.Units)
	if err != nil {
		errs = append(errs, err)
		return errs
	}
	s.logger.Debug("Fetched units", zap.Any("units", units))

	for _, unit := range units {
		// TODO: We might want to fetch all properties for each unit in parallel and record once the fetches are finished
		s.mb.RecordSystemdUnitLoadStateDataPoint(now, int64(metadata.MapAttributeLoadState[unit.LoadState]), metadata.MapAttributeLoadState[unit.LoadState])
		s.mb.RecordSystemdUnitActiveStateDataPoint(now, int64(metadata.MapAttributeActiveState[unit.ActiveState]), metadata.MapAttributeActiveState[unit.ActiveState])
		s.mb.RecordSystemdUnitSubStateDataPoint(now, int64(metadata.MapAttributeSubState[unit.SubState]), metadata.MapAttributeSubState[unit.SubState])
		s.mb.RecordSystemdUnitStateDataPoint(now, int64(metadata.MapAttributeSubState[unit.SubState]), metadata.MapAttributeLoadState[unit.LoadState], metadata.MapAttributeActiveState[unit.ActiveState], metadata.MapAttributeSubState[unit.SubState])

		props, err := s.client.GetAllPropertiesContext(ctx, unit.Name)
		if err != nil {
			errs = append(errs, err)
		}

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

	return errs
}

func (s *systemdReceiver) scrape(ctx context.Context) (pmetric.Metrics, error) {
	s.logger.Debug("Starting scrape")

	if s.client == nil {
		// This happens when a receiver is created outside of the factory and Start() is not called
		// Mostly during testing :)
		client, err := s.newClientFunc(s.ctx)
		if err != nil {
			return pmetric.NewMetrics(), err
		}
		s.client = client
	}

	if !s.client.Connected() {
		s.logger.Debug("Reconnecting to systemd after connection loss")
		s.client.Close()
		client, err := s.newClientFunc(s.ctx)
		if err != nil {
			return pmetric.NewMetrics(), err
		}
		s.client = client
	}

	now := pcommon.NewTimestampFromTime(time.Now())

	var errors []error
	errors = s.scrapeSystemState(now)
	errors = append(errors, s.scrapeJobs(now)...)
	errors = append(errors, s.scrapeUnits(ctx, now)...)

	s.logger.Debug("Finished scrape")

	if len(errors) > 0 {
		return s.mb.Emit(), scrapererror.NewPartialScrapeError(multierr.Combine(errors...), len(errors))
	}
	return s.mb.Emit(), nil
}
