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
		assert.Equal(t, "json", m.format())
	}
	{
		m, err := newMarshaler("otlp_proto", zap.NewNop())
		assert.NoError(t, err)
		require.NotNil(t, m)
		assert.Equal(t, "binpb", m.format())
	}
	{
		m, err := newMarshaler("sumo_ic", zap.NewNop())
		assert.NoError(t, err)
		require.NotNil(t, m)
		assert.Equal(t, "json.gz", m.format())
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
		assert.Equal(t, "txt", m.format())
	}
}

type hostWithExtensions struct {
	encoding encodingExtension
}

func (h hostWithExtensions) GetExtensions() map[component.ID]component.Component {
	return map[component.ID]component.Component{
		component.MustNewID("foo"): h.encoding,
	}
}

type encodingExtension struct{}

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
