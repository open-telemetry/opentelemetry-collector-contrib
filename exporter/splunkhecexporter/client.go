// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter"

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"

	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
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

// iterState captures a state of iteration over the pdata Logs/Metrics/Traces instances.
type iterState struct {
	resource int // index in ResourceLogs/ResourceMetrics/ResourceSpans list
	library  int // index in ScopeLogs/ScopeMetrics/ScopeSpans list
	record   int // index in Logs/Metrics/Spans list
	done     bool
}

func (s iterState) empty() bool {
	return s.resource == 0 && s.library == 0 && s.record == 0
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
	bufferPool        bufferPool
	exporterName      string
}

var jsonStreamPool = sync.Pool{
	New: func() any {
		return jsoniter.NewStream(jsoniter.ConfigDefault, nil, 512)
	},
}

func newClient(set exporter.CreateSettings, cfg *Config, maxContentLength uint) *client {
	return &client{
		config:            cfg,
		logger:            set.Logger,
		telemetrySettings: set.TelemetrySettings,
		buildInfo:         set.BuildInfo,
		bufferPool:        newBufferPool(maxContentLength, !cfg.DisableCompression),
		exporterName:      set.ID.String(),
	}
}

func newLogsClient(set exporter.CreateSettings, cfg *Config) *client {
	return newClient(set, cfg, cfg.MaxContentLengthLogs)
}

func newTracesClient(set exporter.CreateSettings, cfg *Config) *client {
	return newClient(set, cfg, cfg.MaxContentLengthTraces)
}

func newMetricsClient(set exporter.CreateSettings, cfg *Config) *client {
	return newClient(set, cfg, cfg.MaxContentLengthMetrics)
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

	if c.config.UseMultiMetricFormat {
		return c.pushMultiMetricsDataInBatches(ctx, md, localHeaders)
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

	if ld.ResourceLogs().Len() == 0 {
		return nil
	}

	localHeaders := map[string]string{}

	// All logs in a batch have the same access token after batchperresourceattr, so we can just check the first one.
	accessToken, found := ld.ResourceLogs().At(0).Resource().Attributes().Get(splunk.HecTokenLabel)
	if found {
		localHeaders["Authorization"] = splunk.HECTokenHeader + " " + accessToken.Str()
	}

	// All logs in a batch have only one type (regular or profiling logs) after perScopeBatcher,
	// so we can just check the first one.
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		sls := ld.ResourceLogs().At(i).ScopeLogs()
		if sls.Len() > 0 {
			if isProfilingData(sls.At(0)) {
				localHeaders[libraryHeaderName] = profilingLibraryName
			}
			break
		}
	}

	return c.pushLogDataInBatches(ctx, ld, localHeaders)
}

// A guesstimated value > length of bytes of a single event.
// Added to buffer capacity so that buffer is likely to grow by reslicing when buf.Len() > bufCap.
const bufCapPadding = uint(4096)
const libraryHeaderName = "X-Splunk-Instrumentation-Library"
const profilingLibraryName = "otel.profiling"

func isProfilingData(sl plog.ScopeLogs) bool {
	return sl.Scope().Name() == profilingLibraryName
}

// pushLogDataInBatches sends batches of Splunk events in JSON format.
// The batch content length is restricted to MaxContentLengthLogs.
// ld log records are parsed to Splunk events.
func (c *client) pushLogDataInBatches(ctx context.Context, ld plog.Logs, headers map[string]string) error {
	buf := c.bufferPool.get()
	defer c.bufferPool.put(buf)
	is := iterState{}
	var permanentErrors []error

	for !is.done {
		buf.Reset()
		latestIterState, batchPermanentErrors := c.fillLogsBuffer(ld, buf, is)
		permanentErrors = append(permanentErrors, batchPermanentErrors...)
		if !buf.Empty() {
			if err := c.postEvents(ctx, buf, headers); err != nil {
				return consumererror.NewLogs(err, subLogs(ld, is))
			}
		}
		is = latestIterState
	}

	return multierr.Combine(permanentErrors...)
}

// fillLogsBuffer fills the buffer with Splunk events until the buffer is full or all logs are processed.
func (c *client) fillLogsBuffer(logs plog.Logs, buf buffer, is iterState) (iterState, []error) {
	var b []byte
	var permanentErrors []error
	jsonStream := jsonStreamPool.Get().(*jsoniter.Stream)
	defer jsonStreamPool.Put(jsonStream)

	for i := is.resource; i < logs.ResourceLogs().Len(); i++ {
		rl := logs.ResourceLogs().At(i)
		for j := is.library; j < rl.ScopeLogs().Len(); j++ {
			is.library = 0 // Reset library index for next resource.
			sl := rl.ScopeLogs().At(j)
			for k := is.record; k < sl.LogRecords().Len(); k++ {
				is.record = 0 // Reset record index for next library.
				logRecord := sl.LogRecords().At(k)

				if c.config.ExportRaw {
					b = []byte(logRecord.Body().AsString() + "\n")
				} else {
					// Parsing log record to Splunk event.
					event := mapLogRecordToSplunkEvent(rl.Resource(), logRecord, c.config)

					// JSON encoding event and writing to buffer.
					var err error
					b, err = marshalEvent(event, c.config.MaxEventSize, jsonStream)
					if err != nil {
						permanentErrors = append(permanentErrors, consumererror.NewPermanent(fmt.Errorf(
							"dropped log event: %v, error: %w", event, err)))
						continue
					}
				}

				// Continue adding events to buffer up to capacity.
				_, err := buf.Write(b)
				if err == nil {
					continue
				}
				if errors.Is(err, errOverCapacity) {
					if !buf.Empty() {
						return iterState{i, j, k, false}, permanentErrors
					}
					permanentErrors = append(permanentErrors, consumererror.NewPermanent(
						fmt.Errorf("dropped log event: error: event size %d bytes larger than configured max"+
							" content length %d bytes", len(b), c.config.MaxContentLengthLogs)))
					return iterState{i, j, k + 1, false}, permanentErrors
				}
				permanentErrors = append(permanentErrors,
					consumererror.NewPermanent(fmt.Errorf("error writing the event: %w", err)))
			}
		}
	}

	return iterState{done: true}, permanentErrors
}

func (c *client) fillMetricsBuffer(metrics pmetric.Metrics, buf buffer, is iterState) (iterState, []error) {
	var permanentErrors []error
	jsonStream := jsonStreamPool.Get().(*jsoniter.Stream)
	defer jsonStreamPool.Put(jsonStream)

	tempBuf := bytes.NewBuffer(make([]byte, 0, c.config.MaxContentLengthMetrics))
	for i := is.resource; i < metrics.ResourceMetrics().Len(); i++ {
		rm := metrics.ResourceMetrics().At(i)
		for j := is.library; j < rm.ScopeMetrics().Len(); j++ {
			is.library = 0 // Reset library index for next resource.
			sm := rm.ScopeMetrics().At(j)
			for k := is.record; k < sm.Metrics().Len(); k++ {
				is.record = 0 // Reset record index for next library.
				metric := sm.Metrics().At(k)

				// Parsing metric record to Splunk event.
				events := mapMetricToSplunkEvent(rm.Resource(), metric, c.config, c.logger)
				tempBuf.Reset()
				for _, event := range events {
					// JSON encoding event and writing to buffer.
					b, err := marshalEvent(event, c.config.MaxEventSize, jsonStream)
					if err != nil {
						permanentErrors = append(permanentErrors, consumererror.NewPermanent(fmt.Errorf("dropped metric event: %v, error: %w", event, err)))
						continue
					}
					tempBuf.Write(b)
				}

				// Continue adding events to buffer up to capacity.
				b := tempBuf.Bytes()
				_, err := buf.Write(b)
				if err == nil {
					continue
				}
				if errors.Is(err, errOverCapacity) {
					if !buf.Empty() {
						return iterState{i, j, k, false}, permanentErrors
					}
					permanentErrors = append(permanentErrors, consumererror.NewPermanent(
						fmt.Errorf("dropped metric event: error: event size %d bytes larger than configured max"+
							" content length %d bytes", len(b), c.config.MaxContentLengthMetrics)))
					return iterState{i, j, k + 1, false}, permanentErrors
				}
				permanentErrors = append(permanentErrors, consumererror.NewPermanent(fmt.Errorf(
					"error writing the event: %w", err)))
			}
		}
	}

	return iterState{done: true}, permanentErrors
}

func (c *client) fillMetricsBufferMultiMetrics(events []*splunk.Event, buf buffer, is iterState) (iterState, []error) {
	var permanentErrors []error
	jsonStream := jsonStreamPool.Get().(*jsoniter.Stream)
	defer jsonStreamPool.Put(jsonStream)

	for i := is.record; i < len(events); i++ {
		event := events[i]
		// JSON encoding event and writing to buffer.
		b, jsonErr := marshalEvent(event, c.config.MaxEventSize, jsonStream)
		if jsonErr != nil {
			permanentErrors = append(permanentErrors, consumererror.NewPermanent(fmt.Errorf("dropped metric event: %v, error: %w", event, jsonErr)))
			continue
		}
		_, err := buf.Write(b)
		if errors.Is(err, errOverCapacity) {
			if !buf.Empty() {
				return iterState{
					record: i,
					done:   false,
				}, permanentErrors
			}
			permanentErrors = append(permanentErrors, consumererror.NewPermanent(
				fmt.Errorf("dropped metric event: error: event size %d bytes larger than configured max"+
					" content length %d bytes", len(b), c.config.MaxContentLengthMetrics)))
			return iterState{
				record: i + 1,
				done:   i+1 != len(events),
			}, permanentErrors
		} else if err != nil {
			permanentErrors = append(permanentErrors, consumererror.NewPermanent(fmt.Errorf(
				"error writing the event: %w", err)))
		}
	}

	return iterState{done: true}, permanentErrors
}

func (c *client) fillTracesBuffer(traces ptrace.Traces, buf buffer, is iterState) (iterState, []error) {
	var permanentErrors []error
	jsonStream := jsonStreamPool.Get().(*jsoniter.Stream)
	defer jsonStreamPool.Put(jsonStream)

	for i := is.resource; i < traces.ResourceSpans().Len(); i++ {
		rs := traces.ResourceSpans().At(i)
		for j := is.library; j < rs.ScopeSpans().Len(); j++ {
			is.library = 0 // Reset library index for next resource.
			ss := rs.ScopeSpans().At(j)
			for k := is.record; k < ss.Spans().Len(); k++ {
				is.record = 0 // Reset record index for next library.
				span := ss.Spans().At(k)

				// Parsing span record to Splunk event.
				event := mapSpanToSplunkEvent(rs.Resource(), span, c.config)

				// JSON encoding event and writing to buffer.
				b, err := marshalEvent(event, c.config.MaxEventSize, jsonStream)
				if err != nil {
					permanentErrors = append(permanentErrors, consumererror.NewPermanent(fmt.Errorf("dropped span events: %v, error: %w", event, err)))
					continue
				}

				// Continue adding events to buffer up to capacity.
				_, err = buf.Write(b)
				if err == nil {
					continue
				}
				if errors.Is(err, errOverCapacity) {
					if !buf.Empty() {
						return iterState{i, j, k, false}, permanentErrors
					}
					permanentErrors = append(permanentErrors, consumererror.NewPermanent(
						fmt.Errorf("dropped span event: error: event size %d bytes larger than configured max"+
							" content length %d bytes", len(b), c.config.MaxContentLengthTraces)))
					return iterState{i, j, k + 1, false}, permanentErrors
				}
				permanentErrors = append(permanentErrors, consumererror.NewPermanent(fmt.Errorf(
					"error writing the event: %w", err)))
			}
		}
	}

	return iterState{done: true}, permanentErrors
}

// pushMultiMetricsDataInBatches sends batches of Splunk multi-metric events in JSON format.
// The batch content length is restricted to MaxContentLengthMetrics.
// md metrics are parsed to Splunk events.
func (c *client) pushMultiMetricsDataInBatches(ctx context.Context, md pmetric.Metrics, headers map[string]string) error {
	buf := c.bufferPool.get()
	defer c.bufferPool.put(buf)
	is := iterState{}

	var permanentErrors []error
	var events []*splunk.Event
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			for k := 0; k < sm.Metrics().Len(); k++ {
				metric := sm.Metrics().At(k)

				// Parsing metric record to Splunk event.
				events = append(events, mapMetricToSplunkEvent(rm.Resource(), metric, c.config, c.logger)...)
			}
		}
	}

	merged, err := mergeEventsToMultiMetricFormat(events)
	if err != nil {
		return consumererror.NewPermanent(fmt.Errorf("error merging events: %w", err))
	}

	for !is.done {
		buf.Reset()

		latestIterState, batchPermanentErrors := c.fillMetricsBufferMultiMetrics(merged, buf, is)
		permanentErrors = append(permanentErrors, batchPermanentErrors...)
		if !buf.Empty() {
			if err := c.postEvents(ctx, buf, headers); err != nil {
				return consumererror.NewMetrics(err, md)
			}
		}

		is = latestIterState
	}

	return multierr.Combine(permanentErrors...)
}

// pushMetricsDataInBatches sends batches of Splunk events in JSON format.
// The batch content length is restricted to MaxContentLengthMetrics.
// md metrics are parsed to Splunk events.
func (c *client) pushMetricsDataInBatches(ctx context.Context, md pmetric.Metrics, headers map[string]string) error {
	buf := c.bufferPool.get()
	defer c.bufferPool.put(buf)
	is := iterState{}
	var permanentErrors []error

	for !is.done {
		buf.Reset()
		latestIterState, batchPermanentErrors := c.fillMetricsBuffer(md, buf, is)
		permanentErrors = append(permanentErrors, batchPermanentErrors...)
		if !buf.Empty() {
			if err := c.postEvents(ctx, buf, headers); err != nil {
				return consumererror.NewMetrics(err, subMetrics(md, is))
			}
		}

		is = latestIterState
	}

	return multierr.Combine(permanentErrors...)
}

// pushTracesDataInBatches sends batches of Splunk events in JSON format.
// The batch content length is restricted to MaxContentLengthMetrics.
// td traces are parsed to Splunk events.
func (c *client) pushTracesDataInBatches(ctx context.Context, td ptrace.Traces, headers map[string]string) error {
	buf := c.bufferPool.get()
	defer c.bufferPool.put(buf)
	is := iterState{}
	var permanentErrors []error

	for !is.done {
		buf.Reset()
		latestIterState, batchPermanentErrors := c.fillTracesBuffer(td, buf, is)
		permanentErrors = append(permanentErrors, batchPermanentErrors...)
		if !buf.Empty() {
			if err := c.postEvents(ctx, buf, headers); err != nil {
				return consumererror.NewTraces(err, subTraces(td, is))
			}
		}
		is = latestIterState
	}

	return multierr.Combine(permanentErrors...)
}

func (c *client) postEvents(ctx context.Context, buf buffer, headers map[string]string) error {
	if err := buf.Close(); err != nil {
		return err
	}
	return c.hecWorker.send(ctx, buf, headers)
}

// subLogs returns a subset of logs starting from the state.
func subLogs(src plog.Logs, state iterState) plog.Logs {
	if state.empty() {
		return src
	}

	dst := plog.NewLogs()
	resources := src.ResourceLogs()
	resourcesSub := dst.ResourceLogs()

	for i := state.resource; i < resources.Len(); i++ {
		newSub := resourcesSub.AppendEmpty()
		resources.At(i).Resource().CopyTo(newSub.Resource())

		libraries := resources.At(i).ScopeLogs()
		librariesSub := newSub.ScopeLogs()

		j := 0
		if i == state.resource {
			j = state.library
		}
		for ; j < libraries.Len(); j++ {
			lib := libraries.At(j)

			newLibSub := librariesSub.AppendEmpty()
			lib.Scope().CopyTo(newLibSub.Scope())

			logs := lib.LogRecords()
			logsSub := newLibSub.LogRecords()

			k := 0
			if i == state.resource && j == state.library {
				k = state.record
			}
			for ; k < logs.Len(); k++ {
				logs.At(k).CopyTo(logsSub.AppendEmpty())
			}
		}
	}

	return dst
}

// subMetrics returns a subset of metrics starting from the state.
func subMetrics(src pmetric.Metrics, state iterState) pmetric.Metrics {
	if state.empty() {
		return src
	}

	dst := pmetric.NewMetrics()
	resources := src.ResourceMetrics()
	resourcesSub := dst.ResourceMetrics()

	for i := state.resource; i < resources.Len(); i++ {
		newSub := resourcesSub.AppendEmpty()
		resources.At(i).Resource().CopyTo(newSub.Resource())

		libraries := resources.At(i).ScopeMetrics()
		librariesSub := newSub.ScopeMetrics()

		j := 0
		if i == state.resource {
			j = state.library
		}
		for ; j < libraries.Len(); j++ {
			lib := libraries.At(j)

			newLibSub := librariesSub.AppendEmpty()
			lib.Scope().CopyTo(newLibSub.Scope())

			metrics := lib.Metrics()
			metricsSub := newLibSub.Metrics()

			k := 0
			if i == state.resource && j == state.library {
				k = state.record
			}
			for ; k < metrics.Len(); k++ {
				metrics.At(k).CopyTo(metricsSub.AppendEmpty())
			}
		}
	}

	return dst
}

func subTraces(src ptrace.Traces, state iterState) ptrace.Traces {
	if state.empty() {
		return src
	}

	dst := ptrace.NewTraces()
	resources := src.ResourceSpans()
	resourcesSub := dst.ResourceSpans()

	for i := state.resource; i < resources.Len(); i++ {
		newSub := resourcesSub.AppendEmpty()
		resources.At(i).Resource().CopyTo(newSub.Resource())

		libraries := resources.At(i).ScopeSpans()
		librariesSub := newSub.ScopeSpans()

		j := 0
		if i == state.resource {
			j = state.library
		}
		for ; j < libraries.Len(); j++ {
			lib := libraries.At(j)

			newLibSub := librariesSub.AppendEmpty()
			lib.Scope().CopyTo(newLibSub.Scope())

			traces := lib.Spans()
			tracesSub := newLibSub.Spans()

			k := 0
			if i == state.resource && j == state.library {
				k = state.record
			}
			for ; k < traces.Len(); k++ {
				traces.At(k).CopyTo(tracesSub.AppendEmpty())
			}
		}
	}

	return dst
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
		if err := checkHecHealth(ctx, httpClient, healthCheckURL); err != nil {
			return fmt.Errorf("%s: health check failed: %w", c.exporterName, err)
		}
	}
	url, _ := c.config.getURL()
	c.hecWorker = &defaultHecWorker{url, httpClient, buildHTTPHeaders(c.config, c.buildInfo)}
	c.heartbeater = newHeartbeater(c.config, c.buildInfo, getPushLogFn(c))
	if c.config.Heartbeat.Startup {
		if err := c.heartbeater.sendHeartbeat(c.config, c.buildInfo, getPushLogFn(c)); err != nil {
			return fmt.Errorf("%s: heartbeat on startup failed: %w", c.exporterName, err)
		}
	}
	return nil
}

func checkHecHealth(ctx context.Context, client *http.Client, healthCheckURL *url.URL) error {

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthCheckURL.String(), nil)
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

// marshalEvent marshals an event to JSON using a reusable jsoniter stream.
func marshalEvent(event *splunk.Event, sizeLimit uint, stream *jsoniter.Stream) ([]byte, error) {
	stream.Reset(nil)
	stream.Error = nil
	stream.WriteVal(event)
	if stream.Error != nil {
		return nil, stream.Error
	}
	if uint(stream.Buffered()) > sizeLimit {
		return nil, fmt.Errorf("event size %d exceeds limit %d", stream.Buffered(), sizeLimit)
	}
	return stream.Buffer(), nil
}
