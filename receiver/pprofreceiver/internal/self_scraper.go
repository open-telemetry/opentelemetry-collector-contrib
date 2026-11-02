// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pprofreceiver/internal"

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"runtime"
	"runtime/pprof"

	"github.com/google/pprof/profile"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.opentelemetry.io/collector/scraper/xscraper"

	translator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/pprof"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pprofreceiver/internal/metadata"
)

var _ xscraper.Profiles = &SelfScraper{}

type SelfScraper struct {
	BlockProfileFraction int
	MutexProfileFraction int
	BuildInfo            component.BuildInfo
	buf                  *bytes.Buffer
	writer               *bufio.Writer
	// savedMutexFraction stores the mutex profiling fraction before Start
	// so it can be restored in Shutdown. Block profiling rate cannot be
	// read back from the Go runtime API, so it is not saved.
	savedMutexFraction int
}

func (hcs *SelfScraper) Start(_ context.Context, _ component.Host) error {
	// Save the current mutex profiling fraction before overwriting it.
	// SetMutexProfileFraction(-1) returns the previous value without changing it.
	hcs.savedMutexFraction = runtime.SetMutexProfileFraction(-1)
	runtime.SetBlockProfileRate(hcs.BlockProfileFraction)
	runtime.SetMutexProfileFraction(hcs.MutexProfileFraction)
	hcs.buf = bytes.NewBuffer(make([]byte, 0, 8096))
	hcs.writer = bufio.NewWriter(hcs.buf)
	err := pprof.StartCPUProfile(hcs.writer)
	return err
}

func (hcs *SelfScraper) Shutdown(_ context.Context) error {
	pprof.StopCPUProfile()
	// Restore the mutex profiling fraction to its pre-Start value.
	// Block profiling rate cannot be read from the Go runtime API,
	// so we disable it (set to 0) as a safe default. This means
	// if another component configured block profiling, it will be
	// disabled when this receiver shuts down. This is a known
	// limitation due to the Go runtime API.
	runtime.SetBlockProfileRate(0)
	runtime.SetMutexProfileFraction(hcs.savedMutexFraction)
	return nil
}

func (hcs *SelfScraper) ScrapeProfiles(_ context.Context) (pprofile.Profiles, error) {
	pprof.StopCPUProfile()
	_ = hcs.writer.Flush()
	cpuProfile, cpuParseErr := profile.Parse(hcs.buf)
	hcs.buf.Reset()

	result := pprofile.NewProfiles()
	var scrapeErrors []error
	failedProfiles := 0

	// Collect and convert CPU profile
	if cpuParseErr == nil && cpuProfile != nil {
		p, err := translator.ConvertPprofToProfiles(cpuProfile)
		if err != nil {
			scrapeErrors = append(scrapeErrors, fmt.Errorf("failed to convert CPU profile: %w", err))
			failedProfiles++
		} else if p != nil {
			if err := p.MergeTo(result); err != nil {
				scrapeErrors = append(scrapeErrors, fmt.Errorf("failed to merge CPU profile: %w", err))
				failedProfiles++
			}
		}
	} else if cpuParseErr != nil {
		scrapeErrors = append(scrapeErrors, fmt.Errorf("failed to parse CPU profile: %w", cpuParseErr))
		failedProfiles++
	}

	// Collect and convert block profile if configured
	// Do this BEFORE restarting CPU profiling
	if hcs.BlockProfileFraction > 0 {
		if blockProfile := pprof.Lookup("block"); blockProfile != nil {
			blockBuf := bytes.NewBuffer(make([]byte, 0, 4096))
			if err := blockProfile.WriteTo(blockBuf, 0); err != nil {
				scrapeErrors = append(scrapeErrors, fmt.Errorf("failed to write block profile: %w", err))
				failedProfiles++
			} else {
				blockProfileParsed, err := profile.Parse(blockBuf)
				if err != nil {
					scrapeErrors = append(scrapeErrors, fmt.Errorf("failed to parse block profile: %w", err))
					failedProfiles++
				} else if blockProfileParsed != nil {
					p, err := translator.ConvertPprofToProfiles(blockProfileParsed)
					if err != nil {
						scrapeErrors = append(scrapeErrors, fmt.Errorf("failed to convert block profile: %w", err))
						failedProfiles++
					} else if p != nil {
						if err := p.MergeTo(result); err != nil {
							scrapeErrors = append(scrapeErrors, fmt.Errorf("failed to merge block profile: %w", err))
							failedProfiles++
						}
					}
				}
			}
		}
	}

	// Collect and convert mutex profile if configured
	// Do this BEFORE restarting CPU profiling
	if hcs.MutexProfileFraction > 0 {
		if mutexProfile := pprof.Lookup("mutex"); mutexProfile != nil {
			mutexBuf := bytes.NewBuffer(make([]byte, 0, 4096))
			if err := mutexProfile.WriteTo(mutexBuf, 0); err != nil {
				scrapeErrors = append(scrapeErrors, fmt.Errorf("failed to write mutex profile: %w", err))
				failedProfiles++
			} else {
				mutexProfileParsed, err := profile.Parse(mutexBuf)
				if err != nil {
					scrapeErrors = append(scrapeErrors, fmt.Errorf("failed to parse mutex profile: %w", err))
					failedProfiles++
				} else if mutexProfileParsed != nil {
					p, err := translator.ConvertPprofToProfiles(mutexProfileParsed)
					if err != nil {
						scrapeErrors = append(scrapeErrors, fmt.Errorf("failed to convert mutex profile: %w", err))
						failedProfiles++
					} else if p != nil {
						if err := p.MergeTo(result); err != nil {
							scrapeErrors = append(scrapeErrors, fmt.Errorf("failed to merge mutex profile: %w", err))
							failedProfiles++
						}
					}
				}
			}
		}
	}

	// Set scope name and version for all profiles
	if result.ProfileCount() > 0 {
		name := metadata.ScopeName + "/selfscraper"
		version := hcs.BuildInfo.Version
		for i := 0; i < result.ResourceProfiles().Len(); i++ {
			rp := result.ResourceProfiles().At(i)
			for j := 0; j < rp.ScopeProfiles().Len(); j++ {
				sp := rp.ScopeProfiles().At(j)
				sp.Scope().SetName(name)
				sp.Scope().SetVersion(version)
			}
		}
	}

	// Restart CPU profiling before returning
	_ = pprof.StartCPUProfile(hcs.writer)

	// Determine error handling
	if len(scrapeErrors) > 0 {
		if result.ProfileCount() > 0 {
			// Partial success: return profiles with partial-scrape error
			return result, scrapererror.NewPartialScrapeError(errors.Join(scrapeErrors...), failedProfiles)
		}
		// Complete failure: return empty profiles with combined error
		return pprofile.Profiles{}, errors.Join(scrapeErrors...)
	}

	return result, nil
}
