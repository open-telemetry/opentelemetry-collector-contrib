// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pulsarexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/pulsarexporter"
import (
	"errors"
	"testing"

	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestNewLogsExporter_err_encoding(t *testing.T) {
	c := Config{Log: ExporterOption{Topic: "pulsar://public/default/logs", Encoding: "unknown"}}
	mexp, err := newLogsExporter(c, exportertest.NewNopCreateSettings(), logsMarshalers())
	assert.EqualError(t, err, errUnrecognizedEncoding.Error())
	assert.Nil(t, mexp)
}

func TestNewLogsExporter_err_topic_missing(t *testing.T) {
	c := Config{Log: ExporterOption{Topic: "", Encoding: defaultEncoding}}
	mexp, err := newLogsExporter(c, exportertest.NewNopCreateSettings(), logsMarshalers())
	assert.EqualError(t, err, errMissLogsTopic.Error())
	assert.Nil(t, mexp)
}

type customTraceMarshaler struct {
	encoding string
}

func (c *customTraceMarshaler) Marshal(ptrace.Traces, string) ([]*pulsar.ProducerMessage, error) {
	return nil, errors.New("unsupported encoding")
}

func (c *customTraceMarshaler) Encoding() string {
	return c.encoding
}
