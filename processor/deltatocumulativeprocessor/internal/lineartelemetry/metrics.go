// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/lineartelemetry"

import (
	"errors"
	"reflect"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metadata"
)

func New(set component.TelemetrySettings) (Metrics, error) {
	m := Metrics{
		tracked: func() int { return 0 },
	}

	trackedCb := metadata.WithDeltatocumulativeStreamsTrackedLinearCallback(func() int64 {
		return int64(m.tracked())
	})

	telb, err := metadata.NewTelemetryBuilder(set, trackedCb)
	if err != nil {
		return Metrics{}, err
	}
	m.TelemetryBuilder = *telb

	return m, nil
}

type Metrics struct {
	metadata.TelemetryBuilder

	tracked func() int
}

func (m Metrics) Datapoints() metric.Int64Counter {
	return m.DeltatocumulativeDatapointsLinear
}

func (m *Metrics) WithTracked(streams func() int) {
	m.tracked = streams
}

func Error(msg string) metric.MeasurementOption {
	return metric.WithAttributeSet(attribute.NewSet(attribute.String("error", msg)))
}

func Cause(err error) metric.MeasurementOption {
	for {
		uw := errors.Unwrap(err)
		if uw == nil {
			break
		}
		err = uw
	}

	return Error(reflect.TypeOf(err).String())
}
