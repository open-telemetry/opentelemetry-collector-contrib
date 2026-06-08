// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunk // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/splunk"

import (
	"strconv"
	"time"

	"github.com/google/pprof/profile"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"
)

const (
	// Splunk AlwaysOn Profiling sample label keys. Their semantics and types are part of the
	// profiling backend contract (see comment on AddProfilingPprofSampleLabels):
	//   - sourceEventNameLabel (string) optionally names the event that triggered the sampling.
	//   - sourceEventTimeLabel (int64) MUST be the unix time in milliseconds when the sample was taken.
	//   - sourceEventPeriodLabel (string) holds the sampling period for cpu profiles.
	sourceEventNameLabel   = "source.event.name"
	sourceEventTimeLabel   = "source.event.time"
	sourceEventPeriodLabel = "source.event.period"
)

// AddProfilingPprofSampleLabels adds Splunk AlwaysOn Profiling labels to pprof samples.
//
// prof is expected to be normalized so that each pprof sample maps 1:1 to a pprofile.Sample carrying at most one entry
// in timestamps_unix_nano. source.event.time is taken from that per-observation timestamp when present,
// falling back to the Profile time.
func AddProfilingPprofSampleLabels(p *profile.Profile, dict pprofile.ProfilesDictionary, scope pcommon.InstrumentationScope, prof pprofile.Profile) {
	profileTime := prof.Time().AsTime()
	if profileTime.UnixNano() == 0 {
		profileTime = time.Now()
	}

	sampleType := dict.StringTable().At(int(prof.SampleType().TypeStrindex()))
	for sampleIdx, sample := range p.Sample {
		if sample.Label == nil {
			sample.Label = map[string][]string{}
		}
		if sample.NumLabel == nil {
			sample.NumLabel = map[string][]int64{}
		}

		eventTime := profileTime
		if sampleIdx < prof.Samples().Len() {
			if ts := prof.Samples().At(sampleIdx).TimestampsUnixNano(); ts.Len() > 0 {
				eventTime = time.Unix(0, int64(ts.At(0)))
			}
		}

		sample.Label[sourceEventNameLabel] = []string{scope.Name()}
		// source.event.time MUST be the unix time in milliseconds; the backend rejects other units.
		sample.NumLabel[sourceEventTimeLabel] = []int64{eventTime.UnixMilli()}
		if sampleType == "cpu" {
			sample.Label[sourceEventPeriodLabel] = []string{strconv.Itoa(int(prof.Period()))}
		}

		if sampleIdx >= prof.Samples().Len() {
			continue
		}
		if li := prof.Samples().At(sampleIdx).LinkIndex(); li > 0 && int(li) < dict.LinkTable().Len() {
			link := dict.LinkTable().At(int(li))
			sample.Label[spanIDFieldKey] = []string{link.SpanID().String()}
			sample.Label[traceIDFieldKey] = []string{link.TraceID().String()}
		}
	}
}
