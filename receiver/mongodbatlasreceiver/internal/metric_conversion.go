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
		err := metadata.MeasurementsToMetric(mb, meas, false)
		if err != nil {
			errs = multierr.Append(errs, err)
		}
	}

	if errs != nil {
		return fmt.Errorf("errors occurred while processing measurements: %w", errs)
	}

	return nil
}
