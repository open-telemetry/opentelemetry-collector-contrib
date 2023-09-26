// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package encodingextension

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
)

type MyCodec struct {
}

func (m MyCodec) MarshalLogs(ld plog.Logs) ([]byte, error) {
	return []byte(ld.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0).Body().Str()), nil
}

func (m MyCodec) UnmarshalLogs(buf []byte) (plog.Logs, error) {
	pl := plog.NewLogs()
	pl.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Body().SetStr(string(buf))
	return pl, nil
}

var _ (Extension) = &MyCodecExtension{}

type MyCodecExtension struct {
}

func (m MyCodecExtension) GetLogCodec() (Log, error) {
	return MyCodec{}, nil
}

func (m MyCodecExtension) GetMetricCodec() (Metric, error) {
	return nil, errors.New("unsupported")
}

func (m MyCodecExtension) GetTraceCodec() (Trace, error) {
	return nil, errors.New("unsupported")
}

func (m MyCodecExtension) Start(ctx context.Context, host component.Host) error {
	return nil
}

func (m MyCodecExtension) Shutdown(ctx context.Context) error {
	return nil
}

type mockHost struct {
	extensions map[component.ID]component.Component
}

func (m *mockHost) ReportFatalError(err error) {
}

func (m *mockHost) GetFactory(kind component.Kind, componentType component.Type) component.Factory {
	panic("implement me")
}

func (m *mockHost) GetExtensions() map[component.ID]component.Component {
	return m.extensions
}

func (m *mockHost) GetExporters() map[component.DataType]map[component.ID]component.Component {
	panic("implement me")
}

func TestGetLogCodec(t *testing.T) {
	h := &mockHost{
		extensions: map[component.ID]component.Component{},
	}
	componentID := component.NewID("foo")
	h.GetExtensions()[componentID] = &MyCodecExtension{}
	l, err := GetLogCodec(h, &componentID)
	require.NoError(t, err)
	require.NotNil(t, l)
	ld, err := l.UnmarshalLogs([]byte("foo"))
	require.NoError(t, err)
	require.Equal(t, 1, ld.LogRecordCount())
	b, err := l.MarshalLogs(ld)
	require.NoError(t, err)
	require.Equal(t, "foo", string(b))
}
