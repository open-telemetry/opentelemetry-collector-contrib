// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package splunkhecexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter"

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync"

	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

// allow monkey patching for injecting pushLogData function in test
var getPushLogFn = func(c *client) func(ctx context.Context, ld plog.Logs) error {
	return c.pushLogData
}

// client sends the data to the splunk backend.
type client struct {
	config            *Config
	logger            *zap.Logger
	wg                sync.WaitGroup
	telemetrySettings component.TelemetrySettings
	hecWorker         hecWorker
	buildInfo         component.BuildInfo
	heartbeater       *heartbeater
}

func (c *client) pushMetricsData(
	ctx context.Context,
	md pmetric.Metrics,
) error {
	c.wg.Add(1)
	defer c.wg.Done()

	localHeaders := map[string]string{}
	if md.ResourceMetrics().Len() != 0 {
		accessToken, found := md.ResourceMetrics().At(0).Resource().Attributes().Get(splunk.HecTokenLabel)
		if found {
			localHeaders["Authorization"] = splunk.HECTokenHeader + " " + accessToken.Str()
		}
	}

	return c.pushMetricsDataInBatches(ctx, md, localHeaders)
}

func (c *client) pushTraceData(
	ctx context.Context,
	td ptrace.Traces,
) error {
	c.wg.Add(1)
	defer c.wg.Done()

	localHeaders := map[string]string{}
	if td.ResourceSpans().Len() != 0 {
		accessToken, found := td.ResourceSpans().At(0).Resource().Attributes().Get(splunk.HecTokenLabel)
		if found {
			localHeaders["Authorization"] = splunk.HECTokenHeader + " " + accessToken.Str()
		}
	}

	return c.pushTracesDataInBatches(ctx, td, localHeaders)
}

func (c *client) pushLogData(ctx context.Context, ld plog.Logs) error {
	c.wg.Add(1)
	defer c.wg.Done()

	localHeaders := map[string]string{}
	if ld.ResourceLogs().Len() != 0 {
		accessToken, found := ld.ResourceLogs().At(0).Resource().Attributes().Get(splunk.HecTokenLabel)
		if found {
			localHeaders["Authorization"] = splunk.HECTokenHeader + " " + accessToken.Str()
		}
	}

	return c.pushLogDataInBatches(ctx, ld, localHeaders)
}

// A guesstimated value > length of bytes of a single event.
// Added to buffer capacity so that buffer is likely to grow by reslicing when buf.Len() > bufCap.
const bufCapPadding = uint(4096)
const libraryHeaderName = "X-Splunk-Instrumentation-Library"
const profilingLibraryName = "otel.profiling"

var profilingHeaders = map[string]string{
	libraryHeaderName: profilingLibraryName,
}

func isProfilingData(sl plog.ScopeLogs) bool {
	return sl.Scope().Name() == profilingLibraryName
}

// pushLogDataInBatches sends batches of Splunk events in JSON format.
// The batch content length is restricted to MaxContentLengthLogs.
// ld log records are parsed to Splunk events.
// The input data may contain both logs and profiling data.
// They are batched separately and sent with different HTTP headers
func (c *client) pushLogDataInBatches(ctx context.Context, ld plog.Logs, headers map[string]string) error {
	profilingLocalHeaders := map[string]string{}
	for k, v := range profilingHeaders {
		profilingLocalHeaders[k] = v
	}

	for k, v := range headers {
		profilingLocalHeaders[k] = v
	}

	var bufState = makeBlankBufferState(c.config.MaxContentLengthLogs, !c.config.DisableCompression, c.config.MaxEventSize)
	var profilingBufState = makeBlankBufferState(c.config.MaxContentLengthLogs, !c.config.DisableCompression, c.config.MaxEventSize)
	var permanentErrors []error

	var rls = ld.ResourceLogs()
	var droppedProfilingDataRecords, droppedLogRecords int
	for i := 0; i < rls.Len(); i++ {
		ills := rls.At(i).ScopeLogs()
		for j := 0; j < ills.Len(); j++ {
			var err error
			var newPermanentErrors []error

			if isProfilingData(ills.At(j)) {
				if !c.config.ProfilingDataEnabled {
					droppedProfilingDataRecords += ills.At(j).LogRecords().Len()
					continue
				}
				profilingBufState.resource, profilingBufState.library = i, j
				newPermanentErrors, err = c.pushLogRecords(ctx, rls, profilingBufState, profilingLocalHeaders)
			} else {
				if !c.config.LogDataEnabled {
					droppedLogRecords += ills.At(j).LogRecords().Len()
					continue
				}
				bufState.resource, bufState.library = i, j
				newPermanentErrors, err = c.pushLogRecords(ctx, rls, bufState, headers)
			}

			if err != nil {
				return consumererror.NewLogs(err, c.subLogs(ld, bufState.bufFront, profilingBufState.bufFront))
			}

			permanentErrors = append(permanentErrors, newPermanentErrors...)
		}
	}

	if droppedProfilingDataRecords != 0 {
		c.logger.Debug("Profiling data is not allowed", zap.Int("dropped_records", droppedProfilingDataRecords))
	}
	if droppedLogRecords != 0 {
		c.logger.Debug("Log data is not allowed", zap.Int("dropped_records", droppedLogRecords))
	}

	// There's some leftover unsent non-profiling data
	if bufState.containsData {

		if err := c.postEvents(ctx, bufState, headers); err != nil {
			return consumererror.NewLogs(err, c.subLogs(ld, bufState.bufFront, profilingBufState.bufFront))
		}
	}

	// There's some leftover unsent profiling data
	if profilingBufState.containsData {
		if err := c.postEvents(ctx, profilingBufState, profilingLocalHeaders); err != nil {
			// Non-profiling bufFront is set to nil because all non-profiling data was flushed successfully above.
			return consumererror.NewLogs(err, c.subLogs(ld, nil, profilingBufState.bufFront))
		}
	}

	return multierr.Combine(permanentErrors...)
}

func (c *client) pushLogRecords(ctx context.Context, lds plog.ResourceLogsSlice, state *bufferState, headers map[string]string) (permanentErrors []error, sendingError error) {
	res := lds.At(state.resource)
	logs := res.ScopeLogs().At(state.library).LogRecords()

	for k := 0; k < logs.Len(); k++ {
		if state.bufFront == nil {
			state.bufFront = &index{resource: state.resource, library: state.library, record: k}
		}
		var b []byte

		if c.config.ExportRaw {
			b = []byte(logs.At(k).Body().AsString() + "\n")
		} else {
			// Parsing log record to Splunk event.
			event := mapLogRecordToSplunkEvent(res.Resource(), logs.At(k), c.config)
			// JSON encoding event and writing to buffer.
			var err error
			b, err = jsoniter.Marshal(event)
			if err != nil {
				permanentErrors = append(permanentErrors, consumererror.NewPermanent(fmt.Errorf("dropped log event: %v, error: %w", event, err)))
				continue
			}

		}

		// Continue adding events to buffer up to capacity.
		accept, e := state.accept(b)
		if e != nil {
			permanentErrors = append(permanentErrors, consumererror.NewPermanent(
				fmt.Errorf("error writing the event: %w", e)))
			continue
		}
		if accept {
			continue
		}

		if state.containsData {
			if err := c.postEvents(ctx, state, headers); err != nil {
				return permanentErrors, err
			}
		}
		state.reset()

		// Writing truncated bytes back to buffer.
		accept, e = state.accept(b)
		if e != nil {
			permanentErrors = append(permanentErrors, consumererror.NewPermanent(
				fmt.Errorf("error writing the event: %w", e)))
			continue
		}
		if !accept {
			permanentErrors = append(permanentErrors, consumererror.NewPermanent(
				fmt.Errorf("dropped log event error: event size %d bytes larger than configured max content length %d bytes", len(b), state.bufferMaxLen)))
			continue
		}
		if state.containsData {
			// This means that the current record had overflown the buffer and was not sent
			state.bufFront = &index{resource: state.resource, library: state.library, record: k}
		} else {
			// This means that the entire buffer was sent, including the current record
			state.bufFront = nil
		}

	}

	return permanentErrors, nil
}

func (c *client) pushMetricsRecords(ctx context.Context, mds pmetric.ResourceMetricsSlice, state *bufferState, headers map[string]string) (permanentErrors []error, sendingError error) {
	res := mds.At(state.resource)
	metrics := res.ScopeMetrics().At(state.library).Metrics()

	for k := 0; k < metrics.Len(); k++ {
		if state.bufFront == nil {
			state.bufFront = &index{resource: state.resource, library: state.library, record: k}
		}

		// Parsing metric record to Splunk event.
		events := mapMetricToSplunkEvent(res.Resource(), metrics.At(k), c.config, c.logger)
		buf := bytes.NewBuffer(make([]byte, 0, c.config.MaxContentLengthMetrics))
		if c.config.UseMultiMetricFormat {
			merged, err := mergeEventsToMultiMetricFormat(events)
			if err != nil {
				permanentErrors = append(permanentErrors, consumererror.NewPermanent(fmt.Errorf("error merging events: %w", err)))
			} else {
				events = merged
			}
		}
		for _, event := range events {
			// JSON encoding event and writing to buffer.
			b, err := jsoniter.Marshal(event)
			if err != nil {
				permanentErrors = append(permanentErrors, consumererror.NewPermanent(fmt.Errorf("dropped metric event: %v, error: %w", event, err)))
				continue
			}
			buf.Write(b)
		}

		// Continue adding events to buffer up to capacity.
		b := buf.Bytes()
		accept, e := state.accept(b)
		if e != nil {
			permanentErrors = append(permanentErrors, consumererror.NewPermanent(
				fmt.Errorf("error writing the event: %w", e)))
			continue
		}
		if accept {
			continue
		}

		if state.containsData {
			if err := c.postEvents(ctx, state, headers); err != nil {
				return permanentErrors, err
			}
		}
		state.reset()

		// Writing truncated bytes back to buffer.
		accept, e = state.accept(b)
		if e != nil {
			permanentErrors = append(permanentErrors, consumererror.NewPermanent(
				fmt.Errorf("error writing the event: %w", e)))
			continue
		}
		if !accept {
			permanentErrors = append(permanentErrors, consumererror.NewPermanent(
				fmt.Errorf("dropped metric event: error: event size %d bytes larger than configured max content length %d bytes", len(b), state.bufferMaxLen)))
			continue
		}

		if state.containsData {
			// This means that the current record had overflown the buffer and was not sent
			state.bufFront = &index{resource: state.resource, library: state.library, record: k}
		} else {
			// This means that the entire buffer was sent, including the current record
			state.bufFront = nil
		}

	}

	return permanentErrors, nil
}

func (c *client) pushTracesData(ctx context.Context, tds ptrace.ResourceSpansSlice, state *bufferState, headers map[string]string) (permanentErrors []error, sendingError error) {
	res := tds.At(state.resource)
	spans := res.ScopeSpans().At(state.library).Spans()

	for k := 0; k < spans.Len(); k++ {
		if state.bufFront == nil {
			state.bufFront = &index{resource: state.resource, library: state.library, record: k}
		}

		// Parsing span record to Splunk event.
		event := mapSpanToSplunkEvent(res.Resource(), spans.At(k), c.config)
		// JSON encoding event and writing to buffer.
		b, err := jsoniter.Marshal(event)
		if err != nil {
			permanentErrors = append(permanentErrors, consumererror.NewPermanent(fmt.Errorf("dropped span events: %v, error: %w", event, err)))
			continue
		}

		// Continue adding events to buffer up to capacity.
		accept, e := state.accept(b)
		if e != nil {
			permanentErrors = append(permanentErrors, consumererror.NewPermanent(
				fmt.Errorf("error writing the event: %w", e)))
			continue
		}
		if accept {
			continue
		}

		if state.containsData {
			if err = c.postEvents(ctx, state, headers); err != nil {
				return permanentErrors, err
			}
		}
		state.reset()

		// Writing truncated bytes back to buffer.
		accept, e = state.accept(b)
		if e != nil {
			permanentErrors = append(permanentErrors, consumererror.NewPermanent(
				fmt.Errorf("error writing the event: %w", e)))
			continue
		}
		if !accept {
			permanentErrors = append(permanentErrors, consumererror.NewPermanent(
				fmt.Errorf("dropped trace event error: event size %d bytes larger than configured max content length %d bytes", len(b), state.bufferMaxLen)))
			continue
		}

		if state.containsData {
			// This means that the current record had overflown the buffer and was not sent
			state.bufFront = &index{resource: state.resource, library: state.library, record: k}
		} else {
			// This means that the entire buffer was sent, including the current record
			state.bufFront = nil
		}

	}

	return permanentErrors, nil
}

// pushMetricsDataInBatches sends batches of Splunk events in JSON format.
// The batch content length is restricted to MaxContentLengthMetrics.
// md metrics are parsed to Splunk events.
func (c *client) pushMetricsDataInBatches(ctx context.Context, md pmetric.Metrics, headers map[string]string) error {
	var bufState = makeBlankBufferState(c.config.MaxContentLengthMetrics, !c.config.DisableCompression, c.config.MaxEventSize)
	var permanentErrors []error

	var rms = md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		ilms := rms.At(i).ScopeMetrics()
		for j := 0; j < ilms.Len(); j++ {
			var err error
			var newPermanentErrors []error

			bufState.resource, bufState.library = i, j
			newPermanentErrors, err = c.pushMetricsRecords(ctx, rms, bufState, headers)

			if err != nil {
				return consumererror.NewMetrics(err, subMetrics(md, bufState.bufFront))
			}

			permanentErrors = append(permanentErrors, newPermanentErrors...)
		}
	}

	// There's some leftover unsent metrics
	if bufState.containsData {
		if err := c.postEvents(ctx, bufState, headers); err != nil {
			return consumererror.NewMetrics(err, subMetrics(md, bufState.bufFront))
		}
	}

	return multierr.Combine(permanentErrors...)
}

// pushTracesDataInBatches sends batches of Splunk events in JSON format.
// The batch content length is restricted to MaxContentLengthMetrics.
// td traces are parsed to Splunk events.
func (c *client) pushTracesDataInBatches(ctx context.Context, td ptrace.Traces, headers map[string]string) error {
	bufState := makeBlankBufferState(c.config.MaxContentLengthTraces, !c.config.DisableCompression, c.config.MaxEventSize)
	var permanentErrors []error

	var rts = td.ResourceSpans()
	for i := 0; i < rts.Len(); i++ {
		ilts := rts.At(i).ScopeSpans()
		for j := 0; j < ilts.Len(); j++ {
			var err error
			var newPermanentErrors []error

			bufState.resource, bufState.library = i, j
			newPermanentErrors, err = c.pushTracesData(ctx, rts, bufState, headers)

			if err != nil {
				return consumererror.NewTraces(err, subTraces(td, bufState.bufFront))
			}

			permanentErrors = append(permanentErrors, newPermanentErrors...)
		}
	}

	// There's some leftover unsent traces
	if bufState.containsData {
		if err := c.postEvents(ctx, bufState, headers); err != nil {
			return consumererror.NewTraces(err, subTraces(td, bufState.bufFront))
		}
	}

	return multierr.Combine(permanentErrors...)
}

func (c *client) postEvents(ctx context.Context, bufState *bufferState, headers map[string]string) error {
	if err := bufState.Close(); err != nil {
		return err
	}
	return c.hecWorker.send(ctx, bufState, headers)
}

// subLogs returns a subset of `ld` starting from `profilingBufFront` for profiling data
// plus starting from `bufFront` for non-profiling data. Both can be nil, in which case they are ignored
func (c *client) subLogs(ld plog.Logs, bufFront *index, profilingBufFront *index) plog.Logs {
	subset := plog.NewLogs()
	if c.config.LogDataEnabled {
		subLogsByType(ld, bufFront, subset, false)
	}
	if c.config.ProfilingDataEnabled {
		subLogsByType(ld, profilingBufFront, subset, true)
	}

	return subset
}

// subMetrics returns a subset of `md`starting from `bufFront`. It can be nil, in which case it is ignored
func subMetrics(md pmetric.Metrics, bufFront *index) pmetric.Metrics {
	subset := pmetric.NewMetrics()
	subMetricsByType(md, bufFront, subset)

	return subset
}

// subTraces returns a subset of `td`starting from `bufFront`. It can be nil, in which case it is ignored
func subTraces(td ptrace.Traces, bufFront *index) ptrace.Traces {
	subset := ptrace.NewTraces()
	subTracesByType(td, bufFront, subset)

	return subset
}

func subLogsByType(src plog.Logs, from *index, dst plog.Logs, profiling bool) {
	if from == nil {
		return // All the data of this type was sent successfully
	}

	resources := src.ResourceLogs()
	resourcesSub := dst.ResourceLogs()

	for i := from.resource; i < resources.Len(); i++ {
		newSub := resourcesSub.AppendEmpty()
		resources.At(i).Resource().CopyTo(newSub.Resource())

		libraries := resources.At(i).ScopeLogs()
		librariesSub := newSub.ScopeLogs()

		j := 0
		if i == from.resource {
			j = from.library
		}
		for jSub := 0; j < libraries.Len(); j++ {
			lib := libraries.At(j)

			// Only copy profiling data if requested. If not requested, only copy non-profiling data
			if profiling != isProfilingData(lib) {
				continue
			}

			newLibSub := librariesSub.AppendEmpty()
			lib.Scope().CopyTo(newLibSub.Scope())

			logs := lib.LogRecords()
			logsSub := newLibSub.LogRecords()
			jSub++

			k := 0
			if i == from.resource && j == from.library {
				k = from.record
			}

			for kSub := 0; k < logs.Len(); k++ { //revive:disable-line:var-naming
				logs.At(k).CopyTo(logsSub.AppendEmpty())
				kSub++
			}
		}
	}
}

func subMetricsByType(src pmetric.Metrics, from *index, dst pmetric.Metrics) {
	if from == nil {
		return // All the data of this type was sent successfully
	}

	resources := src.ResourceMetrics()
	resourcesSub := dst.ResourceMetrics()

	for i := from.resource; i < resources.Len(); i++ {
		newSub := resourcesSub.AppendEmpty()
		resources.At(i).Resource().CopyTo(newSub.Resource())

		libraries := resources.At(i).ScopeMetrics()
		librariesSub := newSub.ScopeMetrics()

		j := 0
		if i == from.resource {
			j = from.library
		}
		for jSub := 0; j < libraries.Len(); j++ {
			lib := libraries.At(j)

			newLibSub := librariesSub.AppendEmpty()
			lib.Scope().CopyTo(newLibSub.Scope())

			metrics := lib.Metrics()
			metricsSub := newLibSub.Metrics()
			jSub++

			k := 0
			if i == from.resource && j == from.library {
				k = from.record
			}

			for kSub := 0; k < metrics.Len(); k++ { //revive:disable-line:var-naming
				metrics.At(k).CopyTo(metricsSub.AppendEmpty())
				kSub++
			}
		}
	}
}

func subTracesByType(src ptrace.Traces, from *index, dst ptrace.Traces) {
	if from == nil {
		return // All the data of this type was sent successfully
	}

	resources := src.ResourceSpans()
	resourcesSub := dst.ResourceSpans()

	for i := from.resource; i < resources.Len(); i++ {
		newSub := resourcesSub.AppendEmpty()
		resources.At(i).Resource().CopyTo(newSub.Resource())

		libraries := resources.At(i).ScopeSpans()
		librariesSub := newSub.ScopeSpans()

		j := 0
		if i == from.resource {
			j = from.library
		}
		for jSub := 0; j < libraries.Len(); j++ {
			lib := libraries.At(j)

			newLibSub := librariesSub.AppendEmpty()
			lib.Scope().CopyTo(newLibSub.Scope())

			traces := lib.Spans()
			tracesSub := newLibSub.Spans()
			jSub++

			k := 0
			if i == from.resource && j == from.library {
				k = from.record
			}

			for kSub := 0; k < traces.Len(); k++ { //revive:disable-line:var-naming
				traces.At(k).CopyTo(tracesSub.AppendEmpty())
				kSub++
			}
		}
	}
}

func (c *client) stop(context.Context) error {
	c.wg.Wait()
	if c.heartbeater != nil {
		c.heartbeater.shutdown()
	}
	return nil
}

func (c *client) start(ctx context.Context, host component.Host) (err error) {

	httpClient, err := buildHTTPClient(c.config, host, c.telemetrySettings)
	if err != nil {
		return err
	}

	if c.config.HecHealthCheckEnabled {
		healthCheckURL, _ := c.config.getURL()
		healthCheckURL.Path = c.config.HealthPath
		if err := checkHecHealth(httpClient, healthCheckURL); err != nil {
			return fmt.Errorf("health check failed: %w", err)
		}
	}
	url, _ := c.config.getURL()
	c.hecWorker = &defaultHecWorker{url, httpClient, buildHTTPHeaders(c.config, c.buildInfo)}
	c.heartbeater = newHeartbeater(c.config, c.buildInfo, getPushLogFn(c))
	return nil
}

func checkHecHealth(client *http.Client, healthCheckURL *url.URL) error {

	req, err := http.NewRequest("GET", healthCheckURL.String(), nil)
	if err != nil {
		return consumererror.NewPermanent(err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	err = splunk.HandleHTTPCode(resp)
	if err != nil {
		return err
	}

	return nil
}

func buildHTTPClient(config *Config, host component.Host, telemetrySettings component.TelemetrySettings) (*http.Client, error) {
	// we handle compression explicitly.
	config.HTTPClientSettings.Compression = ""
	if config.MaxConnections != 0 && (config.MaxIdleConns == nil || config.HTTPClientSettings.MaxIdleConnsPerHost == nil) {
		telemetrySettings.Logger.Warn("You are using the deprecated `max_connections` option that will be removed soon; use `max_idle_conns` and/or `max_idle_conns_per_host` instead: https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/exporter/splunkhecexporter#advanced-configuration")
		intMaxConns := int(config.MaxConnections)
		if config.HTTPClientSettings.MaxIdleConns == nil {
			config.HTTPClientSettings.MaxIdleConns = &intMaxConns
		}
		if config.HTTPClientSettings.MaxIdleConnsPerHost == nil {
			config.HTTPClientSettings.MaxIdleConnsPerHost = &intMaxConns
		}
	}
	return config.ToClient(host, telemetrySettings)
}

func buildHTTPHeaders(config *Config, buildInfo component.BuildInfo) map[string]string {
	appVersion := config.SplunkAppVersion
	if appVersion == "" {
		appVersion = buildInfo.Version
	}
	return map[string]string{
		"Connection":           "keep-alive",
		"Content-Type":         "application/json",
		"User-Agent":           config.SplunkAppName + "/" + appVersion,
		"Authorization":        splunk.HECTokenHeader + " " + string(config.Token),
		"__splunk_app_name":    config.SplunkAppName,
		"__splunk_app_version": config.SplunkAppVersion,
	}
}
