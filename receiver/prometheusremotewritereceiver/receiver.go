// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package prometheusremotewritereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusremotewritereceiver"

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/gogo/protobuf/proto"
	lru "github.com/hashicorp/golang-lru/v2"
	remoteapi "github.com/prometheus/client_golang/exp/api/remote"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/value"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"github.com/prometheus/prometheus/schema"
	promremote "github.com/prometheus/prometheus/storage/remote"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap/zapcore"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
)

func newRemoteWriteReceiver(settings receiver.Settings, cfg *Config, nextConsumer consumer.Metrics) (receiver.Metrics, error) {
	cache, err := lru.New[uint64, pmetric.ResourceMetrics](1000)
	if err != nil {
		return nil, fmt.Errorf("failed to create LRU cache: %w", err)
	}
	return &prometheusRemoteWriteReceiver{
		settings:     settings,
		nextConsumer: nextConsumer,
		config:       cfg,
		server: &http.Server{
			ReadTimeout: 60 * time.Second,
		},
		rmCache: cache,
		bodyBufferPool: &sync.Pool{
			New: func() any {
				// Pre-allocate 4KiB
				return bytes.NewBuffer(make([]byte, 0, 4*1024))
			},
		},
	}, nil
}

type prometheusRemoteWriteReceiver struct {
	settings     receiver.Settings
	nextConsumer consumer.Metrics

	config *Config
	server *http.Server
	wg     sync.WaitGroup

	rmCache *lru.Cache[uint64, pmetric.ResourceMetrics]
	obsrecv *receiverhelper.ObsReport

	bodyBufferPool *sync.Pool
}

// metricIdentity contains all the components that uniquely identify a metric
// according to the OpenTelemetry Protocol data model.
// The definition of the metric uniqueness is based on the following document. Ref: https://opentelemetry.io/docs/specs/otel/metrics/data-model/#opentelemetry-protocol-data-model
type metricIdentity struct {
	ResourceID   string
	ScopeName    string
	ScopeVersion string
	MetricName   string
	Unit         string
	Type         writev2.Metadata_MetricType
}

// createMetricIdentity creates a metricIdentity struct from the required components
func createMetricIdentity(resourceID, scopeName, scopeVersion, metricName, unit string, metricType writev2.Metadata_MetricType) metricIdentity {
	return metricIdentity{
		ResourceID:   resourceID,
		ScopeName:    scopeName,
		ScopeVersion: scopeVersion,
		MetricName:   metricName,
		Unit:         unit,
		Type:         metricType,
	}
}

// Hash generates a unique hash for the metric identity
func (mi metricIdentity) Hash() uint64 {
	const separator = "\xff"

	combined := strings.Join([]string{
		mi.ResourceID,
		mi.ScopeName,
		mi.ScopeVersion,
		mi.MetricName,
		mi.Unit,
		fmt.Sprintf("%d", mi.Type),
	}, separator)

	return xxhash.Sum64String(combined)
}

func (prw *prometheusRemoteWriteReceiver) Start(ctx context.Context, host component.Host) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/write", prw.handlePRW)
	var err error
	prw.obsrecv, err = receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             prw.settings.ID,
		ReceiverCreateSettings: prw.settings,
		Transport:              "http",
	})
	if err != nil {
		return fmt.Errorf("failed to create obsreport: %w", err)
	}

	prw.server, err = prw.config.ToServer(ctx, host.GetExtensions(), prw.settings.TelemetrySettings, mux)
	if err != nil {
		return fmt.Errorf("failed to create server definition: %w", err)
	}
	listener, err := prw.config.ToListener(ctx)
	if err != nil {
		return fmt.Errorf("failed to create prometheus remote-write listener: %w", err)
	}

	prw.wg.Add(1)
	go func() {
		defer prw.wg.Done()
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
	err := prw.server.Shutdown(ctx)
	if err == nil {
		// Only wait if Shutdown returns successfully,
		// otherwise we may block indefinitely.
		prw.wg.Wait()
	}
	return err
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
	if msgType != remoteapi.WriteV2MessageType {
		prw.settings.Logger.Warn("message received with unsupported proto version, rejecting")
		http.Error(w, "Unsupported proto version", http.StatusUnsupportedMediaType)
		return
	}

	// After parsing the content-type header, the next step would be to handle content-encoding.
	// Luckly confighttp's Server has middleware that already decompress the request body for us.
	buf := prw.bodyBufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer prw.bodyBufferPool.Put(buf)
	_, err = buf.ReadFrom(req.Body)
	if err != nil {
		prw.settings.Logger.Warn("Error reading remote write request body", zapcore.Field{Key: "error", Type: zapcore.ErrorType, Interface: err})
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var prw2Req writev2.Request
	if err = proto.Unmarshal(buf.Bytes(), &prw2Req); err != nil {
		prw.settings.Logger.Warn("Error decoding remote write request", zapcore.Field{Key: "error", Type: zapcore.ErrorType, Interface: err})
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	m, stats, err := prw.translateV2(req.Context(), &prw2Req)
	stats.SetHeaders(w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest) // Following instructions at https://prometheus.io/docs/specs/remote_write_spec_2_0/#invalid-samples
		return
	}

	// Return early if metric count is 0.
	if m.MetricCount() == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	obsrecvCtx := prw.obsrecv.StartMetricsOp(req.Context())
	err = prw.nextConsumer.ConsumeMetrics(req.Context(), m)
	prw.obsrecv.EndMetricsOp(obsrecvCtx, "prometheusremotewritereceiver", m.ResourceMetrics().Len(), err)
	if err != nil {
		prw.settings.Logger.Error("Error consuming metrics", zapcore.Field{Key: "error", Type: zapcore.ErrorType, Interface: err})
		if consumererror.IsPermanent(err) {
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// parseProto parses the content-type header and returns the version of the remote-write protocol.
// We can't expect that senders of remote-write v1 will add the "proto=" parameter since it was not
// a requirement in v1. So, if the parameter is not found, we assume v1.
func (*prometheusRemoteWriteReceiver) parseProto(contentType string) (remoteapi.WriteMessageType, error) {
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
			ret := remoteapi.WriteMessageType(parameter[1])
			if err := ret.Validate(); err != nil {
				return "", fmt.Errorf("got %v content type; %w", contentType, err)
			}
			return ret, nil
		}
	}

	// No "proto=" parameter found, assume v1.
	return remoteapi.WriteV1MessageType, nil
}

// getOrCreateRM returns or creates the ResourceMetrics for a job/instance pair within an HTTP request.
//
// Two-level cache:
//
//  1. reqRM (per-request): groups samples with the same job/instance into a single ResourceMetrics
//     during current request processing, avoiding duplication in the output.
//
//  2. prw.rmCache (global LRU): stores snapshots of previously seen resource attributes (from target_info),
//     allowing future requests to reuse enriched attributes.
//
// This function always creates new ResourceMetrics per request, only copying attributes
// from the LRU cache when available. Never returns cached objects to avoid shared
// mutation across concurrent requests.
func (prw *prometheusRemoteWriteReceiver) getOrCreateRM(ls labels.Labels, otelMetrics pmetric.Metrics, reqRM map[uint64]pmetric.ResourceMetrics) (pmetric.ResourceMetrics, uint64) {
	hashedLabels := xxhash.Sum64String(ls.Get("job") + string([]byte{'\xff'}) + ls.Get("instance"))

	if rm, ok := reqRM[hashedLabels]; ok {
		return rm, hashedLabels
	}

	rm := otelMetrics.ResourceMetrics().AppendEmpty()
	if existingRM, ok := prw.rmCache.Get(hashedLabels); ok {
		// When the ResourceMetrics already exists in the global cache, we can reuse the previous snapshots and perpass the already seen attributes to the current request.
		existingRM.Resource().Attributes().CopyTo(rm.Resource().Attributes())
	} else {
		// When the ResourceMetrics does not exist in the global cache, we need to create a new one and add it to the request map.
		// Saving the new ResourceMetrics in the global cache to avoid creating duplicates in the next requests.
		parseJobAndInstance(rm.Resource().Attributes(), ls.Get("job"), ls.Get("instance"))
		snapshot := pmetric.NewResourceMetrics()
		rm.Resource().Attributes().CopyTo(snapshot.Resource().Attributes())
		prw.rmCache.Add(hashedLabels, snapshot)
	}

	reqRM[hashedLabels] = rm
	return rm, hashedLabels
}

// translateV2 translates a v2 remote-write request into OTLP metrics.
// translate is not feature complete.
func (prw *prometheusRemoteWriteReceiver) translateV2(_ context.Context, req *writev2.Request) (pmetric.Metrics, promremote.WriteResponseStats, error) {
	var (
		badRequestErrors error
		// otelMetrics represents the final metrics, after all the processing, that will be returned by the receiver.
		otelMetrics   = pmetric.NewMetrics()
		labelsBuilder = labels.NewScratchBuilder(0)
		// More about stats: https://github.com/prometheus/docs/blob/main/docs/specs/prw/remote_write_spec_2_0.md#required-written-response-headers
		// TODO: add exemplars to the stats.
		stats = promremote.WriteResponseStats{
			Confirmed: true,
		}
		// The key is composed by: resource_hash:scope_name:scope_version:metric_name:unit:type
		metricCache = make(map[uint64]pmetric.Metric)
		// modifiedResourceMetric keeps track, for each request, of which resources (identified by the job/instance hash) had their resource attributes modified — for example, through target_info.
		// Once the request is fully processed, only the resource attributes contained in the request’s ResourceMetrics are snapshotted back into the LRU cache.
		// This ensures that future requests start with the enriched resource attributes already applied.
		modifiedResourceMetric = make(map[uint64]pmetric.ResourceMetrics)

		// exemplarMap keeps track of exemplars and key is composed by scope_name:scope_version:metric_name:type
		exemplarMap = collectExemplars(req, prw.settings, &stats)
	)

	for i := range req.Timeseries {
		ts := &req.Timeseries[i]
		ls, err := ts.ToLabels(&labelsBuilder, req.Symbols)
		if err != nil {
			badRequestErrors = errors.Join(badRequestErrors, fmt.Errorf("error converting timeseries to labels: %w", err))
			continue
		}
		metadata := schema.NewMetadataFromLabels(ls)
		if metadata.Name == "" {
			badRequestErrors = errors.Join(badRequestErrors, errors.New("missing metric name in labels"))
			continue
		} else if duplicateLabel, hasDuplicate := ls.HasDuplicateLabelNames(); hasDuplicate {
			badRequestErrors = errors.Join(badRequestErrors, fmt.Errorf("duplicate label %q in labels", duplicateLabel))
			continue
		}

		// If the metric name is equal to target_info, we use its labels as attributes of the resource
		// Ref: https://opentelemetry.io/docs/specs/otel/compatibility/prometheus_and_openmetrics/#resource-attributes-1
		if metadata.Name == "target_info" {
			rm, hashed := prw.getOrCreateRM(ls, otelMetrics, modifiedResourceMetric)
			attrs := rm.Resource().Attributes()

			// Add the remaining labels as resource attributes
			ls.Range(func(l labels.Label) {
				if l.Name != "job" && l.Name != "instance" && !schema.IsMetadataLabel(l.Name) {
					attrs.PutStr(l.Name, l.Value)
				}
			})

			snapshot := pmetric.NewResourceMetrics()
			attrs.CopyTo(snapshot.Resource().Attributes())
			prw.rmCache.Add(hashed, snapshot)
			continue
		}

		scopeName, scopeVersion := prw.extractScopeInfo(ls)
		metricName := metadata.Name
		if ts.Metadata.UnitRef >= uint32(len(req.Symbols)) {
			badRequestErrors = errors.Join(badRequestErrors, fmt.Errorf("unit ref %d is out of bounds of symbolsTable", ts.Metadata.UnitRef))
			continue
		}

		if ts.Metadata.HelpRef >= uint32(len(req.Symbols)) {
			badRequestErrors = errors.Join(badRequestErrors, fmt.Errorf("help ref %d is out of bounds of symbolsTable", ts.Metadata.HelpRef))
			continue
		}

		unit := req.Symbols[ts.Metadata.UnitRef]
		description := req.Symbols[ts.Metadata.HelpRef]

		// Handle histograms separately due to their complex mixed-schema processing
		if ts.Metadata.Type == writev2.Metadata_METRIC_TYPE_HISTOGRAM ||
			ts.Metadata.Type == writev2.Metadata_METRIC_TYPE_UNSPECIFIED && len(ts.Histograms) > 0 {
			prw.processHistogramTimeSeries(otelMetrics, ls, ts, scopeName, scopeVersion, metricName, unit, description, metricCache, &stats, modifiedResourceMetric, exemplarMap)
			continue
		}

		// Handle regular metrics (gauge, counter, summary)
		rm, _ := prw.getOrCreateRM(ls, otelMetrics, modifiedResourceMetric)

		resourceID := identity.OfResource(rm.Resource())
		metricIdentity := createMetricIdentity(
			resourceID.String(), // Resource identity
			scopeName,           // Scope name
			scopeVersion,        // Scope version
			metricName,          // Metric name
			unit,                // Unit
			ts.Metadata.Type,    // Metric type
		)

		metricKey := metricIdentity.Hash()

		// Find or create scope
		var scope pmetric.ScopeMetrics
		var foundScope bool
		for i := 0; i < rm.ScopeMetrics().Len(); i++ {
			s := rm.ScopeMetrics().At(i)
			if s.Scope().Name() == scopeName && s.Scope().Version() == scopeVersion {
				scope = s
				foundScope = true
				break
			}
		}
		if !foundScope {
			scope = rm.ScopeMetrics().AppendEmpty()
			scope.Scope().SetName(scopeName)
			scope.Scope().SetVersion(scopeVersion)
		}

		// Get or create metric
		metric, exists := metricCache[metricKey]
		if !exists {
			switch ts.Metadata.Type {
			case writev2.Metadata_METRIC_TYPE_GAUGE, writev2.Metadata_METRIC_TYPE_UNSPECIFIED:
				metric = setMetric(scope, metricName, unit, description)
				metric.SetEmptyGauge()
				if ts.Metadata.Type == writev2.Metadata_METRIC_TYPE_UNSPECIFIED {
					metric.Metadata().PutStr(prometheus.MetricMetadataTypeKey, "unknown")
				} else {
					metric.Metadata().PutStr(prometheus.MetricMetadataTypeKey, "gauge")
				}
			case writev2.Metadata_METRIC_TYPE_COUNTER:
				metric = setMetric(scope, metricName, unit, description)
				sum := metric.SetEmptySum()
				sum.SetIsMonotonic(true)
				sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				metric.Metadata().PutStr(prometheus.MetricMetadataTypeKey, "counter")
			case writev2.Metadata_METRIC_TYPE_SUMMARY:
				// Drop summary series as we will not handle them.
				continue
			default:
				badRequestErrors = errors.Join(badRequestErrors, fmt.Errorf("unsupported metric type %q for metric %q", ts.Metadata.Type, metricName))
				continue
			}
			metricCache[metricKey] = metric
		} else if len(metric.Description()) < len(description) {
			// When the new description is longer than the existing one, we should update the metric description.
			// Reference to this behavior: https://opentelemetry.io/docs/specs/otel/metrics/data-model/#opentelemetry-protocol-data-model-producer-recommendations
			metric.SetDescription(description)
		}

		switch ts.Metadata.Type {
		case writev2.Metadata_METRIC_TYPE_GAUGE, writev2.Metadata_METRIC_TYPE_UNSPECIFIED:
			addNumberDatapoints(metric.Gauge().DataPoints(), ls, ts, &stats)
		case writev2.Metadata_METRIC_TYPE_COUNTER:
			addNumberDatapoints(metric.Sum().DataPoints(), ls, ts, &stats)
		case writev2.Metadata_METRIC_TYPE_SUMMARY:
			// Drop summary series as we will not handle them.
			continue
		default:
			badRequestErrors = errors.Join(badRequestErrors, fmt.Errorf("unsupported metric type %q for metric %q", ts.Metadata.Type, metricName))
		}
	}

	return otelMetrics, stats, badRequestErrors
}

// processHistogramTimeSeries handles all histogram processing, including validation and mixed schemas.
func (prw *prometheusRemoteWriteReceiver) processHistogramTimeSeries(
	otelMetrics pmetric.Metrics,
	ls labels.Labels,
	ts *writev2.TimeSeries,
	scopeName, scopeVersion, metricName, unit, description string,
	metricCache map[uint64]pmetric.Metric,
	stats *promremote.WriteResponseStats,
	modifiedRM map[uint64]pmetric.ResourceMetrics,
	exemplarMap map[uint64]pmetric.ExemplarSlice,
) {
	// Drop classic histogram series (those with samples)
	if len(ts.Samples) != 0 {
		prw.settings.Logger.Info("Dropping classic histogram series. Please configure Prometheus to convert classic histograms into Native Histograms Custom Buckets",
			zapcore.Field{Key: "timeseries", Type: zapcore.StringType, String: ls.Get("__name__")})
		return
	}
	attrs := extractAttributes(ls)

	var (
		hashedLabels uint64
		resourceID   identity.Resource
		scope        pmetric.ScopeMetrics
		rm           pmetric.ResourceMetrics
	)

	for i := range ts.Histograms {
		histogram := &ts.Histograms[i]
		if histogram.ResetHint == writev2.Histogram_RESET_HINT_GAUGE {
			continue
		}

		var histogramType string

		// Determine histogram type based on schema
		// See https://prometheus.io/docs/specs/native_histograms/#schema
		switch histogram.Schema {
		case -53:
			histogramType = "nhcb"
		case -4, -3, -2, -1, 0, 1, 2, 3, 4, 5, 6, 7, 8:
			histogramType = "exponential"
		default:
			// Skip invalid schema
			continue
		}
		// Create resource if needed (only for the first valid histogram)
		if hashedLabels == 0 {
			rm, _ = prw.getOrCreateRM(ls, otelMetrics, modifiedRM)
			resourceID = identity.OfResource(rm.Resource())
		}

		// Find or create scope (search each time since different histograms might need different scopes)
		var foundScope bool
		for i := 0; i < rm.ScopeMetrics().Len(); i++ {
			s := rm.ScopeMetrics().At(i)
			if s.Scope().Name() == scopeName && s.Scope().Version() == scopeVersion {
				scope = s
				foundScope = true
				break
			}
		}
		if !foundScope {
			scope = rm.ScopeMetrics().AppendEmpty()
			scope.Scope().SetName(scopeName)
			scope.Scope().SetVersion(scopeVersion)
		}

		metricID := fmt.Sprintf("%s:%s:%s:%s:%s:%s:%s",
			resourceID.String(),
			scopeName,
			scopeVersion,
			metricName,
			unit,
			fmt.Sprintf("%d", ts.Metadata.Type),
			histogramType,
		)
		metricIDHash := xxhash.Sum64String(metricID)

		histMetric, exists := metricCache[metricIDHash]
		if !exists {
			histMetric = setMetric(scope, metricName, unit, description)
			if histogramType == "nhcb" {
				hist := histMetric.SetEmptyHistogram()
				hist.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
			} else {
				hist := histMetric.SetEmptyExponentialHistogram()
				hist.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
			}
			metricCache[metricIDHash] = histMetric
			switch ts.Metadata.Type {
			case writev2.Metadata_METRIC_TYPE_HISTOGRAM:
				histMetric.Metadata().PutStr(prometheus.MetricMetadataTypeKey, "histogram")
			case writev2.Metadata_METRIC_TYPE_UNSPECIFIED:
				histMetric.Metadata().PutStr(prometheus.MetricMetadataTypeKey, "unknown")
			default:
				// This default case should not be reached as this function is only called when:
				// 1. ts.Metadata.Type == METRIC_TYPE_HISTOGRAM, or
				// 2. ts.Metadata.Type == METRIC_TYPE_UNSPECIFIED && len(ts.Histograms) > 0
			}
		} else if len(histMetric.Description()) < len(description) {
			// When the new description is longer than the existing one, we should update the metric description.
			// Reference to this behavior: https://opentelemetry.io/docs/specs/otel/metrics/data-model/#opentelemetry-protocol-data-model-producer-recommendations
			histMetric.SetDescription(description)
		}
		// all the exemplars for a given histogram are attached to first data point.
		var exemplarSlice pmetric.ExemplarSlice
		// Process the individual histogram
		if histogramType == "nhcb" {
			prw.addNHCBDatapoint(histMetric.Histogram().DataPoints(), histogram, attrs, stats)
			if histMetric.Histogram().DataPoints().Len() > 0 {
				exemplarSlice = histMetric.Histogram().DataPoints().At(0).Exemplars()
			}
		} else {
			prw.addExponentialHistogramDatapoint(histMetric.ExponentialHistogram().DataPoints(), histogram, attrs, ls, stats)
			if histMetric.ExponentialHistogram().DataPoints().Len() > 0 {
				exemplarSlice = histMetric.ExponentialHistogram().DataPoints().At(0).Exemplars()
			}
		}

		key := exemplarKey{
			ScopeName:    scopeName,
			ScopeVersion: scopeVersion,
			MetricName:   metricName,
			MetricType:   ts.Metadata.Type,
		}

		if ex, ok := exemplarMap[key.hash()]; ok && ex.Len() > 0 {
			ex.CopyTo(exemplarSlice)
		}
	}
}

// setMetric append a new empty metric and assign the name, unit and description to it.
func setMetric(scope pmetric.ScopeMetrics, metricName, unit, description string) pmetric.Metric {
	metric := scope.Metrics().AppendEmpty()
	metric.SetName(metricName)
	metric.SetUnit(unit)
	metric.SetDescription(description)
	return metric
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

// addNumberDatapoints adds the labels to the datapoints attributes.
func addNumberDatapoints(datapoints pmetric.NumberDataPointSlice, ls labels.Labels, ts *writev2.TimeSeries, stats *promremote.WriteResponseStats) {
	// Add samples from the timeseries
	attrs := extractAttributes(ls)
	for i := range ts.Samples {
		sample := &ts.Samples[i]
		dp := datapoints.AppendEmpty()
		dp.SetStartTimestamp(pcommon.Timestamp(sample.StartTimestamp * int64(time.Millisecond)))
		// Set timestamp in nanoseconds (Prometheus uses milliseconds)
		dp.SetTimestamp(pcommon.Timestamp(sample.Timestamp * int64(time.Millisecond)))
		dp.SetDoubleValue(sample.Value)

		attributes := dp.Attributes()
		attrs.CopyTo(attributes)
	}
	stats.Samples += len(ts.Samples)
}

func (prw *prometheusRemoteWriteReceiver) addExponentialHistogramDatapoint(datapoints pmetric.ExponentialHistogramDataPointSlice, histogram *writev2.Histogram, attrs pcommon.Map, ls labels.Labels, stats *promremote.WriteResponseStats) {
	// Drop Native Histogram with negative counts
	if hasNegativeCounts(histogram) {
		prw.settings.Logger.Info("Dropping Native Histogram series with negative counts",
			zapcore.Field{Key: "timeseries", Type: zapcore.StringType, String: ls.Get("__name__")})
		return
	}

	dp := datapoints.AppendEmpty()
	dp.SetStartTimestamp(pcommon.Timestamp(histogram.StartTimestamp * int64(time.Millisecond)))
	dp.SetTimestamp(pcommon.Timestamp(histogram.Timestamp * int64(time.Millisecond)))
	dp.SetScale(histogram.Schema)
	dp.SetZeroThreshold(histogram.ZeroThreshold)

	// Set count and sum using common helper
	setCountAndSum(histogram, dp)

	// The difference between float and integer histograms is that float histograms are stored as absolute counts
	// while integer histograms are stored as deltas.
	if histogram.IsFloatHistogram() {
		// Float histograms
		zeroCountFloat := histogram.GetZeroCountFloat()
		dp.SetZeroCount(uint64(zeroCountFloat))

		if len(histogram.PositiveSpans) > 0 {
			dp.Positive().SetOffset(histogram.PositiveSpans[0].Offset - 1) // -1 because OTEL offset are for the lower bound, not the upper bound
			convertAbsoluteBuckets(histogram.PositiveSpans, histogram.PositiveCounts, dp.Positive().BucketCounts())
		}
		if len(histogram.NegativeSpans) > 0 {
			dp.Negative().SetOffset(histogram.NegativeSpans[0].Offset - 1) // -1 because OTEL offset are for the lower bound, not the upper bound
			convertAbsoluteBuckets(histogram.NegativeSpans, histogram.NegativeCounts, dp.Negative().BucketCounts())
		}
	} else {
		// Integer histograms
		zeroCountInt := histogram.GetZeroCountInt()
		dp.SetZeroCount(zeroCountInt)

		if len(histogram.PositiveSpans) > 0 {
			dp.Positive().SetOffset(histogram.PositiveSpans[0].Offset - 1) // -1 because OTEL offset are for the lower bound, not the upper bound
			convertDeltaBuckets(histogram.PositiveSpans, histogram.PositiveDeltas, dp.Positive().BucketCounts())
		}
		if len(histogram.NegativeSpans) > 0 {
			dp.Negative().SetOffset(histogram.NegativeSpans[0].Offset - 1) // -1 because OTEL offset are for the lower bound, not the upper bound
			convertDeltaBuckets(histogram.NegativeSpans, histogram.NegativeDeltas, dp.Negative().BucketCounts())
		}
	}

	attrs.CopyTo(dp.Attributes())
	stats.Histograms++
}

// hasNegativeCounts checks if a histogram has any negative counts
func hasNegativeCounts(histogram *writev2.Histogram) bool {
	if histogram.IsFloatHistogram() {
		// Check overall count
		if histogram.GetCountFloat() < 0 {
			return true
		}

		// Check zero count
		if histogram.GetZeroCountFloat() < 0 {
			return true
		}

		// Check positive bucket counts
		for _, count := range histogram.PositiveCounts {
			if count < 0 {
				return true
			}
		}

		// Check negative bucket counts
		for _, count := range histogram.NegativeCounts {
			if count < 0 {
				return true
			}
		}
	} else {
		// Integer histograms
		var absolute int64
		for _, delta := range histogram.NegativeDeltas {
			absolute += delta
			if absolute < 0 {
				return true
			}
		}

		absolute = 0
		for _, delta := range histogram.PositiveDeltas {
			absolute += delta
			if absolute < 0 {
				return true
			}
		}
	}

	return false
}

// convertDeltaBuckets converts Prometheus native histogram spans and deltas to OpenTelemetry bucket counts
// For integer buckets, the values are deltas between the buckets. i.e a bucket list of 1,2,-2 would correspond to a bucket count of 1,3,1
func convertDeltaBuckets(spans []writev2.BucketSpan, deltas []int64, buckets pcommon.UInt64Slice) {
	// The total capacity is the sum of the deltas and the offsets of the spans.
	totalCapacity := len(deltas)
	for _, span := range spans {
		totalCapacity += int(span.Offset)
	}
	buckets.EnsureCapacity(totalCapacity)

	bucketIdx := 0
	bucketCount := int64(0)
	for spanIdx, span := range spans {
		if spanIdx > 0 {
			for i := int32(0); i < span.Offset; i++ {
				buckets.Append(uint64(0))
			}
		}
		for i := uint32(0); i < span.Length; i++ {
			bucketCount += deltas[bucketIdx]
			bucketIdx++
			buckets.Append(uint64(bucketCount))
		}
	}
}

// convertAbsoluteBuckets converts Prometheus native histogram spans and absolute counts to OpenTelemetry bucket counts
// For float buckets, the values are absolute counts, and must be 0 or positive.
func convertAbsoluteBuckets(spans []writev2.BucketSpan, counts []float64, buckets pcommon.UInt64Slice) {
	// The total capacity is the sum of the counts and the offsets of the spans.
	totalCapacity := len(counts)
	for _, span := range spans {
		totalCapacity += int(span.Offset)
	}
	buckets.EnsureCapacity(totalCapacity)

	bucketIdx := 0
	for spanIdx, span := range spans {
		if spanIdx > 0 {
			for i := int32(0); i < span.Offset; i++ {
				buckets.Append(uint64(0))
			}
		}
		for i := uint32(0); i < span.Length; i++ {
			buckets.Append(uint64(counts[bucketIdx]))
			bucketIdx++
		}
	}
}

// extractAttributes return all attributes different from job, instance, metric name and scope name/version
func extractAttributes(ls labels.Labels) pcommon.Map {
	attrs := pcommon.NewMap()
	// job, instance and metric name will always become labels
	attrs.EnsureCapacity(ls.Len() - 3)
	ls.Range(func(l labels.Label) {
		if l.Name != "instance" && l.Name != "job" && // Become resource attributes
			l.Name != model.MetricNameLabel && // Becomes metric name
			l.Name != "otel_scope_name" && l.Name != "otel_scope_version" { // Becomes scope name and version
			attrs.PutStr(l.Name, l.Value)
		}
	})
	return attrs
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

// addNHCBDatapoint converts a single Native Histogram Custom Buckets (NHCB) to OpenTelemetry histogram datapoints
func (*prometheusRemoteWriteReceiver) addNHCBDatapoint(datapoints pmetric.HistogramDataPointSlice, histogram *writev2.Histogram, attrs pcommon.Map, stats *promremote.WriteResponseStats) {
	if len(histogram.CustomValues) == 0 {
		return
	}

	dp := datapoints.AppendEmpty()
	dp.SetStartTimestamp(pcommon.Timestamp(histogram.StartTimestamp * int64(time.Millisecond)))
	dp.SetTimestamp(pcommon.Timestamp(histogram.Timestamp * int64(time.Millisecond)))

	if value.IsStaleNaN(histogram.Sum) {
		dp.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
	} else {
		setCountAndSum(histogram, dp)
	}

	dp.ExplicitBounds().FromRaw(histogram.CustomValues)
	bucketCounts := convertNHCBBuckets(histogram)
	dp.BucketCounts().FromRaw(bucketCounts)

	attrs.CopyTo(dp.Attributes())
	stats.Histograms++
}

// convertNHCBBuckets converts NHCB bucket data to OpenTelemetry bucket counts
func convertNHCBBuckets(histogram *writev2.Histogram) []uint64 {
	// For NHCB, we need numExplicitBounds + 1 buckets (including the final +inf bucket)
	bucketCounts := make([]uint64, len(histogram.CustomValues)+1)

	// NHCB uses the positive bucket list and spans for all buckets
	if len(histogram.PositiveSpans) == 0 {
		return bucketCounts
	}

	if histogram.IsFloatHistogram() {
		// Float histograms: values are absolute counts
		bucketIdx := 0
		for _, span := range histogram.PositiveSpans {
			// Skip empty buckets based on offset
			bucketIdx += int(span.Offset)

			// Fill buckets for this span
			for i := uint32(0); i < span.Length && bucketIdx < len(bucketCounts) && i < uint32(len(histogram.PositiveCounts)); i++ {
				if bucketIdx >= 0 && bucketIdx < len(bucketCounts) {
					bucketCounts[bucketIdx] = uint64(histogram.PositiveCounts[i])
				}
				bucketIdx++
			}
		}
	} else {
		// Integer histograms: values are deltas between buckets
		bucketIdx := 0
		bucketCount := int64(0)
		deltaIdx := 0

		for _, span := range histogram.PositiveSpans {
			// Skip empty buckets based on offset
			bucketIdx += int(span.Offset)

			// Fill buckets for this span
			for i := uint32(0); i < span.Length && bucketIdx < len(bucketCounts) && deltaIdx < len(histogram.PositiveDeltas); i++ {
				bucketCount += histogram.PositiveDeltas[deltaIdx]
				deltaIdx++

				if bucketIdx >= 0 && bucketIdx < len(bucketCounts) {
					bucketCounts[bucketIdx] = uint64(bucketCount)
				}
				bucketIdx++
			}
		}
	}

	return bucketCounts
}

// setCountAndSum sets count and sum for histogram datapoints (common interface)
type countSumSetter interface {
	SetSum(float64)
	SetCount(uint64)
}

func setCountAndSum(histogram *writev2.Histogram, dp countSumSetter) {
	dp.SetSum(histogram.Sum)

	if histogram.IsFloatHistogram() {
		countFloat := histogram.GetCountFloat()
		dp.SetCount(uint64(countFloat))
	} else {
		countInt := histogram.GetCountInt()
		dp.SetCount(countInt)
	}
}
