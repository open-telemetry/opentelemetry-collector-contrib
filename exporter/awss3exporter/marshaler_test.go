// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3exporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"
)

func TestMarshaler(t *testing.T) {
	{
		m, err := newMarshaler("otlp_json", zap.NewNop())
		assert.NoError(t, err)
		require.NotNil(t, m)
		assert.Equal(t, m.format(), "json")
	}
	{
		m, err := newMarshaler("otlp_proto", zap.NewNop())
		assert.NoError(t, err)
		require.NotNil(t, m)
		assert.Equal(t, m.format(), "binpb")
	}
	{
		m, err := newMarshaler("sumo_ic", zap.NewNop())
		assert.NoError(t, err)
		require.NotNil(t, m)
		assert.Equal(t, m.format(), "json.gz")
	}
	{
		m, err := newMarshaler("unknown", zap.NewNop())
		assert.Error(t, err)
		require.Nil(t, m)
	}
	{
		m, err := newMarshaler("body", zap.NewNop())
		assert.NoError(t, err)
		require.NotNil(t, m)
		assert.Equal(t, m.format(), "txt")
	}
}

type hostWithExtensions struct {
	encoding encodingExtension
}

func (h hostWithExtensions) Start(context.Context, component.Host) error {
	panic("unsupported")
}

func (h hostWithExtensions) Shutdown(context.Context) error {
	panic("unsupported")
}

func (h hostWithExtensions) GetFactory(component.Kind, component.Type) component.Factory {
	panic("unsupported")
}

func (h hostWithExtensions) GetExtensions() map[component.ID]component.Component {
	return map[component.ID]component.Component{
		component.MustNewID("foo"): h.encoding,
	}
}

func (h hostWithExtensions) GetExporters() map[component.DataType]map[component.ID]component.Component {
	panic("unsupported")
}

type encodingExtension struct {
}

func (e encodingExtension) Start(_ context.Context, _ component.Host) error {
	panic("unsupported")
}

func (e encodingExtension) Shutdown(_ context.Context) error {
	panic("unsupported")
}

func TestMarshalerFromEncoding(t *testing.T) {
	id := component.MustNewID("foo")

	{
		host := hostWithExtensions{
			encoding: encodingExtension{},
		}
		m, err := newMarshalerFromEncoding(&id, "myext", host, zap.NewNop())
		assert.NoError(t, err)
		require.NotNil(t, m)
		assert.Equal(t, "myext", m.format())
	}
	{
		m, err := newMarshalerFromEncoding(&id, "", componenttest.NewNopHost(), zap.NewNop())
		assert.EqualError(t, err, `unknown encoding "foo"`)
		require.Nil(t, m)
	}
}
