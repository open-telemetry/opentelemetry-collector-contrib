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
	return nil
}

func (hcs *SelfScraper) ScrapeProfiles(_ context.Context) (pprofile.Profiles, error) {
	pprof.StopCPUProfile()
	_ = hcs.writer.Flush()
	pprofProfile, parseErr := profile.Parse(hcs.buf)
	hcs.buf.Reset()
	if parseErr == nil {
		p, err := translator.ConvertPprofToProfiles(pprofProfile)

		if p != nil {
			name := metadata.ScopeName + "/selfscraper"
			version := hcs.BuildInfo.Version
			for i := 0; i < p.ResourceProfiles().Len(); i++ {
				rp := p.ResourceProfiles().At(i)
				for j := 0; j < rp.ScopeProfiles().Len(); j++ {
					sp := rp.ScopeProfiles().At(j)
					sp.Scope().SetName(name)
					sp.Scope().SetVersion(version)
				}
			}
		}

		_ = pprof.StartCPUProfile(hcs.writer)
		if p == nil {
			return pprofile.Profiles{}, err
		}
		return *p, err
	}

	_ = pprof.StartCPUProfile(hcs.writer)
	return pprofile.Profiles{}, parseErr
}
