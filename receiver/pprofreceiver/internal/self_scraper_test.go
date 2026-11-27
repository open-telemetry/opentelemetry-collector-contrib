// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"bytes"
	"runtime"
	runtimepprof "runtime/pprof"
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

	sum := 0
	for range 1_000_000 {
		sum++
	}
	_ = sum

	p, err := s.ScrapeProfiles(t.Context())
	require.NoError(t, err)
	require.NotEqual(t, 0, p.ProfileCount())

	// Inspect ALL ResourceProfiles
	foundCPUProfile := false
	for i := 0; i < p.ResourceProfiles().Len(); i++ {
		rp := p.ResourceProfiles().At(i)
		for j := 0; j < rp.ScopeProfiles().Len(); j++ {
			sp := rp.ScopeProfiles().At(j)
			// Verify scope name and version for the first ScopeProfile in the first ResourceProfile
			if i == 0 && j == 0 {
				require.Equal(t, metadata.ScopeName+"/selfscraper", sp.Scope().Name())
				require.Equal(t, "1.2.3", sp.Scope().Version())
			}
			for k := 0; k < sp.Profiles().Len(); k++ {
				prof := sp.Profiles().At(k)
				typeIdx := int(prof.SampleType().TypeStrindex())
				sampleType := p.Dictionary().StringTable().At(typeIdx)
				if sampleType == "samples" || sampleType == "cpu" {
					foundCPUProfile = true
				}
			}
		}
	}
	require.True(t, foundCPUProfile, "CPU profile should be collected")
}

// Test B: Block profile is collected
func TestScrapeSelf_BlockProfile(t *testing.T) {
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

	runtime.SetBlockProfileRate(1)

	var mu sync.Mutex
	var cond sync.Cond
	cond.L = &mu
	var wg sync.WaitGroup
	readyCh := make(chan struct{}, 1000)

	for range 1000 {
		wg.Go(func() {
			mu.Lock()
			readyCh <- struct{}{}
			cond.Wait()
			mu.Unlock()
		})
	}

	for range 1000 {
		<-readyCh
	}

	time.Sleep(200 * time.Millisecond)

	// Scrape while goroutines are still blocked
	p, err := s.ScrapeProfiles(t.Context())
	require.NoError(t, err)
	require.NotEqual(t, 0, p.ProfileCount())

	mu.Lock()
	cond.Broadcast()
	mu.Unlock()
	wg.Wait()

	// Inspect ALL ResourceProfiles (block/mutex may be in different ResourceProfiles)
	foundCPUProfile := false
	foundBlockProfile := false
	for i := 0; i < p.ResourceProfiles().Len(); i++ {
		rp := p.ResourceProfiles().At(i)
		for j := 0; j < rp.ScopeProfiles().Len(); j++ {
			sp := rp.ScopeProfiles().At(j)
			for k := 0; k < sp.Profiles().Len(); k++ {
				prof := sp.Profiles().At(k)
				typeIdx := int(prof.SampleType().TypeStrindex())
				sampleType := p.Dictionary().StringTable().At(typeIdx)
				if sampleType == "samples" || sampleType == "cpu" {
					foundCPUProfile = true
				}
				if sampleType == "contentions" || sampleType == "delay" {
					foundBlockProfile = true
				}
			}
		}
	}
	require.True(t, foundCPUProfile, "CPU profile should be collected")
	require.True(t, foundBlockProfile, "Block profile should be collected with contentions or delay sample type")
}

// Test C: Mutex profile is collected
func TestScrapeSelf_MutexProfile(t *testing.T) {
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

	var mu sync.Mutex
	var wg sync.WaitGroup
	readyCh := make(chan struct{}, 100)

	for range 100 {
		wg.Go(func() {
			mu.Lock()
			readyCh <- struct{}{}
			time.Sleep(10 * time.Millisecond)
			mu.Unlock()
		})
	}

	for range 100 {
		<-readyCh
	}

	// Scrape while contention is ongoing
	p, err := s.ScrapeProfiles(t.Context())
	require.NoError(t, err)
	require.NotEqual(t, 0, p.ProfileCount())

	wg.Wait()

	// Inspect ALL ResourceProfiles (block/mutex may be in different ResourceProfiles)
	foundCPUProfile := false
	foundMutexProfile := false
	for i := 0; i < p.ResourceProfiles().Len(); i++ {
		rp := p.ResourceProfiles().At(i)
		for j := 0; j < rp.ScopeProfiles().Len(); j++ {
			sp := rp.ScopeProfiles().At(j)
			for k := 0; k < sp.Profiles().Len(); k++ {
				prof := sp.Profiles().At(k)
				typeIdx := int(prof.SampleType().TypeStrindex())
				sampleType := p.Dictionary().StringTable().At(typeIdx)
				if sampleType == "samples" || sampleType == "cpu" {
					foundCPUProfile = true
				}
				if sampleType == "contentions" || sampleType == "delay" {
					foundMutexProfile = true
				}
			}
		}
	}
	require.True(t, foundCPUProfile, "CPU profile should be collected")
	require.True(t, foundMutexProfile, "Mutex profile should be collected with contentions or delay sample type")
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

	sp := p.ResourceProfiles().At(0).ScopeProfiles().At(0)
	require.Equal(t, 2, sp.Profiles().Len(), "CPU profile should have 2 sample types when block/mutex are disabled")

	for i := 0; i < sp.Profiles().Len(); i++ {
		prof := sp.Profiles().At(i)
		typeIdx := int(prof.SampleType().TypeStrindex())
		sampleType := p.Dictionary().StringTable().At(typeIdx)
		require.Contains(t, []string{"cpu", "samples"}, sampleType,
			"Only CPU sample types should be present when block/mutex are disabled")
	}
}

// Test E: Shutdown without scraping still restores state
func TestScrapeSelf_ShutdownWithoutScrape(t *testing.T) {
	originalFraction := runtime.SetMutexProfileFraction(-1)
	t.Cleanup(func() {
		runtime.SetMutexProfileFraction(originalFraction)
	})

	runtime.SetMutexProfileFraction(10)

	s := SelfScraper{
		BlockProfileFraction: 1,
		MutexProfileFraction: 5,
		BuildInfo:            component.BuildInfo{Version: "1.2.3"},
	}

	err := s.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	err = s.Shutdown(t.Context())
	require.NoError(t, err)

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

	sum := 0
	for range 1_000_000 {
		sum++
	}
	_ = sum

	runtime.SetBlockProfileRate(1)

	var mu sync.Mutex
	var cond sync.Cond
	cond.L = &mu
	var wg sync.WaitGroup
	readyCh := make(chan struct{}, 200)

	for range 100 {
		wg.Go(func() {
			mu.Lock()
			readyCh <- struct{}{}
			cond.Wait()
			mu.Unlock()
		})
	}

	for range 100 {
		wg.Go(func() {
			mu.Lock()
			readyCh <- struct{}{}
			time.Sleep(10 * time.Millisecond)
			mu.Unlock()
		})
	}

	for range 200 {
		<-readyCh
	}

	time.Sleep(50 * time.Millisecond)

	p, err := s.ScrapeProfiles(t.Context())
	require.NoError(t, err)
	require.NotEqual(t, 0, p.ProfileCount())

	mu.Lock()
	cond.Broadcast()
	mu.Unlock()
	wg.Wait()

	// Inspect ALL ResourceProfiles (block/mutex may be in different ResourceProfiles)
	foundCPUProfile := false
	foundBlockProfile := false
	foundMutexProfile := false
	for i := 0; i < p.ResourceProfiles().Len(); i++ {
		rp := p.ResourceProfiles().At(i)
		for j := 0; j < rp.ScopeProfiles().Len(); j++ {
			sp := rp.ScopeProfiles().At(j)
			for k := 0; k < sp.Profiles().Len(); k++ {
				prof := sp.Profiles().At(k)
				typeIdx := int(prof.SampleType().TypeStrindex())
				sampleType := p.Dictionary().StringTable().At(typeIdx)
				if sampleType == "samples" || sampleType == "cpu" {
					foundCPUProfile = true
				}
				if sampleType == "contentions" || sampleType == "delay" {
					if foundBlockProfile {
						foundMutexProfile = true
					} else {
						foundBlockProfile = true
					}
				}
			}
		}
	}
	require.True(t, foundCPUProfile, "CPU profile should be collected")
	require.True(t, foundBlockProfile, "Block profile should be collected")
	require.True(t, foundMutexProfile, "Mutex profile should be collected")
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

	p1, err := s.ScrapeProfiles(t.Context())
	require.NoError(t, err)
	require.NotEqual(t, 0, p1.ProfileCount())

	p2, err := s.ScrapeProfiles(t.Context())
	require.NoError(t, err)
	require.NotEqual(t, 0, p2.ProfileCount())
}

// Test: Startup failure when CPU profiling is already started
func TestScrapeSelf_StartupFailure(t *testing.T) {
	var buf bytes.Buffer
	err := runtimepprof.StartCPUProfile(&buf)
	require.NoError(t, err)
	defer runtimepprof.StopCPUProfile()

	s := SelfScraper{
		BlockProfileFraction: 0,
		MutexProfileFraction: 0,
		BuildInfo:            component.BuildInfo{Version: "1.2.3"},
	}

	err = s.Start(t.Context(), componenttest.NewNopHost())
	require.Error(t, err, "Start should fail when CPU profiling is already active")
}

// Test: Verify mutex profiling fraction is restored after Shutdown
func TestScrapeSelf_MutexFractionRestored(t *testing.T) {
	originalFraction := runtime.SetMutexProfileFraction(-1)
	t.Cleanup(func() {
		runtime.SetMutexProfileFraction(originalFraction)
	})

	runtime.SetMutexProfileFraction(10)

	s := SelfScraper{
		BlockProfileFraction: 0,
		MutexProfileFraction: 5,
		BuildInfo:            component.BuildInfo{Version: "1.2.3"},
	}

	err := s.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	currentFraction := runtime.SetMutexProfileFraction(-1)
	require.Equal(t, 5, currentFraction, "Scraper should have set mutex fraction to 5")

	err = s.Shutdown(t.Context())
	require.NoError(t, err)

	restoredFraction := runtime.SetMutexProfileFraction(-1)
	require.Equal(t, 10, restoredFraction, "Shutdown should restore mutex fraction to 10")
}
