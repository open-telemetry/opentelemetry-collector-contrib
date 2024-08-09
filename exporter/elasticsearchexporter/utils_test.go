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

	"github.com/stretchr/testify/assert"
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

func assertItemsEqual(t *testing.T, expected, actual []itemRequest, assertOrder bool) { // nolint:unparam
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
	assert.Equal(t, expectedItems, actualItems)
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

func (r *bulkRecorder) WaitItems(n int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for n > r.countItems() {
		r.cond.Wait()
	}
}

func (r *bulkRecorder) Requests() [][]itemRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.recordings
}

func (r *bulkRecorder) Items() (docs []itemRequest) {
	for _, rec := range r.Requests() {
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

		dec := json.NewDecoder(req.Body)
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

func newLogsWithAttributeAndResourceMap(attrMp map[string]string, resMp map[string]string) plog.Logs {
	logs := plog.NewLogs()
	resourceSpans := logs.ResourceLogs()
	rs := resourceSpans.AppendEmpty()

	scopeAttr := rs.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty().Attributes()
	fillResourceAttributeMap(scopeAttr, attrMp)

	resAttr := rs.Resource().Attributes()
	fillResourceAttributeMap(resAttr, resMp)

	return logs
}

func newMetricsWithAttributeAndResourceMap(attrMp map[string]string, resMp map[string]string) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics().AppendEmpty()

	fillResourceAttributeMap(resourceMetrics.Resource().Attributes(), resMp)
	dp := resourceMetrics.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetEmptySum().DataPoints().AppendEmpty()
	dp.SetIntValue(0)
	fillResourceAttributeMap(dp.Attributes(), attrMp)

	return metrics
}

func newTracesWithAttributeAndResourceMap(attrMp map[string]string, resMp map[string]string) ptrace.Traces {
	traces := ptrace.NewTraces()
	resourceSpans := traces.ResourceSpans()
	rs := resourceSpans.AppendEmpty()

	scopeAttr := rs.ScopeSpans().AppendEmpty().Spans().AppendEmpty().Attributes()
	fillResourceAttributeMap(scopeAttr, attrMp)

	resAttr := rs.Resource().Attributes()
	fillResourceAttributeMap(resAttr, resMp)

	return traces
}

func fillResourceAttributeMap(attrs pcommon.Map, mp map[string]string) {
	attrs.EnsureCapacity(len(mp))
	for k, v := range mp {
		attrs.PutStr(k, v)
	}
}

func TestGetSuffixTime(t *testing.T) {
	defaultCfg := createDefaultConfig().(*Config)
	defaultCfg.LogstashFormat.Enabled = true
	testTime := time.Date(2023, 12, 2, 10, 10, 10, 1, time.UTC)
	index, err := generateIndexWithLogstashFormat(defaultCfg.LogsIndex, &defaultCfg.LogstashFormat, testTime)
	assert.NoError(t, err)
	assert.Equal(t, index, "logs-generic-default-2023.12.02")

	defaultCfg.LogsIndex = "logstash"
	defaultCfg.LogstashFormat.PrefixSeparator = "."
	otelLogsIndex, err := generateIndexWithLogstashFormat(defaultCfg.LogsIndex, &defaultCfg.LogstashFormat, testTime)
	assert.NoError(t, err)
	assert.Equal(t, otelLogsIndex, "logstash.2023.12.02")

	defaultCfg.LogstashFormat.DateFormat = "%Y-%m-%d"
	newOtelLogsIndex, err := generateIndexWithLogstashFormat(defaultCfg.LogsIndex, &defaultCfg.LogstashFormat, testTime)
	assert.NoError(t, err)
	assert.Equal(t, newOtelLogsIndex, "logstash.2023-12-02")

	defaultCfg.LogstashFormat.DateFormat = "%d/%m/%Y"
	newOtelLogsIndexWithSpecDataFormat, err := generateIndexWithLogstashFormat(defaultCfg.LogsIndex, &defaultCfg.LogstashFormat, testTime)
	assert.NoError(t, err)
	assert.Equal(t, newOtelLogsIndexWithSpecDataFormat, "logstash.02/12/2023")
}
