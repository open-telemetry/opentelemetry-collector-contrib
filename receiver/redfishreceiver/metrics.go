// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redfishreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redfishreceiver"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redfishreceiver/internal/redfish"
)

func (s *redfishScraper) recordComputerSystem(baseURL string, compSys *redfish.ComputerSystem) {
	now := pcommon.NewTimestampFromTime(time.Now())
	s.mb.RecordSystemPowerstateDataPoint(now,
		redfish.PowerStateToMetric(compSys.PowerState),
		baseURL,
		compSys.ID,
		compSys.AssetTag,
		compSys.BiosVersion,
		compSys.HostName,
		compSys.Model,
		compSys.Name,
		compSys.Manufacturer,
		compSys.SerialNumber,
		compSys.SKU,
		compSys.SystemType,
	)
	s.mb.RecordSystemStatusHealthDataPoint(
		now,
		redfish.StatusHealthToMetric(compSys.Status.Health),
		baseURL,
		compSys.ID,
		compSys.AssetTag,
		compSys.BiosVersion,
		compSys.HostName,
		compSys.Model,
		compSys.Name,
		compSys.Manufacturer,
		compSys.SerialNumber,
		compSys.SKU,
		compSys.SystemType,
	)
	s.mb.RecordSystemStatusStateDataPoint(
		now,
		redfish.StatusStateToMetric(compSys.Status.State),
		baseURL,
		compSys.ID,
		compSys.AssetTag,
		compSys.BiosVersion,
		compSys.HostName,
		compSys.Model,
		compSys.Name,
		compSys.Manufacturer,
		compSys.SerialNumber,
		compSys.SKU,
		compSys.SystemType,
	)
}

func (s *redfishScraper) recordChassis(hostName, baseURL string, chassis *redfish.Chassis) {
	now := pcommon.NewTimestampFromTime(time.Now())
	s.mb.RecordChassisPowerstateDataPoint(
		now,
		redfish.PowerStateToMetric(chassis.PowerState),
		hostName,
		baseURL,
		chassis.ID,
		chassis.AssetTag,
		chassis.Model,
		chassis.Name,
		chassis.Manufacturer,
		chassis.SerialNumber,
		chassis.SKU,
		chassis.ChassisType,
	)
	s.mb.RecordChassisStatusHealthDataPoint(
		now,
		redfish.StatusHealthToMetric(chassis.Status.Health),
		hostName,
		baseURL,
		chassis.ID,
		chassis.AssetTag,
		chassis.Model,
		chassis.Name,
		chassis.Manufacturer,
		chassis.SerialNumber,
		chassis.SKU,
		chassis.ChassisType,
	)
	s.mb.RecordChassisStatusStateDataPoint(
		now,
		redfish.StatusStateToMetric(chassis.Status.State),
		hostName,
		baseURL,
		chassis.ID,
		chassis.AssetTag,
		chassis.Model,
		chassis.Name,
		chassis.Manufacturer,
		chassis.SerialNumber,
		chassis.SKU,
		chassis.ChassisType,
	)
}

func (s *redfishScraper) recordFans(hostName, baseURL, chassisID string, fans []redfish.Fan) {
	now := pcommon.NewTimestampFromTime(time.Now())
	for _, fan := range fans {
		if fan.Reading != nil {
			s.mb.RecordFanReadingDataPoint(
				now,
				*fan.Reading,
				hostName,
				baseURL,
				chassisID,
				fan.Name,
				fan.ReadingUnits,
			)
		}
		s.mb.RecordFanStatusHealthDataPoint(
			now,
			redfish.StatusHealthToMetric(fan.Status.Health),
			hostName,
			baseURL,
			chassisID,
			fan.Name,
		)
		s.mb.RecordFanStatusStateDataPoint(
			now,
			redfish.StatusStateToMetric(fan.Status.State),
			hostName,
			baseURL,
			chassisID,
			fan.Name,
		)
	}
}

func (s *redfishScraper) recordTemperatures(hostName, baseURL, chassisID string, temps []redfish.Temperature) {
	now := pcommon.NewTimestampFromTime(time.Now())
	for _, temp := range temps {
		if temp.ReadingCelsius != nil {
			s.mb.RecordTemperatureReadingDataPoint(
				now,
				*temp.ReadingCelsius,
				hostName,
				baseURL,
				chassisID,
				temp.Name,
			)
		}
		s.mb.RecordTemperatureStatusHealthDataPoint(
			now,
			redfish.StatusHealthToMetric(temp.Status.Health),
			hostName,
			baseURL,
			chassisID,
			temp.Name,
		)
		s.mb.RecordTemperatureStatusStateDataPoint(
			now,
			redfish.StatusStateToMetric(temp.Status.State),
			hostName,
			baseURL,
			chassisID,
			temp.Name,
		)
	}
}
