// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package internal

import (
	"context"
	"encoding/hex"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/scrape"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/obsreport"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

var (
	target = scrape.NewTarget(
		// processedLabels contain label values after processing (e.g. relabeling)
		labels.FromMap(map[string]string{
			model.InstanceLabel: "localhost:8080",
		}),
		// discoveredLabels contain labels prior to any processing
		labels.FromMap(map[string]string{
			model.AddressLabel:    "address:8080",
			model.MetricNameLabel: "foo",
			model.SchemeLabel:     "http",
		}),
		nil)

	scrapeCtx = scrape.ContextWithMetricMetadataStore(
		scrape.ContextWithTarget(context.Background(), target),
		testMetadataStore(map[string]scrape.MetricMetadata{}))
)

func TestTransactionCommitWithoutAdding(t *testing.T) {
	nomc := consumertest.NewNop()
	tr := newTransaction(scrapeCtx, nil, true, nil, nomc, nil, componenttest.NewNopReceiverCreateSettings(), nopObsRecv())
	assert.NoError(t, tr.Commit())
}

func TestTransactionRollbackDoesNothing(t *testing.T) {
	nomc := consumertest.NewNop()
	tr := newTransaction(scrapeCtx, nil, true, nil, nomc, nil, componenttest.NewNopReceiverCreateSettings(), nopObsRecv())
	assert.NoError(t, tr.Rollback())
}

func TestTransactionUpdateMetadataDoesNothing(t *testing.T) {
	nomc := consumertest.NewNop()
	tr := newTransaction(scrapeCtx, nil, true, nil, nomc, nil, componenttest.NewNopReceiverCreateSettings(), nopObsRecv())
	_, err := tr.UpdateMetadata(0, labels.New(), metadata.Metadata{})
	assert.NoError(t, err)
}

func TestTransactionAppendNoTarget(t *testing.T) {
	badLabels := labels.FromStrings(model.MetricNameLabel, "foo")
	nomc := consumertest.NewNop()
	tr := newTransaction(scrapeCtx, nil, true, nil, nomc, nil, componenttest.NewNopReceiverCreateSettings(), nopObsRecv())
	_, err := tr.Append(0, badLabels, time.Now().Unix()*1000, 1.0)
	assert.Error(t, err)
}

func TestTransactionAppendNoMetricName(t *testing.T) {
	jobNotFoundLb := labels.FromMap(map[string]string{
		model.InstanceLabel: "localhost:8080",
		model.JobLabel:      "test2",
	})
	nomc := consumertest.NewNop()
	tr := newTransaction(scrapeCtx, nil, true, nil, nomc, nil, componenttest.NewNopReceiverCreateSettings(), nopObsRecv())
	_, err := tr.Append(0, jobNotFoundLb, time.Now().Unix()*1000, 1.0)
	assert.Error(t, err)
}

func TestTransactionAppend(t *testing.T) {
	goodLabels := labels.FromMap(map[string]string{
		model.InstanceLabel:   "localhost:8080",
		model.JobLabel:        "test",
		model.MetricNameLabel: "foo",
	})
	sink := new(consumertest.MetricsSink)
	tr := newTransaction(scrapeCtx, nil, true, nil, sink, nil, componenttest.NewNopReceiverCreateSettings(), nopObsRecv())
	_, err := tr.Append(0, goodLabels, time.Now().Unix()*1000, 1.0)
	assert.NoError(t, err)
	tr.metricBuilder.startTime = 1.0 // set to a non-zero value
	assert.NoError(t, tr.Commit())
	expectedResource := CreateResource("test", "localhost:8080", labels.FromStrings(model.SchemeLabel, "http"))
	mds := sink.AllMetrics()
	require.Len(t, mds, 1)
	gotResource := mds[0].ResourceMetrics().At(0).Resource()
	require.Equal(t, expectedResource, gotResource)
}

func TestTransactionCommitErrorWhenNoStarTime(t *testing.T) {
	goodLabels := labels.FromMap(map[string]string{
		model.InstanceLabel:   "localhost:8080",
		model.JobLabel:        "test",
		model.MetricNameLabel: "foo",
	})
	sink := new(consumertest.MetricsSink)
	tr := newTransaction(scrapeCtx, nil, true, nil, sink, nil, componenttest.NewNopReceiverCreateSettings(), nopObsRecv())
	_, err := tr.Append(0, goodLabels, time.Now().Unix()*1000, 1.0)
	assert.NoError(t, err)
	tr.metricBuilder.startTime = 0 // zero value means the start time metric is missing
	assert.ErrorIs(t, tr.Commit(), errNoStartTimeMetrics)
}

func TestTransactionAppendExemplar(t *testing.T) {
	var ed exemplar.Exemplar
	ed.Ts = 1660233371385
	ed.Value = 0.012
	ed.Labels = labels.FromMap(map[string]string{
		model.InstanceLabel: "localhost:8080",
		model.JobLabel:      "test",
		"trace_id":          "0000a5bc3defg93h",
		"span_id":           "0000a5bc3defg93h",
	})
	lbls := labels.FromMap(map[string]string{
		model.InstanceLabel:   "localhost:8080",
		model.JobLabel:        "test",
		model.MetricNameLabel: "foo_request_duration"})

	sink := new(consumertest.MetricsSink)
	tr := newTransaction(scrapeCtx, nil, true, nil, sink, nil, componenttest.NewNopReceiverCreateSettings(), nopObsRecv())

	_, err := tr.Append(0, lbls, ed.Ts, 0.012)
	assert.NoError(t, err)
	_, err = tr.AppendExemplar(0, lbls, ed)
	assert.NoError(t, err)
	fn := normalizeMetricName(lbls.Get(model.MetricNameLabel))
	gk := tr.metricBuilder.families[fn].getGroupKey(lbls)
	expected := tr.metricBuilder.families[fn].groups[gk].exemplars[ed.Value]
	assert.Equal(t, expected.Timestamp(), pcommon.NewTimestampFromTime(time.UnixMilli(ed.Ts)))
	assert.Equal(t, expected.DoubleVal(), ed.Value)
	assert.Equal(t, expected.TraceID().HexString(), getTraceID("0000a5bc3defg93h"))
	assert.Equal(t, expected.SpanID().HexString(), getSpanID("0000a5bc3defg93h"))
}

func getTraceID(trace string) string {
	var tid [16]byte
	tr, _ := hex.DecodeString(trace)
	copyToLowerBytes(tid[:], tr)
	return pcommon.TraceID(tid).HexString()
}

func getSpanID(span string) string {
	var sid [8]byte
	sp, _ := hex.DecodeString(span)
	copyToLowerBytes(sid[:], sp)
	return pcommon.SpanID(sid).HexString()
}

func nopObsRecv() *obsreport.Receiver {
	return obsreport.NewReceiver(obsreport.ReceiverSettings{
		ReceiverID:             config.NewComponentID("prometheus"),
		Transport:              transport,
		ReceiverCreateSettings: componenttest.NewNopReceiverCreateSettings(),
	})
}
