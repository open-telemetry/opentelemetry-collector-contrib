// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rabbitmqexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

func TestMarshalUsingEncodingExtension(t *testing.T) {
	host := mockHostWithEncodings{}
	extension := mockEncodingExtension{}
	extensionMap := make(map[component.ID]component.Component)
	extensionMap[encodingComponentID] = &extension
	host.On("GetExtensions").Return(extensionMap)

	m, err := newMarshaler(&encodingComponentID, &host)

	require.NotNil(t, m)
	require.NoError(t, err)
	require.Equal(t, m.logsMarshaler, &extension)
	require.Equal(t, m.metricsMarshaler, &extension)
	require.Equal(t, m.tracesMarshaler, &extension)
}

type mockHostWithEncodings struct {
	mock.Mock
}

type mockEncodingExtension struct {
	mock.Mock
}

func (h *mockHostWithEncodings) GetExtensions() map[component.ID]component.Component {
	args := h.Called()
	return args.Get(0).(map[component.ID]component.Component)
}

func (m *mockEncodingExtension) MarshalLogs(plog.Logs) ([]byte, error) {
	return nil, nil
}

func (m *mockEncodingExtension) MarshalTraces(ptrace.Traces) ([]byte, error) {
	return nil, nil
}

func (m *mockEncodingExtension) MarshalMetrics(pmetric.Metrics) ([]byte, error) {
	return nil, nil
}

func (m *mockEncodingExtension) Start(context.Context, component.Host) error {
	return nil
}

func (m *mockEncodingExtension) Shutdown(context.Context) error {
	return nil
}
