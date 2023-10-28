// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pulsarexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/pulsarexporter"
import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/exporter/exportertest"
)

func TestNewMetricsExporter_err_encoding(t *testing.T) {
	c := Config{Metric: ExporterOption{Topic: "pulsar://public/default/metrics", Encoding: "unknown"}}
	mexp, err := newMetricsExporter(c, exportertest.NewNopCreateSettings(), metricsMarshalers())
	assert.EqualError(t, err, errUnrecognizedEncoding.Error())
	assert.Nil(t, mexp)
}

func TestNewMetricsExporter_err_topic_missing(t *testing.T) {
	c := Config{Metric: ExporterOption{Topic: "", Encoding: defaultEncoding}}
	mexp, err := newMetricsExporter(c, exportertest.NewNopCreateSettings(), metricsMarshalers())
	assert.EqualError(t, err, errMissMetricsTopic.Error())
	assert.Nil(t, mexp)
}
