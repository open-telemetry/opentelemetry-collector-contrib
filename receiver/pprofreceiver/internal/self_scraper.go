// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pprofreceiver/internal"

import (
	"bufio"
	"bytes"
	"context"
	"runtime"
	"runtime/pprof"

	"github.com/google/pprof/profile"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pprofile"
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
}

func (hcs *SelfScraper) Start(_ context.Context, _ component.Host) error {
	runtime.SetBlockProfileRate(hcs.BlockProfileFraction)
	runtime.SetMutexProfileFraction(hcs.MutexProfileFraction)
	hcs.buf = bytes.NewBuffer(make([]byte, 0, 8096))
	hcs.writer = bufio.NewWriter(hcs.buf)
	err := pprof.StartCPUProfile(hcs.writer)
	return err
}

func (*SelfScraper) Shutdown(_ context.Context) error {
	pprof.StopCPUProfile()
	// Disable block and mutex profiling when the scraper shuts down.
	runtime.SetBlockProfileRate(0)
	runtime.SetMutexProfileFraction(0)
	return nil
}

func (hcs *SelfScraper) ScrapeProfiles(_ context.Context) (pprofile.Profiles, error) {
	pprof.StopCPUProfile()
	_ = hcs.writer.Flush()
	cpuProfile, parseErr := profile.Parse(hcs.buf)
	hcs.buf.Reset()

	result := pprofile.NewProfiles()

	// Collect and convert CPU profile
	if parseErr == nil && cpuProfile != nil {
		p, err := translator.ConvertPprofToProfiles(cpuProfile)
		if err != nil {
			parseErr = err
		} else if p != nil {
			if err := p.MergeTo(result); err != nil {
				parseErr = err
			}
		}
	}

	// Collect and convert block profile if configured
	if hcs.BlockProfileFraction > 0 {
		if blockProfile := pprof.Lookup("block"); blockProfile != nil {
			blockBuf := bytes.NewBuffer(make([]byte, 0, 4096))
			if err := blockProfile.WriteTo(blockBuf, 0); err == nil {
				blockProfileParsed, err := profile.Parse(blockBuf)
				if err == nil && blockProfileParsed != nil {
					p, err := translator.ConvertPprofToProfiles(blockProfileParsed)
					if err == nil && p != nil {
						_ = p.MergeTo(result)
					}
				}
			}
		}
	}

	// Collect and convert mutex profile if configured
	if hcs.MutexProfileFraction > 0 {
		if mutexProfile := pprof.Lookup("mutex"); mutexProfile != nil {
			mutexBuf := bytes.NewBuffer(make([]byte, 0, 4096))
			if err := mutexProfile.WriteTo(mutexBuf, 0); err == nil {
				mutexProfileParsed, err := profile.Parse(mutexBuf)
				if err == nil && mutexProfileParsed != nil {
					p, err := translator.ConvertPprofToProfiles(mutexProfileParsed)
					if err == nil && p != nil {
						_ = p.MergeTo(result)
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

	_ = pprof.StartCPUProfile(hcs.writer)
	if result.ProfileCount() == 0 {
		return pprofile.Profiles{}, parseErr
	}
	return result, parseErr
}
