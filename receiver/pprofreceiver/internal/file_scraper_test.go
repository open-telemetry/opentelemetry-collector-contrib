// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestScrapeMultipleFilesMergesDictionaries(t *testing.T) {
	// Scrape two heap profiles with the same sample types but different
	// dictionary content (different function names and file names).
	s := FileScraper{
		Logger:  zap.NewNop(),
		Include: filepath.Join("testdata", "pprof.otelcol.alloc_objects.alloc_space.inuse_objects.inuse_space.{001,002}.pb.gz"),
	}
	result, err := s.Scrape(t.Context())
	require.NoError(t, err)
	require.Positive(t, result.ProfileCount())

	// Collect all strings from the merged dictionary.
	dict := result.Dictionary().StringTable()
	strings := make(map[string]bool, dict.Len())
	for i := 0; i < dict.Len(); i++ {
		strings[dict.At(i)] = true
	}

	// Both files' dictionary entries must be present in the merged result.
	// The old MoveAndAppendTo skipped dictionary merging, so the second
	// file's profiles would reference indices into a dictionary that did
	// not contain its strings.
	assert.True(t, strings["betaFunction"], "merged dictionary missing string from second file")
	assert.True(t, strings["beta.go"], "merged dictionary missing string from second file")
	assert.True(t, strings["inuse_objects"], "merged dictionary missing string from first file")
}
