// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package subtractinitial // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor/internal/subtractinitial"

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// Type is the value users can use to configure the subtract initial point adjuster.
// The subtract initial point adjuster sets the start time of all points in a series by:
//   - Dropping the initial point, and recording its value and timestamp.
//   - Subtracting the initial point from all subsequent points, and using the timestamp of the initial point as the start timestamp.
const Type = "subtract_initial_point"

type Adjuster struct {
	set component.TelemetrySettings
}

// NewAdjuster returns a new Adjuster which adjust metrics' start times based on the initial received points.
func NewAdjuster(set component.TelemetrySettings, _ time.Duration) *Adjuster {
	return &Adjuster{
		set: set,
	}
}

// AdjustMetrics takes a sequence of metrics and adjust their start times based on the initial and
// previous points in the timeseriesMap.
func (a *Adjuster) AdjustMetrics(_ context.Context, metrics pmetric.Metrics) (pmetric.Metrics, error) {
	// TODO(#38379): Implement the subtract_initial_point adjuster
	return metrics, errors.New("not implemented")
}
