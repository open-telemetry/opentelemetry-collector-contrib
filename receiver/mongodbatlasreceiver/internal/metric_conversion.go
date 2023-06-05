// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
