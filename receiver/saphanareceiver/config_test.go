// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package saphanareceiver

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/saphanareceiver/internal/metadata"
)

func TestValidate(t *testing.T) {
	testCases := []struct {
		desc                  string
		defaultConfigModifier func(cfg *Config)
		expected              error
	}{
		{
			desc:                  "missing username and password",
			defaultConfigModifier: func(*Config) {},
			expected: multierr.Combine(
				errors.New(ErrNoUsername),
				errors.New(ErrNoPassword),
			),
		},
		{
			desc: "missing password",
			defaultConfigModifier: func(cfg *Config) {
				cfg.Username = "otel"
			},
			expected: multierr.Combine(
				errors.New(ErrNoPassword),
			),
		},
		{
			desc: "missing username",
			defaultConfigModifier: func(cfg *Config) {
				cfg.Password = "otel"
			},
			expected: multierr.Combine(
				errors.New(ErrNoUsername),
			),
		},
		{
			desc: "no error",
			defaultConfigModifier: func(cfg *Config) {
				cfg.Username = "otel"
				cfg.Password = "otel"
			},
			expected: nil,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig().(*Config)
			tC.defaultConfigModifier(cfg)
			actual := confmap.Validate(cfg)

			if tC.expected != nil {
				require.ErrorContains(t, actual, tC.expected.Error())
			} else {
				require.NoError(t, actual)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	expected := factory.CreateDefaultConfig().(*Config)
	expected.MetricsBuilderConfig = metadata.NewDefaultMetricsBuilderConfig()
	expected.MetricsBuilderConfig.Metrics.SaphanaCPUUsed.Enabled = false
	expected.TCPAddrConfig.Endpoint = "example.com:30015"
	expected.Username = "otel"
	expected.Password = "password"
	expected.CollectionInterval = 2 * time.Minute

	if diff := cmp.Diff(expected, cfg,
		// mdatagen gives metric and resource attribute configs an unexported enabledSetByUser,
		// set from parser.IsSet("enabled"), so it is only true on the unmarshaled side:
		// https://github.com/open-telemetry/opentelemetry-collector/blob/e4e58cda0aa6d5d4d275ff12072ae418410e6ae7/cmd/mdatagen/internal/templates/config.go.tmpl#L42-L44
		cmp.FilterPath(
			func(fp cmp.Path) bool {
				return fp.Last().String() == ".enabledSetByUser"
			},
			cmp.Ignore(),
		),
		// Allow go-cmp to read unexported fields instead of panicking on them, so new
		// upstream fields can't break this (https://pkg.go.dev/github.com/google/go-cmp/cmp#Exporter).
		cmp.Exporter(func(reflect.Type) bool { return true })); diff != "" {
		t.Errorf("Config mismatch (-expected +actual):\n%s", diff)
	}
}
