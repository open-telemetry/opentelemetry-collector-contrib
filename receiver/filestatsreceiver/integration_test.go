// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package filestatsreceiver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/scraperinttest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func Test_Integration(t *testing.T) {
	expectedFile := filepath.Join("testdata", "integration", "expected.yaml")
	tempDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "foo.txt"), []byte("foo"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "bar.txt"), []byte("bar"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "foobar.txt"), []byte("foobar"), 0o600))
	scraperinttest.NewIntegrationTest(
		NewFactory(),
		scraperinttest.WithCustomConfig(
			func(_ *testing.T, cfg component.Config, _ *scraperinttest.ContainerInfo) {
				rCfg := cfg.(*Config)
				rCfg.CollectionInterval = time.Second
				rCfg.Include = filepath.Join(tempDir, "foo*")
				rCfg.ResourceAttributes.FilePath.Enabled = true
			}),
		scraperinttest.WithExpectedFile(expectedFile),
		scraperinttest.WithCompareOptions(
			pmetrictest.ChangeResourceAttributeValue("file.path", func(s string) string {
				return strings.TrimPrefix(s, tempDir)
			}),
			pmetrictest.IgnoreResourceMetricsOrder(),
			pmetrictest.IgnoreMetricValues(),
			pmetrictest.IgnoreMetricDataPointsOrder(),
			pmetrictest.IgnoreStartTimestamp(),
			pmetrictest.IgnoreTimestamp(),
		),
	).Run(t)
}
