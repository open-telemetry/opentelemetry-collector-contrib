// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate go run ./cmd/generate

package prometheusremotewrite // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite"

import (
	"strconv"

	"github.com/prometheus/prometheus/prompb"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// TimeSeries returns a slice of the prompb.TimeSeries that were converted from OTel format.
func (c *PrometheusConverter) TimeSeries() []prompb.TimeSeries {
	conflicts := 0
	for _, ts := range c.conflicts {
		conflicts += len(ts)
	}
	allTS := make([]prompb.TimeSeries, 0, len(c.unique)+conflicts)
	for _, ts := range c.unique {
		allTS = append(allTS, *ts)
	}
	for _, cTS := range c.conflicts {
		for _, ts := range cTS {
			allTS = append(allTS, *ts)
		}
	}

	return allTS
}

// FromMetrics converts pmetric.Metrics to Prometheus remote write format.
//
// Deprecated: FromMetrics exists for historical compatibility and should not be used.
// Use instead {{.Name}}.FromMetrics() and then {{.Name}}Converter.TimeSeries()
// to obtain the Prometheus remote write format metrics.
func FromMetrics(md pmetric.Metrics, settings Settings) (map[string]*prompb.TimeSeries, error) {
	c := NewPrometheusConverter()
	errs := c.FromMetrics(md, settings)
	tss := c.TimeSeries()
	out := make(map[string]*prompb.TimeSeries, len(tss))
	for i := range tss {
		out[strconv.Itoa(i)] = &tss[i]
	}

	return out, errs
}
