// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package prometheusremotewritereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusremotewritereceiver"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/gogo/protobuf/proto"
	promconfig "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/model/labels"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	promremote "github.com/prometheus/prometheus/storage/remote"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap/zapcore"
)

func newRemoteWriteReceiver(settings receiver.Settings, cfg *Config, nextConsumer consumer.Metrics) (receiver.Metrics, error) {
	return &prometheusRemoteWriteReceiver{
		settings:     settings,
		nextConsumer: nextConsumer,
		config:       cfg,
		server: &http.Server{
			ReadTimeout: 60 * time.Second,
		},
	}, nil
}

type prometheusRemoteWriteReceiver struct {
	settings     receiver.Settings
	nextConsumer consumer.Metrics

	config *Config
	server *http.Server
}

func (prw *prometheusRemoteWriteReceiver) Start(ctx context.Context, host component.Host) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/write", prw.handlePRW)
	var err error

	prw.server, err = prw.config.ToServer(ctx, host, prw.settings.TelemetrySettings, mux)
	if err != nil {
		return fmt.Errorf("failed to create server definition: %w", err)
	}
	listener, err := prw.config.ToListener(ctx)
	if err != nil {
		return fmt.Errorf("failed to create prometheus remote-write listener: %w", err)
	}

	go func() {
		if err := prw.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(fmt.Errorf("error starting prometheus remote-write receiver: %w", err)))
		}
	}()
	return nil
}

func (prw *prometheusRemoteWriteReceiver) Shutdown(ctx context.Context) error {
	if prw.server == nil {
		return nil
	}
	return prw.server.Shutdown(ctx)
}

func (prw *prometheusRemoteWriteReceiver) handlePRW(w http.ResponseWriter, req *http.Request) {
	contentType := req.Header.Get("Content-Type")
	if contentType == "" {
		prw.settings.Logger.Warn("message received without Content-Type header, rejecting")
		http.Error(w, "Content-Type header is required", http.StatusUnsupportedMediaType)
		return
	}

	msgType, err := prw.parseProto(contentType)
	if err != nil {
		prw.settings.Logger.Warn("Error decoding remote-write request", zapcore.Field{Key: "error", Type: zapcore.ErrorType, Interface: err})
		http.Error(w, err.Error(), http.StatusUnsupportedMediaType)
		return
	}
	if msgType != promconfig.RemoteWriteProtoMsgV2 {
		prw.settings.Logger.Warn("message received with unsupported proto version, rejecting")
		http.Error(w, "Unsupported proto version", http.StatusUnsupportedMediaType)
		return
	}

	// After parsing the content-type header, the next step would be to handle content-encoding.
	// Luckly confighttp's Server has middleware that already decompress the request body for us.

	body, err := io.ReadAll(req.Body)
	if err != nil {
		prw.settings.Logger.Warn("Error decoding remote write request", zapcore.Field{Key: "error", Type: zapcore.ErrorType, Interface: err})
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var prw2Req writev2.Request
	if err = proto.Unmarshal(body, &prw2Req); err != nil {
		prw.settings.Logger.Warn("Error decoding remote write request", zapcore.Field{Key: "error", Type: zapcore.ErrorType, Interface: err})
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, stats, err := prw.translateV2(req.Context(), &prw2Req)
	stats.SetHeaders(w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest) // Following instructions at https://prometheus.io/docs/specs/remote_write_spec_2_0/#invalid-samples
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// parseProto parses the content-type header and returns the version of the remote-write protocol.
// We can't expect that senders of remote-write v1 will add the "proto=" parameter since it was not
// a requirement in v1. So, if the parameter is not found, we assume v1.
func (prw *prometheusRemoteWriteReceiver) parseProto(contentType string) (promconfig.RemoteWriteProtoMsg, error) {
	contentType = strings.TrimSpace(contentType)

	parts := strings.Split(contentType, ";")
	if parts[0] != "application/x-protobuf" {
		return "", fmt.Errorf("expected %q as the first (media) part, got %v content-type", "application/x-protobuf", contentType)
	}

	for _, part := range parts[1:] {
		parameter := strings.Split(part, "=")
		if len(parameter) != 2 {
			return "", fmt.Errorf("as per https://www.rfc-editor.org/rfc/rfc9110#parameter expected parameters to be key-values, got %v in %v content-type", part, contentType)
		}

		if strings.TrimSpace(parameter[0]) == "proto" {
			ret := promconfig.RemoteWriteProtoMsg(parameter[1])
			if err := ret.Validate(); err != nil {
				return "", fmt.Errorf("got %v content type; %w", contentType, err)
			}
			return ret, nil
		}
	}

	// No "proto=" parameter found, assume v1.
	return promconfig.RemoteWriteProtoMsgV1, nil
}

// translateV2 translates a v2 remote-write request into OTLP metrics.
// translate is not feature complete.
//
//nolint:unparam
func (prw *prometheusRemoteWriteReceiver) translateV2(_ context.Context, req *writev2.Request) (pmetric.Metrics, promremote.WriteResponseStats, error) {
	var (
		badRequestErrors error
		otelMetrics      = pmetric.NewMetrics()
		labelsBuilder    = labels.NewScratchBuilder(0)
		stats            = promremote.WriteResponseStats{}
		// Prometheus Remote-Write can send multiple time series with the same labels in the same request.
		// Instead of creating a whole new OTLP metric, we just append the new sample to the existing OTLP metric.
		// This cache is called "intra" because in the future we'll have a "interRequestCache" to cache resourceAttributes
		// between requests based on the metric "target_info".
		intraRequestCache = make(map[uint64]pmetric.ResourceMetrics)
	)

	for _, ts := range req.Timeseries {
		ls := ts.ToLabels(&labelsBuilder, req.Symbols)

		if !ls.Has(labels.MetricName) {
			badRequestErrors = errors.Join(badRequestErrors, fmt.Errorf("missing metric name in labels"))
			continue
		} else if duplicateLabel, hasDuplicate := ls.HasDuplicateLabelNames(); hasDuplicate {
			badRequestErrors = errors.Join(badRequestErrors, fmt.Errorf("duplicate label %q in labels", duplicateLabel))
			continue
		}

		var rm pmetric.ResourceMetrics
		hashedLabels := xxhash.Sum64String(ls.Get("job") + string([]byte{'\xff'}) + ls.Get("instance"))
		intraCacheEntry, ok := intraRequestCache[hashedLabels]
		if ok {
			// We found the same time series in the same request, so we should append to the same OTLP metric.
			rm = intraCacheEntry
		} else {
			rm = otelMetrics.ResourceMetrics().AppendEmpty()
			parseJobAndInstance(rm.Resource().Attributes(), ls.Get("job"), ls.Get("instance"))
			intraRequestCache[hashedLabels] = rm
		}

		switch ts.Metadata.Type {
		case writev2.Metadata_METRIC_TYPE_COUNTER:
			prw.addCounterDatapoints(rm, ls, ts)
		case writev2.Metadata_METRIC_TYPE_GAUGE:
			prw.addGaugeDatapoints(rm, ls, ts)
		case writev2.Metadata_METRIC_TYPE_SUMMARY:
			prw.addSummaryDatapoints(rm, ls, ts)
		case writev2.Metadata_METRIC_TYPE_HISTOGRAM:
			prw.addHistogramDatapoints(rm, ls, ts)
		default:
			badRequestErrors = errors.Join(badRequestErrors, fmt.Errorf("unsupported metric type %q for metric %q", ts.Metadata.Type, ls.Get(labels.MetricName)))
		}
	}

	return otelMetrics, stats, badRequestErrors
}

// parseJobAndInstance turns the job and instance labels service resource attributes.
// Following the specification at https://opentelemetry.io/docs/specs/otel/compatibility/prometheus_and_openmetrics/
func parseJobAndInstance(dest pcommon.Map, job, instance string) {
	if instance != "" {
		dest.PutStr("service.instance.id", instance)
	}
	if job != "" {
		parts := strings.Split(job, "/")
		if len(parts) == 2 {
			dest.PutStr("service.namespace", parts[0])
			dest.PutStr("service.name", parts[1])
			return
		}
		dest.PutStr("service.name", job)
	}
}

func (prw *prometheusRemoteWriteReceiver) addCounterDatapoints(_ pmetric.ResourceMetrics, _ labels.Labels, _ writev2.TimeSeries) {
	// TODO: Implement this function
}

func (prw *prometheusRemoteWriteReceiver) addGaugeDatapoints(rm pmetric.ResourceMetrics, ls labels.Labels, ts writev2.TimeSeries) {
	// TODO: Cache metric name+type+unit and look up cache before creating new empty metric.
	// In OTel name+type+unit is the unique identifier of a metric and we should not create
	// a new metric if it already exists.

	scopeName, scopeVersion := prw.extractScopeInfo(ls)

	// Check if the name and version present in the labels are already present in the ResourceMetrics.
	// If it is not present, we should create a new ScopeMetrics.
	// Otherwise, we should append to the existing ScopeMetrics.
	for j := 0; j < rm.ScopeMetrics().Len(); j++ {
		scope := rm.ScopeMetrics().At(j)
		if scopeName == scope.Scope().Name() && scopeVersion == scope.Scope().Version() {
			addDatapoints(scope.Metrics().AppendEmpty().SetEmptyGauge().DataPoints(), ls, ts)
			return
		}
	}

	scope := rm.ScopeMetrics().AppendEmpty()
	scope.Scope().SetName(scopeName)
	scope.Scope().SetVersion(scopeVersion)
	m := scope.Metrics().AppendEmpty().SetEmptyGauge()
	addDatapoints(m.DataPoints(), ls, ts)
}

func (prw *prometheusRemoteWriteReceiver) addSummaryDatapoints(_ pmetric.ResourceMetrics, _ labels.Labels, _ writev2.TimeSeries) {
	// TODO: Implement this function
}

func (prw *prometheusRemoteWriteReceiver) addHistogramDatapoints(_ pmetric.ResourceMetrics, _ labels.Labels, _ writev2.TimeSeries) {
	// TODO: Implement this function
}

// addDatapoints adds the labels to the datapoints attributes.
// TODO: We're still not handling the StartTimestamp.
func addDatapoints(datapoints pmetric.NumberDataPointSlice, ls labels.Labels, ts writev2.TimeSeries) {
	// Add samples from the timeseries
	for _, sample := range ts.Samples {
		dp := datapoints.AppendEmpty()

		// Set timestamp in nanoseconds (Prometheus uses milliseconds)
		dp.SetTimestamp(pcommon.Timestamp(sample.Timestamp * int64(time.Millisecond)))
		dp.SetDoubleValue(sample.Value)

		attributes := dp.Attributes()
		for _, l := range ls {
			if l.Name == "instance" || l.Name == "job" || // Become resource attributes
				l.Name == labels.MetricName || // Becomes metric name
				l.Name == "otel_scope_name" || l.Name == "otel_scope_version" { // Becomes scope name and version
				continue
			}
			attributes.PutStr(l.Name, l.Value)
		}
	}
}

// extractScopeInfo extracts the scope name and version from the labels. If the labels do not contain the scope name/version,
// it will use the default values from the settings.
func (prw *prometheusRemoteWriteReceiver) extractScopeInfo(ls labels.Labels) (string, string) {
	scopeName := prw.settings.BuildInfo.Description
	scopeVersion := prw.settings.BuildInfo.Version

	if sName := ls.Get("otel_scope_name"); sName != "" {
		scopeName = sName
	}

	if sVersion := ls.Get("otel_scope_version"); sVersion != "" {
		scopeVersion = sVersion
	}
	return scopeName, scopeVersion
}
