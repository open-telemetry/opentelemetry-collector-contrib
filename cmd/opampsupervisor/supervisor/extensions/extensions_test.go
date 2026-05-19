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
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensiontest"
)

func nopFactories() map[component.Type]extension.Factory {
	return map[component.Type]extension.Factory{
		extensiontest.NopType: extensiontest.NewNopFactory(),
	}
}

func nopConfig(t *testing.T, ids ...component.ID) Config {
	t.Helper()
	factory := extensiontest.NewNopFactory()
	cfg := make(Config, len(ids))
	for _, id := range ids {
		cfg[id] = factory.CreateDefaultConfig()
	}
	return cfg
}

func TestNew_EmptyConfig(t *testing.T) {
	exts, err := New(t.Context(), nil, nopFactories(), componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	require.NotNil(t, exts)
	require.Empty(t, exts.GetExtensions())
}

func TestNew_SingleNopExtension(t *testing.T) {
	id := component.NewID(extensiontest.NopType)
	cfg := nopConfig(t, id)

	exts, err := New(t.Context(), cfg, nopFactories(), componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)

	got := exts.GetExtensions()
	require.Len(t, got, 1)
	require.Contains(t, got, id)

	require.NoError(t, exts.Start(t.Context()))
	require.NoError(t, exts.Shutdown(t.Context()))
}

func TestNew_NamedExtension(t *testing.T) {
	id := component.MustNewIDWithName(extensiontest.NopType.String(), "myext")
	cfg := nopConfig(t, id)

	exts, err := New(t.Context(), cfg, nopFactories(), componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)

	require.Contains(t, exts.GetExtensions(), id)
}

func TestNew_UnknownFactoryType(t *testing.T) {
	// Manually construct a Config bypassing Unmarshal so we exercise the
	// defensive lookup inside New.
	id := component.MustNewID("doesnotexist")
	cfg := Config{id: struct{}{}}

	_, err := New(t.Context(), cfg, map[component.Type]extension.Factory{}, componenttest.NewNopTelemetrySettings())
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown extension type")
	require.Contains(t, err.Error(), "doesnotexist")
}

func TestNew_MultipleExtensionsDeterministicOrder(t *testing.T) {
	cfg := nopConfig(t,
		component.MustNewIDWithName(extensiontest.NopType.String(), "c"),
		component.MustNewIDWithName(extensiontest.NopType.String(), "a"),
		component.MustNewIDWithName(extensiontest.NopType.String(), "b"),
	)

	exts, err := New(t.Context(), cfg, nopFactories(), componenttest.NewNopTelemetrySettings())
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
	cfg := Config{
		component.NewID(firstType):  firstFactory.CreateDefaultConfig(),
		component.NewID(secondType): secondFactory.CreateDefaultConfig(),
	}

	exts, err := New(t.Context(), cfg, factories, componenttest.NewNopTelemetrySettings())
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
	cfg := Config{
		component.NewID(aType): &struct{}{},
		component.NewID(bType): &struct{}{},
		component.NewID(cType): &struct{}{},
	}

	exts, err := New(t.Context(), cfg, factories, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)

	require.NoError(t, exts.Start(t.Context()))
	require.NoError(t, exts.Shutdown(t.Context()))
	require.Equal(t, []string{"c", "b", "a"}, order)
}

func TestConfig_Unmarshal(t *testing.T) {
	tests := []struct {
		name        string
		rawCfg      map[string]any
		wantErr     bool
		errContains string
		wantIDs     []component.ID
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
			wantIDs: []component.ID{component.NewID(extensiontest.NopType)},
		},
		{
			name: "valid named nop",
			rawCfg: map[string]any{
				"nop/myext": map[string]any{},
			},
			wantIDs: []component.ID{component.MustNewIDWithName(extensiontest.NopType.String(), "myext")},
		},
		{
			name: "multiple valid entries",
			rawCfg: map[string]any{
				"nop":   map[string]any{},
				"nop/a": map[string]any{},
				"nop/b": map[string]any{},
			},
			wantIDs: []component.ID{
				component.NewID(extensiontest.NopType),
				component.MustNewIDWithName(extensiontest.NopType.String(), "a"),
				component.MustNewIDWithName(extensiontest.NopType.String(), "b"),
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
			var cfg Config
			err := cfg.Unmarshal(confmap.NewFromStringMap(tc.rawCfg))
			if tc.wantErr {
				require.Error(t, err)
				if tc.errContains != "" {
					require.Contains(t, err.Error(), tc.errContains)
				}
				return
			}
			require.NoError(t, err)
			for _, id := range tc.wantIDs {
				require.Contains(t, cfg, id)
			}
			require.Len(t, cfg, len(tc.wantIDs))
		})
	}
}

// TestConfig_TopLevelValidate confirms that confmap.Validate walks into a
// Config map and reports the failing field path with the extension ID, so we
// don't need to wrap per-extension errors ourselves.
func TestConfig_TopLevelValidate(t *testing.T) {
	type parent struct {
		Extensions Config `mapstructure:"extensions"`
	}

	id := component.MustNewIDWithName("requiredfield", "primary")
	p := parent{
		Extensions: Config{id: &requiredFieldConfig{}},
	}

	err := confmap.Validate(p)
	require.Error(t, err)
	// Path should include the extensions key, the component ID, and the field.
	require.Contains(t, err.Error(), "extensions::requiredfield/primary")
	require.Contains(t, err.Error(), "field is required")
}

// requiredFieldConfig is a tiny config type that fails Validate when its
// required field is empty. It exists only to verify path-prefixed error
// formatting from confmap.Validate.
type requiredFieldConfig struct {
	Field string `mapstructure:"field"`
}

func (c *requiredFieldConfig) Validate() error {
	if c.Field == "" {
		return errors.New("field is required")
	}
	return nil
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
