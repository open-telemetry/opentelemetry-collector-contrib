// Copyright  OpenTelemetry Authors
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
	"errors"

	"github.com/hashicorp/go-multierror"
	"go.mongodb.org/atlas/mongodbatlas"
	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal/metadata"
)

func processMeasurements(
	resource pdata.Resource,
	measurements []*mongodbatlas.Measurements,
) (pdata.Metrics, error) {
	allErrors := make([]error, 0)
	metricSlice := pdata.NewMetrics()
	rm := metricSlice.ResourceMetrics().AppendEmpty()
	resource.CopyTo(rm.Resource())
	ilms := rm.ScopeMetrics().AppendEmpty()
	for _, meas := range measurements {
		metric, err := metadata.MeasurementsToMetric(meas, false)
		if err != nil {
			allErrors = append(allErrors, err)
		} else {
			if metric != nil {
				// TODO: still handling skipping metrics, there's got to be better
				metric.CopyTo(ilms.Metrics().AppendEmpty())
			}
		}
	}
	if len(allErrors) > 0 {
		return metricSlice, multierror.Append(errors.New("errors occurred while processing measurements"), allErrors...)
	}
	return metricSlice, nil
}
