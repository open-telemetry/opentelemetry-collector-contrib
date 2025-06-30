// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dnslookupprocessor

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/dnslookupprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/dnslookupprocessor/internal/testutil"
)

func TestProcessor(t *testing.T) {
	testCases := []struct {
		name      string
		goldenDir string
		resolve   LookupConfig
		reverse   LookupConfig
	}{
		{
			name:      "resolve source.address and reverse custom.ip",
			goldenDir: "normal",
			resolve:   defaultResolve(),
			reverse:   customReverse(),
		},
		{
			name:      "attributes not found",
			goldenDir: "attr_not_found",
			resolve:   defaultResolve(),
			reverse:   customReverse(),
		},
		{
			name:      "attributes are empty",
			goldenDir: "attr_empty",
			resolve:   defaultResolve(),
			reverse:   customReverse(),
		},
		{
			name:      "take the first valid attribute",
			goldenDir: "multiple_attrs",
			resolve: LookupConfig{
				Context:          resource,
				SourceAttributes: []string{"bad.address", "good.address"},
				TargetAttribute:  "resolved.ip",
			},
			reverse: LookupConfig{
				Context:          resource,
				SourceAttributes: []string{"bad.ip", "good.ip"},
				TargetAttribute:  "resolved.address",
			},
		},
		{
			name:      "attributes has no resolution",
			goldenDir: "no_resolution",
			resolve:   defaultResolve(),
			reverse:   customReverse(),
		},
		{
			name:      "custom resolve attributes",
			goldenDir: "custom_resolve_attr",
			resolve: LookupConfig{
				Context:          record,
				SourceAttributes: []string{"custom.address", "custom.another.address"},
				TargetAttribute:  "custom.ip",
			},
			reverse: LookupConfig{},
		},
		{
			name:      "custom reverse attributes",
			goldenDir: "custom_reverse_attr",
			resolve:   defaultResolve(),
			reverse: LookupConfig{
				Context:          record,
				SourceAttributes: []string{"custom.ip", "custom.another.ip"},
				TargetAttribute:  "custom.address",
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createNonExpiryHostsConfig(t, tt.resolve, tt.reverse)
			compareAllSignals(cfg, tt.goldenDir)(t)
		})
	}
}

func defaultResolve() LookupConfig {
	return LookupConfig{
		Context:          resource,
		SourceAttributes: []string{"source.address"},
		TargetAttribute:  "source.ip",
	}
}

func customReverse() LookupConfig {
	return LookupConfig{
		Context:          resource,
		SourceAttributes: []string{"custom.ip", "custom.another.ip"},
		TargetAttribute:  "custom.address",
	}
}

func createNonExpiryHostsConfig(t *testing.T, resolve LookupConfig, reverse LookupConfig) component.Config {
	const hostsContent = `
192.168.1.20 example.com
192.168.1.30 another.example.com
192.168.1.40 ninja.a.co ninja.b.co
192.168.1.50 ninja.a.co ninja.c.co
`
	hostFilePath := testutil.CreateTempHostFile(t, hostsContent)

	return &Config{
		Resolve:   resolve,
		Reverse:   reverse,
		Hostfiles: []string{hostFilePath},
	}
}

func compareAllSignals(cfg component.Config, goldenDir string) func(t *testing.T) {
	return func(t *testing.T) {
		dir := filepath.Join("testdata", goldenDir)
		factory := NewFactory()

		// compare logs
		logsSink := new(consumertest.LogsSink)
		logsProcessor, err := factory.CreateLogs(context.Background(), processortest.NewNopSettings(metadata.Type), cfg, logsSink)
		require.NoError(t, err)

		inputLogs, err := golden.ReadLogs(filepath.Join(dir, "input-logs.yaml"))
		require.NoError(t, err)
		expectedLogs, err := golden.ReadLogs(filepath.Join(dir, "output-logs.yaml"))
		require.NoError(t, err)

		err = logsProcessor.ConsumeLogs(context.Background(), inputLogs)
		require.NoError(t, err)

		actualLogs := logsSink.AllLogs()
		require.Len(t, actualLogs, 1)
		// golden.WriteLogs(t, filepath.Join(dir, "output-logs.yaml"), actualLogs[0])
		require.NoError(t, plogtest.CompareLogs(expectedLogs, actualLogs[0]))
		require.NoError(t, logsProcessor.Shutdown(context.Background()))
	}
}
