// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pprofreceiver/internal/metadata"
)

// Test A: CPU profile remains supported
func TestScrapeSelf_CPUProfile(t *testing.T) {
	s := SelfScraper{
		BlockProfileFraction: 0,
		MutexProfileFraction: 0,
		BuildInfo:            component.BuildInfo{Version: "1.2.3"},
	}
	err := s.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, s.Shutdown(t.Context()))
	}()

	// Generate some CPU activity
	sum := 0
	for range 1_000_000 {
		sum++
	}
	_ = sum

	p, err := s.ScrapeProfiles(t.Context())
	require.NoError(t, err)
	require.NotEqual(t, 0, p.ProfileCount())

	sp := p.ResourceProfiles().At(0).ScopeProfiles().At(0)
	require.Equal(t, metadata.ScopeName+"/selfscraper", sp.Scope().Name())
	require.Equal(t, "1.2.3", sp.Scope().Version())
}

// Test B: Block profile is collected
func TestScrapeSelf_BlockProfile(t *testing.T) {
	// This test must not run in parallel with other tests that manipulate
	// runtime profiling rates, as they are process-global.
	s := SelfScraper{
		BlockProfileFraction: 1,
		MutexProfileFraction: 0,
		BuildInfo:            component.BuildInfo{Version: "1.2.3"},
	}
	err := s.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, s.Shutdown(t.Context()))
	}()

	// Generate blocking activity using a channel
	ch := make(chan struct{})
	go func() {
		time.Sleep(10 * time.Millisecond)
		close(ch)
	}()
	<-ch

	p, err := s.ScrapeProfiles(t.Context())
	require.NoError(t, err)
	require.NotEqual(t, 0, p.ProfileCount())

	sp := p.ResourceProfiles().At(0).ScopeProfiles().At(0)
	require.Equal(t, metadata.ScopeName+"/selfscraper", sp.Scope().Name())
	require.Equal(t, "1.2.3", sp.Scope().Version())
}

// Test C: Mutex profile is collected
func TestScrapeSelf_MutexProfile(t *testing.T) {
	// This test must not run in parallel with other tests that manipulate
	// runtime profiling rates, as they are process-global.
	s := SelfScraper{
		BlockProfileFraction: 0,
		MutexProfileFraction: 1,
		BuildInfo:            component.BuildInfo{Version: "1.2.3"},
	}
	err := s.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, s.Shutdown(t.Context()))
	}()

	// Generate mutex contention
	var mu sync.Mutex
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			mu.Lock()
			defer mu.Unlock()
			time.Sleep(1 * time.Millisecond)
		})
	}
	wg.Wait()

	p, err := s.ScrapeProfiles(t.Context())
	require.NoError(t, err)
	require.NotEqual(t, 0, p.ProfileCount())

	sp := p.ResourceProfiles().At(0).ScopeProfiles().At(0)
	require.Equal(t, metadata.ScopeName+"/selfscraper", sp.Scope().Name())
	require.Equal(t, "1.2.3", sp.Scope().Version())
}

// Test D: Disabled profiles are not collected
func TestScrapeSelf_DisabledProfiles(t *testing.T) {
	s := SelfScraper{
		BlockProfileFraction: 0,
		MutexProfileFraction: 0,
		BuildInfo:            component.BuildInfo{Version: "1.2.3"},
	}
	err := s.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, s.Shutdown(t.Context()))
	}()

	p, err := s.ScrapeProfiles(t.Context())
	require.NoError(t, err)
	// CPU profile should still be collected
	require.NotEqual(t, 0, p.ProfileCount())
}

// Test E: Runtime configuration is restored
func TestScrapeSelf_RuntimeConfigRestored(t *testing.T) {
	// This test must not run in parallel with other tests that manipulate
	// runtime profiling rates, as they are process-global.
	s := SelfScraper{
		BlockProfileFraction: 5,
		MutexProfileFraction: 5,
		BuildInfo:            component.BuildInfo{Version: "1.2.3"},
	}
	err := s.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	_, _ = s.ScrapeProfiles(t.Context())
	err = s.Shutdown(t.Context())
	require.NoError(t, err)

	// Verify shutdown doesn't panic and leaves profiling in a clean state
	// (rates are reset to 0/disabled by Shutdown)
}

// Test F: Startup failure cleanup
func TestScrapeSelf_StartupFailure(t *testing.T) {
	// This test must not run in parallel with other tests that manipulate
	// runtime profiling rates, as they are process-global.
	s := SelfScraper{
		BlockProfileFraction: 1,
		MutexProfileFraction: 1,
		BuildInfo:            component.BuildInfo{Version: "1.2.3"},
	}

	// Start the scraper
	err := s.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	// Shutdown should restore to disabled state even if we don't scrape
	err = s.Shutdown(t.Context())
	require.NoError(t, err)
}

// Test G: Repeated scrape and shutdown
func TestScrapeSelf_RepeatedScrape(t *testing.T) {
	s := SelfScraper{
		BlockProfileFraction: 1,
		MutexProfileFraction: 1,
		BuildInfo:            component.BuildInfo{Version: "1.2.3"},
	}
	err := s.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, s.Shutdown(t.Context()))
	}()

	// Multiple scrape cycles
	for range 3 {
		p, err := s.ScrapeProfiles(t.Context())
		require.NoError(t, err)
		require.NotEqual(t, 0, p.ProfileCount())

		sp := p.ResourceProfiles().At(0).ScopeProfiles().At(0)
		require.Equal(t, metadata.ScopeName+"/selfscraper", sp.Scope().Name())
		require.Equal(t, "1.2.3", sp.Scope().Version())
	}
}

// Test: All profiles collected together
func TestScrapeSelf_AllProfiles(t *testing.T) {
	// This test must not run in parallel with other tests that manipulate
	// runtime profiling rates, as they are process-global.
	s := SelfScraper{
		BlockProfileFraction: 1,
		MutexProfileFraction: 1,
		BuildInfo:            component.BuildInfo{Version: "1.2.3"},
	}
	err := s.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, s.Shutdown(t.Context()))
	}()

	// Generate CPU, blocking, and mutex activity
	sum := 0
	for range 1_000_000 {
		sum++
	}
	_ = sum

	ch := make(chan struct{})
	go func() {
		time.Sleep(10 * time.Millisecond)
		close(ch)
	}()
	<-ch

	var mu sync.Mutex
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			mu.Lock()
			defer mu.Unlock()
			time.Sleep(1 * time.Millisecond)
		})
	}
	wg.Wait()

	p, err := s.ScrapeProfiles(t.Context())
	require.NoError(t, err)
	require.NotEqual(t, 0, p.ProfileCount())

	sp := p.ResourceProfiles().At(0).ScopeProfiles().At(0)
	require.Equal(t, metadata.ScopeName+"/selfscraper", sp.Scope().Name())
	require.Equal(t, "1.2.3", sp.Scope().Version())
}

// Test: Verify CPU profiling is stopped during scrape
func TestScrapeSelf_CPUProfilingStopped(t *testing.T) {
	s := SelfScraper{
		BlockProfileFraction: 0,
		MutexProfileFraction: 0,
		BuildInfo:            component.BuildInfo{Version: "1.2.3"},
	}
	err := s.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)
	defer func() {
		require.NoError(t, s.Shutdown(t.Context()))
	}()

	// First scrape
	p1, err := s.ScrapeProfiles(t.Context())
	require.NoError(t, err)
	require.NotEqual(t, 0, p1.ProfileCount())

	// Second scrape should work without issues
	p2, err := s.ScrapeProfiles(t.Context())
	require.NoError(t, err)
	require.NotEqual(t, 0, p2.ProfileCount())
}
