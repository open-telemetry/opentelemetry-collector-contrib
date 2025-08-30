// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mqttexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/mqttexporter"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestNewMarshaler(t *testing.T) {
	host := componenttest.NewNopHost()
	m, err := newMarshaler(nil, host)
	require.NoError(t, err)
	assert.NotNil(t, m)
	assert.NotNil(t, m.tracesMarshaler)
	assert.NotNil(t, m.metricsMarshaler)
	assert.NotNil(t, m.logsMarshaler)
}

func TestNewMarshalerWithEncodingExtension(t *testing.T) {
	host := componenttest.NewNopHost()
	encodingID := component.NewID(component.MustNewType("otlp_encoding"))
	m, err := newMarshaler(&encodingID, host)
	require.NoError(t, err)
	assert.NotNil(t, m)
}

func TestMarshalerTraces(t *testing.T) {
	host := componenttest.NewNopHost()
	m, err := newMarshaler(nil, host)
	require.NoError(t, err)

	traces := ptrace.NewTraces()
	body, err := m.tracesMarshaler.MarshalTraces(traces)
	require.NoError(t, err)
	assert.NotNil(t, body)
}

func TestMarshalerMetrics(t *testing.T) {
	host := componenttest.NewNopHost()
	m, err := newMarshaler(nil, host)
	require.NoError(t, err)

	metrics := pmetric.NewMetrics()
	body, err := m.metricsMarshaler.MarshalMetrics(metrics)
	require.NoError(t, err)
	assert.NotNil(t, body)
}

func TestMarshalerLogs(t *testing.T) {
	host := componenttest.NewNopHost()
	m, err := newMarshaler(nil, host)
	require.NoError(t, err)

	logs := plog.NewLogs()
	body, err := m.logsMarshaler.MarshalLogs(logs)
	require.NoError(t, err)
	assert.NotNil(t, body)
}
