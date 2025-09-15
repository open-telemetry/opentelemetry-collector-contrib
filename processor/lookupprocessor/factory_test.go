// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lookupprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/lookup"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/internal/metadata"
)

func TestFactoryCreatesProcessors(t *testing.T) {
	factory := NewFactory()

	cfg := factory.CreateDefaultConfig()
	require.IsType(t, &Config{}, cfg)
	cfg.(*Config).Source.Extension = component.MustNewID("lookup_ext")

	settings := processortest.NewNopSettings(metadata.Type)

	traces, err := factory.CreateTraces(t.Context(), settings, cfg, consumertest.NewNop())
	require.NoError(t, err)
	host := stubHost{extensions: map[component.ID]component.Component{cfg.(*Config).Source.Extension: buildLookupExtensionForTest(t)}}
	require.NoError(t, traces.Start(t.Context(), host))
	require.NoError(t, traces.Shutdown(t.Context()))
}

func TestCreateLookupSourceWithExtension(t *testing.T) {
	settings := processortest.NewNopSettings(metadata.Type)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.Source.Extension = component.MustNewID("stublookup")

	source, err := createLookupSource(settings, cfg)
	require.NoError(t, err)
	ctx := t.Context()

	lookupExt := lookup.NewLookupExtension(
		lookup.LookupFunc(func(context.Context, string) (any, bool, error) {
			return "value", true, nil
		}),
		lookup.TypeFunc(func() string { return "stublookup" }),
		nil,
		lookup.ShutdownFunc(func(context.Context) error { return nil }),
	)

	host := stubHost{
		extensions: map[component.ID]component.Component{
			cfg.Source.Extension: lookupExt,
		},
	}

	require.NoError(t, source.Start(ctx, host))
	val, found, err := source.Lookup(ctx, "key")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "value", val)
	require.Equal(t, "stublookup", source.Type())
	require.NoError(t, source.Shutdown(ctx))
}

func buildLookupExtensionForTest(t *testing.T) lookup.LookupExtension {
	t.Helper()
	return lookup.NewLookupExtension(
		lookup.LookupFunc(func(context.Context, string) (any, bool, error) {
			return nil, false, nil
		}),
		lookup.TypeFunc(func() string { return "nooplookup" }),
		nil,
		nil,
	)
}

type stubHost struct {
	extensions map[component.ID]component.Component
}

func (s stubHost) GetExtensions() map[component.ID]component.Component {
	return s.extensions
}
