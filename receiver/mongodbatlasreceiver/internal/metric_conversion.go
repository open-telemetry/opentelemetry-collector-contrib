// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal"

import (
	"fmt"

	"go.mongodb.org/atlas/mongodbatlas"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal/metadata"
)

func processMeasurements(
	mb *metadata.MetricsBuilder,
	measurements []*mongodbatlas.Measurements,
) error {
	var errs error

	for _, meas := range measurements {
		err := metadata.MeasurementsToMetric(mb, meas)
		if err != nil {
			errs = multierr.Append(errs, err)
		}
	}

	err := calculateTotalMetrics(mb, measurements)
	if err != nil {
		errs = multierr.Append(errs, err)
	}
	if errs != nil {
		return fmt.Errorf("errors occurred while processing measurements: %w", errs)
	}

	return nil
}

func calculateTotalMetrics(
	mb *metadata.MetricsBuilder,
	measurements []*mongodbatlas.Measurements,
) error {
	var err error
	dptTotalMeasCombined := false
	var dptTotalMeas *mongodbatlas.Measurements

	for _, meas := range measurements {
		switch meas.Name {
		case "DISK_PARTITION_THROUGHPUT_READ", "DISK_PARTITION_THROUGHPUT_WRITE":
			if dptTotalMeas == nil {
				dptTotalMeas = cloneMeasurement(meas)
				dptTotalMeas.Name = "DISK_PARTITION_THROUGHPUT_TOTAL"
				continue
			}

			// Combine data point values with matching timestamps
			for j, totalMeas := range dptTotalMeas.DataPoints {
				if totalMeas.Timestamp != meas.DataPoints[j].Timestamp ||
					(totalMeas.Value == nil && meas.DataPoints[j].Value == nil) {
					continue
				}
				if totalMeas.Value == nil {
					totalMeas.Value = new(float32)
				}
				addValue := *meas.DataPoints[j].Value
				if meas.DataPoints[j].Value == nil {
					addValue = 0
				}
				*totalMeas.Value += addValue
				dptTotalMeasCombined = true
			}
		default:
		}
	}

	if dptTotalMeasCombined {
		err = metadata.MeasurementsToMetric(mb, dptTotalMeas)
	}
	return err
}

func cloneMeasurement(meas *mongodbatlas.Measurements) *mongodbatlas.Measurements {
	clone := &mongodbatlas.Measurements{
		Name:       meas.Name,
		Units:      meas.Units,
		DataPoints: make([]*mongodbatlas.DataPoints, len(meas.DataPoints)),
	}

	for i, dp := range meas.DataPoints {
		if dp != nil {
			newDP := *dp
			clone.DataPoints[i] = &newDP
		}
	}

	return clone
}
