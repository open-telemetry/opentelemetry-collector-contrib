// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package extensions

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensiontest"
)

func TestNew_EmptyConfig(t *testing.T) {
	exts, err := New(t.Context(), nil, map[component.Type]extension.Factory{}, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	require.NotNil(t, exts)
	require.Empty(t, exts.GetExtensions())
}

func TestNew_SingleNopExtension(t *testing.T) {
	factories := map[component.Type]extension.Factory{
		extensiontest.NopType: extensiontest.NewNopFactory(),
	}

	rawCfg := map[string]any{
		"nop": map[string]any{},
	}

	exts, err := New(t.Context(), rawCfg, factories, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)

	got := exts.GetExtensions()
	require.Len(t, got, 1)

	id := component.NewID(extensiontest.NopType)
	require.Contains(t, got, id)

	require.NoError(t, exts.Start(t.Context()))
	require.NoError(t, exts.Shutdown(t.Context()))
}

func TestNew_NamedExtension(t *testing.T) {
	factories := map[component.Type]extension.Factory{
		extensiontest.NopType: extensiontest.NewNopFactory(),
	}

	rawCfg := map[string]any{
		"nop/myext": map[string]any{},
	}

	exts, err := New(t.Context(), rawCfg, factories, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)

	got := exts.GetExtensions()
	expectedID := component.MustNewIDWithName(extensiontest.NopType.String(), "myext")
	require.Contains(t, got, expectedID)
}

func TestNew_UnknownFactoryType(t *testing.T) {
	rawCfg := map[string]any{
		"doesnotexist": map[string]any{},
	}

	_, err := New(t.Context(), rawCfg, map[component.Type]extension.Factory{}, componenttest.NewNopTelemetrySettings())
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown extension type")
	require.Contains(t, err.Error(), "doesnotexist")
}

func TestNew_MultipleExtensionsDeterministicOrder(t *testing.T) {
	factories := map[component.Type]extension.Factory{
		extensiontest.NopType: extensiontest.NewNopFactory(),
	}

	// Use multiple named nop instances; parsing is map-based so insertion order
	// is non-deterministic, but our Extensions.order must be deterministic.
	rawCfg := map[string]any{
		"nop/c": map[string]any{},
		"nop/a": map[string]any{},
		"nop/b": map[string]any{},
	}

	exts, err := New(t.Context(), rawCfg, factories, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)

	ids := make([]string, 0, len(exts.order))
	for _, id := range exts.order {
		ids = append(ids, id.String())
	}
	require.Equal(t, []string{"nop/a", "nop/b", "nop/c"}, ids)

	require.NoError(t, exts.Start(t.Context()))
	require.NoError(t, exts.Shutdown(t.Context()))
}

func TestStart_RollbackOnFailure(t *testing.T) {
	startedFirst := false
	shutdownFirst := false

	firstType := component.MustNewType("first")
	secondType := component.MustNewType("second")

	firstFactory := extension.NewFactory(
		firstType,
		func() component.Config { return &struct{}{} },
		func(context.Context, extension.Settings, component.Config) (extension.Extension, error) {
			return &trackingExtension{
				onStart: func() error {
					startedFirst = true
					return nil
				},
				onShutdown: func() error {
					shutdownFirst = true
					return nil
				},
			}, nil
		},
		component.StabilityLevelStable,
	)

	startErr := errors.New("intentional start failure")
	secondFactory := extension.NewFactory(
		secondType,
		func() component.Config { return &struct{}{} },
		func(context.Context, extension.Settings, component.Config) (extension.Extension, error) {
			return &trackingExtension{
				onStart: func() error { return startErr },
			}, nil
		},
		component.StabilityLevelStable,
	)

	factories := map[component.Type]extension.Factory{
		firstType:  firstFactory,
		secondType: secondFactory,
	}

	rawCfg := map[string]any{
		"first":  map[string]any{},
		"second": map[string]any{},
	}

	exts, err := New(t.Context(), rawCfg, factories, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)

	err = exts.Start(t.Context())
	require.ErrorIs(t, err, startErr)
	assert.True(t, startedFirst, "first extension should have started")
	assert.True(t, shutdownFirst, "first extension should have been shut down during rollback")
}

func TestShutdown_ReverseOrder(t *testing.T) {
	var order []string

	mkFactory := func(name string) extension.Factory {
		typ := component.MustNewType(name)
		return extension.NewFactory(
			typ,
			func() component.Config { return &struct{}{} },
			func(context.Context, extension.Settings, component.Config) (extension.Extension, error) {
				return &trackingExtension{
					onShutdown: func() error {
						order = append(order, name)
						return nil
					},
				}, nil
			},
			component.StabilityLevelStable,
		)
	}

	aType := component.MustNewType("a")
	bType := component.MustNewType("b")
	cType := component.MustNewType("c")

	factories := map[component.Type]extension.Factory{
		aType: mkFactory("a"),
		bType: mkFactory("b"),
		cType: mkFactory("c"),
	}

	rawCfg := map[string]any{
		"a": map[string]any{},
		"b": map[string]any{},
		"c": map[string]any{},
	}

	exts, err := New(t.Context(), rawCfg, factories, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)

	require.NoError(t, exts.Start(t.Context()))
	require.NoError(t, exts.Shutdown(t.Context()))
	require.Equal(t, []string{"c", "b", "a"}, order)
}

func TestValidateConfigs(t *testing.T) {
	tests := []struct {
		name        string
		rawCfg      map[string]any
		wantErr     bool
		errContains string
	}{
		{
			name:   "nil config",
			rawCfg: nil,
		},
		{
			name:   "empty config",
			rawCfg: map[string]any{},
		},
		{
			name: "valid nop",
			rawCfg: map[string]any{
				"nop": map[string]any{},
			},
		},
		{
			name: "valid named nop",
			rawCfg: map[string]any{
				"nop/myext": map[string]any{},
			},
		},
		{
			name: "multiple valid entries",
			rawCfg: map[string]any{
				"nop":   map[string]any{},
				"nop/a": map[string]any{},
				"nop/b": map[string]any{},
			},
		},
		{
			name: "unknown extension type",
			rawCfg: map[string]any{
				"doesnotexist": map[string]any{},
			},
			wantErr:     true,
			errContains: "unknown extension type",
		},
		{
			name: "unknown type alongside valid",
			rawCfg: map[string]any{
				"nop":          map[string]any{},
				"doesnotexist": map[string]any{},
			},
			wantErr:     true,
			errContains: "unknown extension type",
		},
		{
			name: "invalid id empty name",
			rawCfg: map[string]any{
				"nop/": map[string]any{},
			},
			wantErr:     true,
			errContains: "failed to parse extensions config",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateConfigs(tc.rawCfg)
			if tc.wantErr {
				require.Error(t, err)
				if tc.errContains != "" {
					require.Contains(t, err.Error(), tc.errContains)
				}
				return
			}
			require.NoError(t, err)
		})
	}
}

// trackingExtension is a minimal extension.Extension used in tests to observe
// lifecycle calls and inject failures.
type trackingExtension struct {
	onStart    func() error
	onShutdown func() error
}

func (e *trackingExtension) Start(context.Context, component.Host) error {
	if e.onStart != nil {
		return e.onStart()
	}
	return nil
}

func (e *trackingExtension) Shutdown(context.Context) error {
	if e.onShutdown != nil {
		return e.onShutdown()
	}
	return nil
}
