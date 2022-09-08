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
			model.AddressLabel: "address:8080",
			model.SchemeLabel:  "http",
		}),
		nil)

	scrapeCtx = scrape.ContextWithMetricMetadataStore(
		scrape.ContextWithTarget(context.Background(), target),
		testMetadataStore(testMetadata))
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
	badLabels := labels.FromStrings(model.MetricNameLabel, "counter_test")
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
	assert.ErrorIs(t, err, errMetricNameNotFound)

	assert.ErrorIs(t, tr.Commit(), errNoDataToBuild)
}

func TestTransactionAppendEmptyMetricName(t *testing.T) {
	nomc := consumertest.NewNop()
	tr := newTransaction(scrapeCtx, nil, true, nil, nomc, nil, componenttest.NewNopReceiverCreateSettings(), nopObsRecv())
	_, err := tr.Append(0, labels.FromMap(map[string]string{
		model.InstanceLabel:   "localhost:8080",
		model.JobLabel:        "test2",
		model.MetricNameLabel: "",
	}), time.Now().Unix()*1000, 1.0)
	assert.ErrorIs(t, err, errMetricNameNotFound)
}

func TestTransactionAppend(t *testing.T) {
	sink := new(consumertest.MetricsSink)
	tr := newTransaction(scrapeCtx, nil, true, nil, sink, nil, componenttest.NewNopReceiverCreateSettings(), nopObsRecv())
	_, err := tr.Append(0, labels.FromMap(map[string]string{
		model.InstanceLabel:   "localhost:8080",
		model.JobLabel:        "test",
		model.MetricNameLabel: "counter_test",
	}), time.Now().Unix()*1000, 1.0)
	assert.NoError(t, err)
	_, err = tr.Append(0, labels.FromMap(map[string]string{
		model.InstanceLabel:   "localhost:8080",
		model.JobLabel:        "test",
		model.MetricNameLabel: startTimeMetricName,
	}), time.Now().UnixMilli(), 1.0)
	assert.NoError(t, err)
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
		model.MetricNameLabel: "counter_test",
	})
	sink := new(consumertest.MetricsSink)
	tr := newTransaction(scrapeCtx, nil, true, nil, sink, nil, componenttest.NewNopReceiverCreateSettings(), nopObsRecv())
	_, err := tr.Append(0, goodLabels, time.Now().Unix()*1000, 1.0)
	assert.NoError(t, err)
	assert.ErrorIs(t, tr.Commit(), errNoStartTimeMetrics)
}

func TestTransactionAppendExemplar(t *testing.T) {
	ed := exemplar.Exemplar{
		Labels: labels.FromMap(map[string]string{
			"foo":      "bar",
			traceIDKey: "1234567890abcdeffedcba0987654321",
			spanIDKey:  "8765432112345678",
		}),
		Ts:    1660233371385,
		HasTs: true,
		Value: 0.012,
	}
	lbls := labels.FromMap(map[string]string{
		model.InstanceLabel:   "localhost:8080",
		model.JobLabel:        "test",
		model.MetricNameLabel: "counter_test"})

	sink := new(consumertest.MetricsSink)
	tr := newTransaction(scrapeCtx, nil, true, nil, sink, nil, componenttest.NewNopReceiverCreateSettings(), nopObsRecv())

	_, err := tr.Append(0, lbls, ed.Ts, 0.012)
	assert.NoError(t, err)
	_, err = tr.AppendExemplar(0, lbls, ed)
	assert.NoError(t, err)
	fn := normalizeMetricName(lbls.Get(model.MetricNameLabel))
	gk := tr.metricBuilder.families[fn].getGroupKey(lbls)
	got := tr.metricBuilder.families[fn].groups[gk].exemplars[ed.Value]
	assert.Equal(t, pcommon.NewTimestampFromTime(time.UnixMilli(ed.Ts)), got.Timestamp())
	assert.Equal(t, ed.Value, got.DoubleVal())
	assert.Equal(t, "1234567890abcdeffedcba0987654321", got.TraceID().HexString())
	assert.Equal(t, "8765432112345678", got.SpanID().HexString())
	assert.Equal(t, pcommon.NewMapFromRaw(map[string]interface{}{"foo": "bar"}), got.FilteredAttributes())
}

// Ensure that we reject duplicate label keys. See https://github.com/open-telemetry/wg-prometheus/issues/44.
func TestTransactionAppendDuplicateLabels(t *testing.T) {
	sink := new(consumertest.MetricsSink)
	tr := newTransaction(scrapeCtx, nil, true, nil, sink, nil, componenttest.NewNopReceiverCreateSettings(), nopObsRecv())

	dupLabels := labels.FromStrings(
		model.InstanceLabel, "0.0.0.0:8855",
		model.JobLabel, "test",
		model.MetricNameLabel, "counter_test",
		"a", "1",
		"a", "1",
		"z", "9",
		"z", "1",
	)

	_, err := tr.Append(0, dupLabels, 1917, 1.0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `invalid sample: non-unique label names: ["a" "z"]`)
}

func TestTransactionAppendHistogramNoLe(t *testing.T) {
	sink := new(consumertest.MetricsSink)
	tr := newTransaction(scrapeCtx, nil, true, nil, sink, nil, componenttest.NewNopReceiverCreateSettings(), nopObsRecv())

	goodLabels := labels.FromStrings(
		model.InstanceLabel, "0.0.0.0:8855",
		model.JobLabel, "test",
		model.MetricNameLabel, "hist_test_bucket",
	)

	_, err := tr.Append(0, goodLabels, 1917, 1.0)
	require.ErrorIs(t, err, errEmptyLeLabel)
}

func TestTransactionAppendSummaryNoQuantile(t *testing.T) {
	sink := new(consumertest.MetricsSink)
	tr := newTransaction(scrapeCtx, nil, true, nil, sink, nil, componenttest.NewNopReceiverCreateSettings(), nopObsRecv())

	goodLabels := labels.FromStrings(
		model.InstanceLabel, "0.0.0.0:8855",
		model.JobLabel, "test",
		model.MetricNameLabel, "summary_test",
	)

	_, err := tr.Append(0, goodLabels, 1917, 1.0)
	require.ErrorIs(t, err, errEmptyQuantileLabel)
}

func nopObsRecv() *obsreport.Receiver {
	return obsreport.NewReceiver(obsreport.ReceiverSettings{
		ReceiverID:             config.NewComponentID("prometheus"),
		Transport:              transport,
		ReceiverCreateSettings: componenttest.NewNopReceiverCreateSettings(),
	})
}
