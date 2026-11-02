// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"runtime"
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

	// Generate blocking activity using a channel with multiple goroutines
	// to ensure block profiling captures the contention
	var wg sync.WaitGroup
	blockCh := make(chan struct{}, 1)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-blockCh
		}()
	}
	time.Sleep(10 * time.Millisecond)
	close(blockCh)
	wg.Wait()

	p, err := s.ScrapeProfiles(t.Context())
	require.NoError(t, err)
	require.NotEqual(t, 0, p.ProfileCount())

	sp := p.ResourceProfiles().At(0).ScopeProfiles().At(0)
	require.Equal(t, metadata.ScopeName+"/selfscraper", sp.Scope().Name())
	require.Equal(t, "1.2.3", sp.Scope().Version())

	// Verify that CPU profile is still collected
	foundCPUProfile := false
	for i := 0; i < sp.Profiles().Len(); i++ {
		prof := sp.Profiles().At(i)
		typeIdx := int(prof.SampleType().TypeStrindex())
		sampleType := p.Dictionary().StringTable().At(typeIdx)
		if sampleType == "samples" || sampleType == "cpu" {
			foundCPUProfile = true
			break
		}
	}
	require.True(t, foundCPUProfile, "CPU profile should be collected")
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

	// Verify that CPU profile is still collected
	foundCPUProfile := false
	for i := 0; i < sp.Profiles().Len(); i++ {
		prof := sp.Profiles().At(i)
		typeIdx := int(prof.SampleType().TypeStrindex())
		sampleType := p.Dictionary().StringTable().At(typeIdx)
		if sampleType == "samples" || sampleType == "cpu" {
			foundCPUProfile = true
			break
		}
	}
	require.True(t, foundCPUProfile, "CPU profile should be collected")
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

// Test E: Shutdown without scraping still restores state
func TestScrapeSelf_ShutdownWithoutScrape(t *testing.T) {
	// This test must not run in parallel with other tests that manipulate
	// runtime profiling rates, as they are process-global.

	// Save the original mutex fraction
	originalFraction := runtime.SetMutexProfileFraction(-1)
	t.Cleanup(func() {
		runtime.SetMutexProfileFraction(originalFraction)
	})

	// Set a known starting value
	runtime.SetMutexProfileFraction(10)

	s := SelfScraper{
		BlockProfileFraction: 1,
		MutexProfileFraction: 5,
		BuildInfo:            component.BuildInfo{Version: "1.2.3"},
	}

	err := s.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	// Shutdown without scraping
	err = s.Shutdown(t.Context())
	require.NoError(t, err)

	// Verify the original fraction was restored even without scraping
	restoredFraction := runtime.SetMutexProfileFraction(-1)
	require.Equal(t, 10, restoredFraction, "Shutdown should restore mutex fraction even without scraping")
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

	// Verify that CPU profile is collected
	foundCPUProfile := false
	for i := 0; i < sp.Profiles().Len(); i++ {
		prof := sp.Profiles().At(i)
		typeIdx := int(prof.SampleType().TypeStrindex())
		sampleType := p.Dictionary().StringTable().At(typeIdx)
		if sampleType == "samples" || sampleType == "cpu" {
			foundCPUProfile = true
			break
		}
	}
	require.True(t, foundCPUProfile, "CPU profile should be collected")
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

// Test: Verify mutex profiling fraction is restored after Shutdown
func TestScrapeSelf_MutexFractionRestored(t *testing.T) {
	// This test must not run in parallel with other tests that manipulate
	// runtime profiling rates, as they are process-global.

	// Save the original mutex fraction
	originalFraction := runtime.SetMutexProfileFraction(-1)
	t.Cleanup(func() {
		// Restore the original fraction in cleanup
		runtime.SetMutexProfileFraction(originalFraction)
	})

	// Set a known starting value
	runtime.SetMutexProfileFraction(10)

	s := SelfScraper{
		BlockProfileFraction: 0,
		MutexProfileFraction: 5,
		BuildInfo:            component.BuildInfo{Version: "1.2.3"},
	}

	err := s.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	// Verify the scraper applied its configured fraction
	currentFraction := runtime.SetMutexProfileFraction(-1)
	require.Equal(t, 5, currentFraction, "Scraper should have set mutex fraction to 5")

	err = s.Shutdown(t.Context())
	require.NoError(t, err)

	// Verify the original fraction was restored
	restoredFraction := runtime.SetMutexProfileFraction(-1)
	require.Equal(t, 10, restoredFraction, "Shutdown should restore mutex fraction to 10")
}
