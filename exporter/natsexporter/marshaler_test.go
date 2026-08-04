// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package natsexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// TestGetMarshalerBuiltin verifies the built-in encodings resolve for every signal.
func TestGetMarshalerBuiltin(t *testing.T) {
	host := componenttest.NewNopHost()
	for _, encoding := range []string{"otlp_proto", "otlp_json"} {
		t.Run(encoding, func(t *testing.T) {
			tm, err := getTracesMarshaler(encoding, host)
			require.NoError(t, err)
			require.NotNil(t, tm)

			mm, err := getMetricsMarshaler(encoding, host)
			require.NoError(t, err)
			require.NotNil(t, mm)

			lm, err := getLogsMarshaler(encoding, host)
			require.NoError(t, err)
			require.NotNil(t, lm)
		})
	}
}

// TestGetMarshalerUnknown verifies an unrecognized encoding that is neither a
// built-in nor a registered extension is reported as an error per signal.
func TestGetMarshalerUnknown(t *testing.T) {
	host := componenttest.NewNopHost()

	_, err := getTracesMarshaler("bogus", host)
	assert.ErrorContains(t, err, "unrecognized traces encoding")

	_, err = getMetricsMarshaler("bogus", host)
	assert.ErrorContains(t, err, "unrecognized metrics encoding")

	_, err = getLogsMarshaler("bogus", host)
	assert.ErrorContains(t, err, "unrecognized logs encoding")
}

// TestGetMarshalerEncodingExtension verifies an encoding that resolves to a
// registered encoding extension is used in preference to the built-ins.
func TestGetMarshalerEncodingExtension(t *testing.T) {
	id := component.MustNewID("myencoding")
	host := extensionHost{extensions: map[component.ID]component.Component{id: mockEncodingExtension{}}}

	tm, err := getTracesMarshaler("myencoding", host)
	require.NoError(t, err)
	tb, err := tm.MarshalTraces(ptrace.NewTraces())
	require.NoError(t, err)
	assert.Equal(t, []byte("traces"), tb)

	mm, err := getMetricsMarshaler("myencoding", host)
	require.NoError(t, err)
	mb, err := mm.MarshalMetrics(pmetric.NewMetrics())
	require.NoError(t, err)
	assert.Equal(t, []byte("metrics"), mb)

	lm, err := getLogsMarshaler("myencoding", host)
	require.NoError(t, err)
	lb, err := lm.MarshalLogs(plog.NewLogs())
	require.NoError(t, err)
	assert.Equal(t, []byte("logs"), lb)
}

// TestGetMarshalerExtensionWrongType verifies that an extension that does not
// implement the signal's marshaler interface produces a clear error.
func TestGetMarshalerExtensionWrongType(t *testing.T) {
	id := component.MustNewID("notmarshaler")
	host := extensionHost{extensions: map[component.ID]component.Component{id: nopComponent{}}}

	_, err := getTracesMarshaler("notmarshaler", host)
	assert.ErrorContains(t, err, "not a marshaler")
}

// extensionHost is a component.Host exposing a fixed set of extensions.
type extensionHost struct {
	extensions map[component.ID]component.Component
}

func (h extensionHost) GetExtensions() map[component.ID]component.Component {
	return h.extensions
}

func (extensionHost) GetFactory(component.Kind, component.Type) component.Factory {
	return nil
}

// mockEncodingExtension is a fake encoding extension that marshals every signal.
type mockEncodingExtension struct{}

func (mockEncodingExtension) Start(context.Context, component.Host) error { return nil }
func (mockEncodingExtension) Shutdown(context.Context) error              { return nil }

func (mockEncodingExtension) MarshalTraces(ptrace.Traces) ([]byte, error) {
	return []byte("traces"), nil
}

func (mockEncodingExtension) MarshalMetrics(pmetric.Metrics) ([]byte, error) {
	return []byte("metrics"), nil
}

func (mockEncodingExtension) MarshalLogs(plog.Logs) ([]byte, error) {
	return []byte("logs"), nil
}

// nopComponent is a registered extension that is not a marshaler.
type nopComponent struct{}

func (nopComponent) Start(context.Context, component.Host) error { return nil }
func (nopComponent) Shutdown(context.Context) error              { return nil }
