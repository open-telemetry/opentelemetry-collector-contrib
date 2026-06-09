//go:build !windows

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFactoryMaps exercises all three factory-var shapes that expandFactoryMaps supports:
//   - call expr with key in internal/metadata (hostmetrics)
//   - composite literal with inline component.MustNewType key (ciscoosreceiver)
//   - composite literal with qualified pkg.Const key (geoipprocessor)
func TestFactoryMaps(t *testing.T) {
	cases := []struct {
		name         string
		dir          string
		overrideKey  string
		propertyName string
		expectedKeys []string
	}{
		{
			name:         "hostmetrics — call expr, key from internal/metadata",
			dir:          "../../../receiver/hostmetricsreceiver",
			overrideKey:  "receiver/host_metrics",
			propertyName: "scrapers",
			expectedKeys: []string{"cpu", "disk", "filesystem", "load", "memory", "network", "nfs", "paging", "process", "processes", "system"},
		},
		{
			name:         "ciscoosreceiver — composite literal, inline MustNewType key",
			dir:          "../../../receiver/ciscoosreceiver",
			overrideKey:  "receiver/cisco_os",
			propertyName: "scrapers",
			expectedKeys: []string{"interfaces", "system"},
		},
		{
			name:         "geoipprocessor — composite literal, qualified pkg.Const key",
			dir:          "../../../processor/geoipprocessor",
			overrideKey:  "processor/geoip",
			propertyName: "providers",
			expectedKeys: []string{"maxmind"},
		},
	}

	s, settingsDir, ok := ReadSettingsFile()
	require.True(t, ok, "expected .schemagen.yaml to be found")

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			absDir, err := filepath.Abs(tc.dir)
			require.NoError(t, err)

			override, found := s.ComponentOverrides[tc.overrideKey]
			require.True(t, found, "expected componentOverride for %q in .schemagen.yaml", tc.overrideKey)
			require.Len(t, override.FactoryMaps, 1)

			overrideCopy := override
			cfg := &Config{
				Mode:              Component,
				DirPath:           absDir,
				ConfigType:        "Config",
				Mappings:          s.Mappings,
				AllowedRefs:       s.AllowedRefs,
				Namespace:         s.Namespace,
				ComponentOverride: &overrideCopy,
				SettingsDir:       settingsDir,
				Pattern:           ".",
			}

			schema, err := NewParser(cfg).Parse()
			require.NoError(t, err)

			propRaw, ok := schema.Properties[tc.propertyName]
			require.True(t, ok, "schema should contain %q property", tc.propertyName)

			prop, ok := propRaw.(*ObjectSchemaElement)
			require.True(t, ok, "%q should be an ObjectSchemaElement", tc.propertyName)

			for _, key := range tc.expectedKeys {
				_, found := prop.Properties[key]
				assert.True(t, found, "expected key %q in %q property", key, tc.propertyName)
			}
			assert.Len(t, prop.Properties, len(tc.expectedKeys))
		})
	}
}
