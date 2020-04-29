// Copyright 2019 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sourceprocessor

import (
	"context"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector/config/configmodels"
	"github.com/open-telemetry/opentelemetry-collector/consumer/pdata"
	"github.com/stretchr/testify/assert"
)

var (
	cfg = &Config{
		ProcessorSettings: configmodels.ProcessorSettings{
			TypeVal: "resource",
			NameVal: "resource",
		},
		SourceName:                "%{namespace}.%{pod}.%{container}",
		SourceCategory:            "%{namespace}/%{pod_name}",
		SourceCategoryPrefix:      "prefix/",
		SourceCategoryReplaceDash: "#",
	}

	resourceLabels = map[string]string{
		"namespace": "namespace-1",
		"pod_id":    "pod-1234",
		"pod":       "pod-1",
		"pod_name":  "some-pod",
		"container": "container-1",
	}

	mergedResourceLabels = map[string]string{
		"namespace":       "namespace-1",
		"pod_id":          "pod-1234",
		"pod":             "pod-1",
		"container":       "container-1",
		"pod_name":        "some-pod",
		"_sourceName":     "namespace-1.pod-1.container-1",
		"_sourceCategory": "prefix/namespace#1/some#pod",
	}
)

func newTraceData(labels map[string]string) pdata.Traces {
	td := pdata.NewTraces()
	td.ResourceSpans().Resize(1)
	rs := td.ResourceSpans().At(0)
	rs.Resource().InitEmpty()
	attrs := rs.Resource().Attributes()
	for k, v := range labels {
		attrs.UpsertString(k, v)
	}
	return td
}

func prepareAttributes(t pdata.Traces) {
	for i := 0; i < t.ResourceSpans().Len(); i++ {
		t.ResourceSpans().At(i).Resource().Attributes().Sort()
	}
}

func assertAttributesEqual(t *testing.T, t1 pdata.Traces, t2 pdata.Traces) {
	prepareAttributes(t1)
	prepareAttributes(t2)
	assert.Equal(t, t1, t2)
}

//func TestTraceSourceProcessor(t *testing.T) {
//	want := newTraceData(mergedResourceLabels)
//	test := newTraceData(resourceLabels)
//
//	ttn := &testTraceConsumer{}
//	rtp, _ := newSourceTraceProcessor(ttn, cfg)
//	assert.True(t, rtp.GetCapabilities().MutatesConsumedData)
//
//	rtp.ConsumeTraces(context.Background(), test)
//
//	assertAttributesEqual(t, ttn.td, want)
//}
//
//func TestTraceSourceProcessorEmpty(t *testing.T) {
//	want := newTraceData(resourceLabels2)
//	test := newTraceData(resourceLabels2)
//
//	ttn := &testTraceConsumer{}
//	rtp, _ := newSourceTraceProcessor(ttn, cfg)
//
//	rtp.ConsumeTraces(context.Background(), test)
//	assertAttributesEqual(t, ttn.td, want)
//}

func TestTraceSourceProcessorAnnotations(t *testing.T) {
	resourceLabels["sumologic.com/sourceHost"] = "sh:%{pod_id}"
	resourceLabels["sumologic.com/sourceCategory"] = "sc:%{pod_id}"
	test := newTraceData(resourceLabels)

	mergedResourceLabels["sumologic.com/sourceHost"] = "sh:%{pod_id}"
	mergedResourceLabels["sumologic.com/sourceCategory"] = "sc:%{pod_id}"
	mergedResourceLabels["_sourceHost"] = "sh:pod-1234"
	mergedResourceLabels["_sourceCategory"] = "prefix/sc:pod#1234"
	want := newTraceData(mergedResourceLabels)

	ttn := &testTraceConsumer{}
	rtp, _ := newSourceTraceProcessor(ttn, cfg)
	assert.True(t, rtp.GetCapabilities().MutatesConsumedData)

	rtp.ConsumeTraces(context.Background(), test)

	assertAttributesEqual(t, ttn.td, want)
}

type testTraceConsumer struct {
	td pdata.Traces
}

func (ttn *testTraceConsumer) ConsumeTraces(ctx context.Context, td pdata.Traces) error {
	ttn.td = td
	return nil
}
