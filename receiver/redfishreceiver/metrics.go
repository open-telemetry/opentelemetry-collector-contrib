// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redfishreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redfishreceiver"

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

func (s *redfishScraper) recordComputerSystem(baseUrl string, compSys *computerSystem) {
	now := pcommon.NewTimestampFromTime(time.Now())
	s.mb.RecordSystemPowerstateDataPoint(now,
		powerStateToMetric(compSys.PowerState),
		baseUrl,
		compSys.Id,
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
		statusHealthToMetric(compSys.Status.Health),
		baseUrl,
		compSys.Id,
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
		statusStateToMetric(compSys.Status.State),
		baseUrl,
		compSys.Id,
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

func (s *redfishScraper) recordChassis(hostName, baseUrl string, chassis *chassis) {
	now := pcommon.NewTimestampFromTime(time.Now())
	s.mb.RecordChassisPowerstateDataPoint(
		now,
		powerStateToMetric(chassis.PowerState),
		hostName,
		baseUrl,
		chassis.Id,
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
		statusHealthToMetric(chassis.Status.Health),
		hostName,
		baseUrl,
		chassis.Id,
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
		statusStateToMetric(chassis.Status.State),
		hostName,
		baseUrl,
		chassis.Id,
		chassis.AssetTag,
		chassis.Model,
		chassis.Name,
		chassis.Manufacturer,
		chassis.SerialNumber,
		chassis.SKU,
		chassis.ChassisType,
	)
}

func (s *redfishScraper) recordFans(hostName, baseUrl, chassisId string, fans []fan) {
	now := pcommon.NewTimestampFromTime(time.Now())
	for _, fan := range fans {
		s.mb.RecordFanReadingDataPoint(
			now,
			*fan.Reading,
			hostName,
			baseUrl,
			chassisId,
			fan.Name,
		)
		s.mb.RecordFanStatusHealthDataPoint(
			now,
			statusHealthToMetric(fan.Status.Health),
			hostName,
			baseUrl,
			chassisId,
			fan.Name,
		)
		s.mb.RecordFanStatusStateDataPoint(
			now,
			statusStateToMetric(fan.Status.State),
			hostName,
			baseUrl,
			chassisId,
			fan.Name,
		)
	}
}

func (s *redfishScraper) recordTemperatures(hostName, baseUrl, chassisId string, temps []temperature) {
	now := pcommon.NewTimestampFromTime(time.Now())
	for _, temp := range temps {
		s.mb.RecordTemperatureReadingDataPoint(
			now,
			int64(*temp.ReadingCelsius),
			hostName,
			baseUrl,
			chassisId,
			temp.Name,
		)
		s.mb.RecordTemperatureStatusHealthDataPoint(
			now,
			statusHealthToMetric(temp.Status.Health),
			hostName,
			baseUrl,
			chassisId,
			temp.Name,
		)
		s.mb.RecordTemperatureStatusStateDataPoint(
			now,
			statusStateToMetric(temp.Status.State),
			hostName,
			baseUrl,
			chassisId,
			temp.Name,
		)
	}
}
