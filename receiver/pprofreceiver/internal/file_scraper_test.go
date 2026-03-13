// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestScrapeFile(t *testing.T) {
	s := FileScraper{
		Logger:  zap.NewNop(),
		Include: filepath.Join("testdata", "pprof.otelcol.alloc_objects.alloc_space.inuse_objects.inuse_space.001.pb.gz"),
	}
	p, err := s.Scrape(t.Context())
	require.NoError(t, err)
	require.NotEqual(t, 0, p.ProfileCount())
}
