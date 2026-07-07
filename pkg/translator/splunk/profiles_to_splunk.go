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
func AddProfilingPprofSampleLabels(p *profile.Profile, dict pprofile.ProfilesDictionary, scope pcommon.InstrumentationScope, prof pprofile.Profile) {
	profileTime := prof.Time().AsTime()
	if profileTime.UnixNano() == 0 {
		profileTime = time.Now()
	}

	typeIdx := int(prof.SampleType().TypeStrindex())
	var sampleType string
	if typeIdx >= 0 && typeIdx < dict.StringTable().Len() {
		sampleType = dict.StringTable().At(typeIdx)
	}
	emittedIdx := 0
	for srcIdx := range prof.Samples().Len() {
		if emittedIdx >= len(p.Sample) {
			return
		}
		src := prof.Samples().At(srcIdx)
		nValues := src.Values().Len()
		nTimestamps := src.TimestampsUnixNano().Len()

		obsCount := 1
		if nValues > 0 && nValues == nTimestamps {
			obsCount = nValues // shape 3: per-observation
		}

		for obs := range obsCount {
			if emittedIdx >= len(p.Sample) {
				return
			}
			sample := p.Sample[emittedIdx]
			if sample.Label == nil {
				sample.Label = map[string][]string{}
			}
			if sample.NumLabel == nil {
				sample.NumLabel = map[string][]int64{}
			}

			eventTime := profileTime
			if nTimestamps > 0 {
				tsIdx := 0
				if obsCount == nTimestamps {
					tsIdx = obs // shape 3
				}
				eventTime = time.Unix(0, int64(src.TimestampsUnixNano().At(tsIdx)))
			}

			sample.Label[sourceEventNameLabel] = []string{scope.Name()}
			// source.event.time MUST be the unix time in milliseconds; the backend rejects other units.
			sample.NumLabel[sourceEventTimeLabel] = []int64{eventTime.UnixMilli()}
			if sampleType == "cpu" {
				sample.Label[sourceEventPeriodLabel] = []string{strconv.Itoa(int(prof.Period()))}
			}

			if li := src.LinkIndex(); li > 0 && int(li) < dict.LinkTable().Len() {
				link := dict.LinkTable().At(int(li))
				sample.Label[spanIDFieldKey] = []string{link.SpanID().String()}
				sample.Label[traceIDFieldKey] = []string{link.TraceID().String()}
			}

			emittedIdx++
		}
	}
}
