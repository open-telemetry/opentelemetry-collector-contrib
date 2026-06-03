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

// AddProfilingPprofSampleLabels adds Splunk AlwaysOn Profiling labels to pprof samples.
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

		sample.Label["source.event.name"] = []string{scope.Name()}
		sample.NumLabel["source.event.time"] = []int64{profileTime.UnixMilli()}
		if sampleType == "cpu" {
			sample.Label["source.event.period"] = []string{strconv.Itoa(int(prof.Period()))}
		}

		if sampleIdx >= prof.Samples().Len() {
			continue
		}
		if li := prof.Samples().At(sampleIdx).LinkIndex(); li > 0 && int(li) < dict.LinkTable().Len() {
			link := dict.LinkTable().At(int(li))
			sample.Label["span_id"] = []string{link.SpanID().String()}
			sample.Label["trace_id"] = []string{link.TraceID().String()}
		}
	}
}
