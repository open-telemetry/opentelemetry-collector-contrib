// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/klauspost/compress/gzip"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type itemRequest struct {
	Action   json.RawMessage
	Document json.RawMessage
}

func itemRequestsSortFunc(a, b itemRequest) int {
	comp := bytes.Compare(a.Action, b.Action)
	if comp == 0 {
		return bytes.Compare(a.Document, b.Document)
	}
	return comp
}

func assertRecordedItems(t *testing.T, expected []itemRequest, recorder *bulkRecorder, assertOrder bool) { //nolint:unparam
	recorder.WaitItems(len(expected))
	assertItemRequests(t, expected, recorder.Items(), assertOrder)
}

func assertItemRequests(t *testing.T, expected, actual []itemRequest, assertOrder bool) {
	expectedItems := expected
	actualItems := actual
	if !assertOrder {
		// Make copies to avoid mutating the args
		expectedItems = make([]itemRequest, len(expected))
		copy(expectedItems, expected)
		slices.SortFunc(expectedItems, itemRequestsSortFunc)
		actualItems = make([]itemRequest, len(actual))
		copy(actualItems, actual)
		slices.SortFunc(actualItems, itemRequestsSortFunc)
	}

	require.Len(t, actualItems, len(expectedItems), "want %d items, got %d", len(expectedItems), len(actualItems))
	for i, want := range expectedItems {
		got := actualItems[i]
		assert.JSONEq(t, string(want.Action), string(got.Action), "item %d action", i)
		assert.JSONEq(t, string(want.Document), string(got.Document), "item %d document", i)
	}
}

type itemResponse struct {
	Status int `json:"status"`
}

type bulkResult struct {
	Took      int            `json:"took"`
	HasErrors bool           `json:"errors"`
	Items     []itemResponse `json:"items"`
}

type bulkHandler func([]itemRequest) ([]itemResponse, error)

type httpTestError struct {
	status  int
	message string
	cause   error
}

const currentESVersion = "7.17.7"

func (e *httpTestError) Error() string {
	return fmt.Sprintf("http request failed (status=%v): %v", e.Status(), e.Message())
}

func (e *httpTestError) Status() int {
	if e.status == 0 {
		return http.StatusInternalServerError
	}
	return e.status
}

func (e *httpTestError) Message() string {
	var buf strings.Builder
	if e.message != "" {
		buf.WriteString(e.message)
	}
	if e.cause != nil {
		if buf.Len() > 0 {
			buf.WriteString(": ")
		}
		buf.WriteString(e.cause.Error())
	}
	return buf.String()
}

type bulkRecorder struct {
	mu         sync.Mutex
	cond       *sync.Cond
	recordings [][]itemRequest
}

func newBulkRecorder() *bulkRecorder {
	r := &bulkRecorder{}
	r.cond = sync.NewCond(&r.mu)
	return r
}

func (r *bulkRecorder) Record(bulk []itemRequest) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.recordings = append(r.recordings, bulk)
	r.cond.Broadcast()
}

func (r *bulkRecorder) WaitItems(n int) []itemRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	for n > r.countItems() {
		r.cond.Wait()
	}
	return r.items()
}

func (r *bulkRecorder) Requests() [][]itemRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.recordings
}

func (r *bulkRecorder) Items() (docs []itemRequest) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.items()
}

func (r *bulkRecorder) items() (docs []itemRequest) {
	for _, rec := range r.recordings {
		docs = append(docs, rec...)
	}
	return docs
}

func (r *bulkRecorder) NumItems() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.countItems()
}

func (r *bulkRecorder) countItems() (count int) {
	for _, docs := range r.recordings {
		count += len(docs)
	}
	return count
}

func newESTestServer(t *testing.T, bulkHandler bulkHandler) *httptest.Server {
	return newESTestServerBulkHandlerFunc(t, handleErr(func(w http.ResponseWriter, req *http.Request) error {
		tsStart := time.Now()
		var items []itemRequest

		body := req.Body
		if req.Header.Get("Content-Encoding") == "gzip" {
			body, _ = gzip.NewReader(req.Body)
		}
		dec := json.NewDecoder(body)
		for dec.More() {
			var action, doc json.RawMessage
			if err := dec.Decode(&action); err != nil {
				return &httpTestError{status: http.StatusBadRequest, cause: err}
			}
			if !dec.More() {
				return &httpTestError{status: http.StatusBadRequest, message: "action without document"}
			}
			if err := dec.Decode(&doc); err != nil {
				return &httpTestError{status: http.StatusBadRequest, cause: err}
			}

			items = append(items, itemRequest{Action: action, Document: doc})
		}

		resp, err := bulkHandler(items)
		if err != nil {
			return err
		}
		took := int(time.Since(tsStart) / time.Microsecond)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		enc := json.NewEncoder(w)
		return enc.Encode(bulkResult{Took: took, Items: resp, HasErrors: itemsHasError(resp)})
	}))
}

func newESTestServerBulkHandlerFunc(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleErr(func(w http.ResponseWriter, _ *http.Request) error {
		w.Header().Add("X-Elastic-Product", "Elasticsearch")

		enc := json.NewEncoder(w)
		return enc.Encode(map[string]any{
			"version": map[string]any{
				"number": currentESVersion,
			},
		})
	}))
	mux.HandleFunc("/_bulk", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("X-Elastic-Product", "Elasticsearch")
		handler.ServeHTTP(w, r)
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func handleErr(fn func(http.ResponseWriter, *http.Request) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := fn(w, r)
		if err != nil {
			httpError := &httpTestError{}
			if errors.As(err, &httpError) {
				http.Error(w, httpError.Message(), httpError.Status())
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}
	}
}

func (item *itemResponse) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, `{"create": {"status": %v}}`, item.Status)
	return buf.Bytes(), nil
}

func itemsAllOK(docs []itemRequest) ([]itemResponse, error) {
	return itemsReportStatus(docs, http.StatusOK)
}

func itemsReportStatus(docs []itemRequest, status int) ([]itemResponse, error) {
	responses := make([]itemResponse, len(docs))
	for i := range docs {
		responses[i].Status = status
	}
	return responses, nil
}

func itemsHasError(resp []itemResponse) bool {
	for _, r := range resp {
		if r.Status != http.StatusOK {
			return true
		}
	}
	return false
}

func newLogsWithAttributes(recordAttrs, scopeAttrs, resourceAttrs map[string]any) plog.Logs {
	logs := plog.NewLogs()
	resourceLog := logs.ResourceLogs().AppendEmpty()
	scopeLog := resourceLog.ScopeLogs().AppendEmpty()
	fillAttributeMap(resourceLog.Resource().Attributes(), resourceAttrs)
	fillAttributeMap(scopeLog.Scope().Attributes(), scopeAttrs)
	fillAttributeMap(scopeLog.LogRecords().AppendEmpty().Attributes(), recordAttrs)

	return logs
}

func newMetricsWithAttributes(recordAttrs, scopeAttrs, resourceAttrs map[string]any) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	resourceMetric := metrics.ResourceMetrics().AppendEmpty()
	scopeMetric := resourceMetric.ScopeMetrics().AppendEmpty()

	fillAttributeMap(resourceMetric.Resource().Attributes(), resourceAttrs)
	fillAttributeMap(scopeMetric.Scope().Attributes(), scopeAttrs)
	dp := scopeMetric.Metrics().AppendEmpty().SetEmptySum().DataPoints().AppendEmpty()
	dp.SetIntValue(0)
	fillAttributeMap(dp.Attributes(), recordAttrs)

	return metrics
}

func newTracesWithAttributes(recordAttrs, scopeAttrs, resourceAttrs map[string]any) ptrace.Traces {
	traces := ptrace.NewTraces()
	resourceSpan := traces.ResourceSpans().AppendEmpty()
	scopeSpan := resourceSpan.ScopeSpans().AppendEmpty()

	fillAttributeMap(resourceSpan.Resource().Attributes(), resourceAttrs)
	fillAttributeMap(scopeSpan.Scope().Attributes(), scopeAttrs)
	fillAttributeMap(scopeSpan.Spans().AppendEmpty().Attributes(), recordAttrs)

	return traces
}

func fillAttributeMap(attrs pcommon.Map, m map[string]any) {
	attrs.EnsureCapacity(len(m))
	for k, v := range m {
		switch vv := v.(type) {
		case bool:
			attrs.PutBool(k, vv)
		case string:
			attrs.PutStr(k, vv)
		case []string:
			slice := attrs.PutEmptySlice(k)
			slice.EnsureCapacity(len(vv))
			for _, s := range vv {
				slice.AppendEmpty().SetStr(s)
			}
		}
	}
}

func TestGetSuffixTime(t *testing.T) {
	defaultCfg := createDefaultConfig().(*Config)
	defaultCfg.LogstashFormat.Enabled = true
	defaultCfg.LogsIndex = "logs-generic-default"
	testTime := time.Date(2023, 12, 2, 10, 10, 10, 1, time.UTC)
	index, err := generateIndexWithLogstashFormat(defaultCfg.LogsIndex, &defaultCfg.LogstashFormat, testTime)
	assert.NoError(t, err)
	assert.Equal(t, "logs-generic-default-2023.12.02", index)

	defaultCfg.LogsIndex = "logstash"
	defaultCfg.LogstashFormat.PrefixSeparator = "."
	otelLogsIndex, err := generateIndexWithLogstashFormat(defaultCfg.LogsIndex, &defaultCfg.LogstashFormat, testTime)
	assert.NoError(t, err)
	assert.Equal(t, "logstash.2023.12.02", otelLogsIndex)

	defaultCfg.LogstashFormat.DateFormat = "%Y-%m-%d"
	newOtelLogsIndex, err := generateIndexWithLogstashFormat(defaultCfg.LogsIndex, &defaultCfg.LogstashFormat, testTime)
	assert.NoError(t, err)
	assert.Equal(t, "logstash.2023-12-02", newOtelLogsIndex)

	defaultCfg.LogstashFormat.DateFormat = "%d/%m/%Y"
	newOtelLogsIndexWithSpecDataFormat, err := generateIndexWithLogstashFormat(defaultCfg.LogsIndex, &defaultCfg.LogstashFormat, testTime)
	assert.NoError(t, err)
	assert.Equal(t, "logstash.02/12/2023", newOtelLogsIndexWithSpecDataFormat)
}
