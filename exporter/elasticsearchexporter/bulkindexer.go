// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/elastic/go-docappender/v2"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/logging"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/metadata"
)

type bulkIndexer interface {
	// StartSession starts a new bulk indexing session.
	StartSession(context.Context) bulkIndexerSession

	// Close closes the bulk indexer, ending any in-progress
	// sessions and stopping any background processing.
	Close(ctx context.Context) error
}

type bulkIndexerSession interface {
	// Add adds a document to the bulk indexing session.
	Add(ctx context.Context, index, docID, pipeline string, document io.WriterTo, dynamicTemplates map[string]string, action string) error

	// End must be called on the session object once it is no longer
	// needed, in order to release any associated resources.
	//
	// Note that ending the session does _not_ implicitly flush
	// documents. Call Flush before calling End as needed.
	//
	// Calling other methods (including End) after End may panic.
	End()

	// Flush flushes any documents added to the bulk indexing session.
	Flush(context.Context) error
}

const defaultMaxRetries = 2

const (
	// errorHintKnownIssues is the hint message for errors that are documented in the Known issues section.
	errorHintKnownIssues = "check the \"Known issues\" section of Elasticsearch Exporter docs"
	// errorHintOTelMappingMode is the hint message for illegal_argument_exception when using OTel mapping mode with incompatible Elasticsearch versions.
	errorHintOTelMappingMode = "OTel mapping mode requires Elasticsearch 8.12+ (see Known issues in README)"
)

// otelDatasetSuffixRegex matches the .otel-{namespace} suffix pattern in OTel mapping mode indices.
// Pattern: {signal}-{dataset}.otel-{namespace}
var otelDatasetSuffixRegex = regexp.MustCompile(`^[^-]+?-[^-]+?\.otel-`)

func newBulkIndexer(
	client esapi.Transport,
	config *Config,
	requireDataStream bool,
	tb *metadata.TelemetryBuilder,
	logger *zap.Logger,
) bulkIndexer {
	return newSyncBulkIndexer(client, config, requireDataStream, tb, logger)
}

func bulkIndexerConfig(client esapi.Transport, config *Config, requireDataStream bool, logger *zap.Logger) docappender.BulkIndexerConfig {
	var maxDocRetries int
	if config.Retry.Enabled {
		maxDocRetries = defaultMaxRetries
		if config.Retry.MaxRetries != 0 {
			maxDocRetries = config.Retry.MaxRetries
		}
	}
	var compressionLevel int
	if config.Compression == configcompression.TypeGzip {
		compressionLevel = int(config.CompressionParams.Level)
	}
	return docappender.BulkIndexerConfig{
		Client:                  client,
		MaxDocumentRetries:      maxDocRetries,
		Pipeline:                config.Pipeline,
		RetryOnDocumentStatus:   config.Retry.RetryOnStatus,
		RequireDataStream:       requireDataStream,
		CompressionLevel:        compressionLevel,
		PopulateFailedDocsInput: config.LogFailedDocsInput,
		IncludeSourceOnError:    bulkIndexerIncludeSourceOnError(config.IncludeSourceOnError),
		QueryParams:             getQueryParamsFromEndpoint(config, logger),
	}
}

func getQueryParamsFromEndpoint(config *Config, logger *zap.Logger) (queryParams map[string][]string) {
	endpoints, _ := config.endpoints()

	if len(endpoints) != 0 {
		// we check the query params set on the first endpoint only
		// this is enough to replicate to all requests
		parsedURL, err := url.Parse(endpoints[0])
		if err != nil {
			logger.Warn("Failed to parse URL from endpoint", zap.Error(err))
		}

		rawQuery := parsedURL.RawQuery
		queryParams, err = url.ParseQuery(rawQuery)
		if err != nil {
			logger.Warn("Failed to parse query parameters from endpoint", zap.Error(err))
		}
		return queryParams
	}
	return nil
}

func bulkIndexerIncludeSourceOnError(includeSourceOnError *bool) docappender.Value {
	if includeSourceOnError == nil {
		return docappender.Unset
	}
	if *includeSourceOnError {
		return docappender.True
	}
	return docappender.False
}

func newSyncBulkIndexer(
	client esapi.Transport,
	config *Config,
	requireDataStream bool,
	tb *metadata.TelemetryBuilder,
	logger *zap.Logger,
) *syncBulkIndexer {
	var maxFlushBytes int64
	if config.QueueBatchConfig.HasValue() && config.QueueBatchConfig.Get().Batch.HasValue() {
		batch := config.QueueBatchConfig.Get().Batch.Get()
		if batch.Sizer == exporterhelper.RequestSizerTypeBytes {
			maxFlushBytes = batch.MaxSize
		}
	}
	return &syncBulkIndexer{
		config:                bulkIndexerConfig(client, config, requireDataStream, logger),
		maxFlushBytes:         maxFlushBytes,
		flushTimeout:          config.Timeout,
		retryConfig:           config.Retry,
		metadataKeys:          config.MetadataKeys,
		telemetryBuilder:      tb,
		logger:                logger,
		failedDocsInputLogger: newFailedDocsInputLogger(logger, config),
	}
}

type syncBulkIndexer struct {
	config                docappender.BulkIndexerConfig
	maxFlushBytes         int64
	flushTimeout          time.Duration
	retryConfig           RetrySettings
	metadataKeys          []string
	telemetryBuilder      *metadata.TelemetryBuilder
	logger                *zap.Logger
	failedDocsInputLogger *zap.Logger
}

// StartSession creates a new docappender.BulkIndexer, and wraps
// it with a syncBulkIndexerSession.
func (s *syncBulkIndexer) StartSession(context.Context) bulkIndexerSession {
	bi, err := docappender.NewBulkIndexer(s.config)
	if err != nil {
		// This should never happen in practice:
		// NewBulkIndexer should only fail if the
		// config is invalid, and we expect it to
		// always be valid at this point.
		return errBulkIndexerSession{err: err}
	}
	return &syncBulkIndexerSession{s: s, bi: bi}
}

// Close is a no-op.
func (*syncBulkIndexer) Close(context.Context) error {
	return nil
}

type syncBulkIndexerSession struct {
	s  *syncBulkIndexer
	bi *docappender.BulkIndexer
}

// Add adds an item to the sync bulk indexer session.
func (s *syncBulkIndexerSession) Add(ctx context.Context, index, docID, pipeline string, document io.WriterTo, dynamicTemplates map[string]string, action string) error {
	doc := docappender.BulkIndexerItem{
		Index:            index,
		Body:             document,
		DocumentID:       docID,
		DynamicTemplates: dynamicTemplates,
		Action:           action,
		Pipeline:         pipeline,
	}
	err := s.bi.Add(doc)
	if err != nil {
		return err
	}
	s.s.telemetryBuilder.ElasticsearchDocsReceived.Add(
		ctx, 1,
		metric.WithAttributeSet(attribute.NewSet(
			getAttributesFromMetadataKeys(ctx, s.s.metadataKeys)...),
		),
	)
	// sending_queue operates on flush sizes based on pdata model whereas bulk
	// indexers operate on ndjson. Force a flush if the ndjson size is too large.
	// when the uncompressed length exceeds the configured max flush size.
	if s.s.maxFlushBytes > 0 && int64(s.bi.UncompressedLen()) >= s.s.maxFlushBytes {
		return s.Flush(ctx)
	}
	return nil
}

// End is a no-op.
func (*syncBulkIndexerSession) End() {
	// TODO acquire docappender.BulkIndexer from pool in StartSession, release here
}

// Flush flushes documents added to the bulk indexer session.
func (s *syncBulkIndexerSession) Flush(ctx context.Context) error {
	var retryBackoff func(int) time.Duration
	for attempts := 0; ; attempts++ {
		if err := flushBulkIndexer(
			ctx,
			s.bi,
			s.s.flushTimeout,
			s.s.metadataKeys,
			s.s.telemetryBuilder,
			s.s.logger,
			s.s.failedDocsInputLogger,
		); err != nil {
			return err
		}
		if s.bi.Items() == 0 {
			// No documents in buffer waiting for per-document retry, exit retry loop.
			return nil
		}
		if retryBackoff == nil {
			retryBackoff = createElasticsearchBackoffFunc(&s.s.retryConfig)
			if retryBackoff == nil {
				// BUG: This should never happen in practice.
				// When retry is disabled / document level retry limit is reached,
				// documents should go into FailedDocs instead of indexer buffer.
				return errors.New("bulk indexer contains documents pending retry but retry is disabled")
			}
		}
		backoff := retryBackoff(attempts + 1) // TODO: use exporterhelper retry_sender
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func flushBulkIndexer(
	ctx context.Context,
	bi *docappender.BulkIndexer,
	timeout time.Duration,
	tMetaKeys []string,
	tb *metadata.TelemetryBuilder,
	logger *zap.Logger,
	failedDocsInputLogger *zap.Logger,
) error {
	itemsCount := bi.Items()
	if itemsCount == 0 {
		return nil
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	startTime := time.Now()
	stat, err := bi.Flush(ctx)
	latency := time.Since(startTime).Seconds()
	defaultMetaAttrs := getAttributesFromMetadataKeys(ctx, tMetaKeys)
	defaultAttrsSet := attribute.NewSet(defaultMetaAttrs...)
	if flushed := bi.BytesFlushed(); flushed > 0 {
		tb.ElasticsearchFlushedBytes.Add(ctx, int64(flushed), metric.WithAttributeSet(defaultAttrsSet))
	}
	if flushed := bi.BytesUncompressedFlushed(); flushed > 0 {
		tb.ElasticsearchFlushedUncompressedBytes.Add(
			ctx, int64(flushed), metric.WithAttributeSet(defaultAttrsSet),
		)
	}

	var fields []zap.Field
	// append metadata attributes to error log fields
	for _, kv := range defaultMetaAttrs {
		switch kv.Value.Type() {
		case attribute.STRINGSLICE:
			fields = append(fields, zap.Strings(string(kv.Key), kv.Value.AsStringSlice()))
		default:
			// For other types, convert to string
			fields = append(fields, zap.String(string(kv.Key), kv.Value.AsString()))
		}
	}
	if err != nil {
		logger.Error("bulk indexer flush error", append(fields, zap.Error(err))...)
		var bulkFailedErr docappender.ErrorFlushFailed
		switch {
		case errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded):
			attrSet := metric.WithAttributeSet(attribute.NewSet(
				append([]attribute.KeyValue{attribute.String("outcome", "timeout")}, defaultMetaAttrs...)...,
			))
			tb.ElasticsearchDocsProcessed.Add(ctx, int64(itemsCount), attrSet)
			tb.ElasticsearchBulkRequestsCount.Add(ctx, int64(1), attrSet)
			tb.ElasticsearchBulkRequestsLatency.Record(ctx, latency, attrSet)
		case errors.As(err, &bulkFailedErr):
			var outcome string
			code := bulkFailedErr.StatusCode()
			switch {
			case code == http.StatusTooManyRequests:
				outcome = "too_many"
			case code >= 500:
				outcome = "failed_server"
			case code >= 400:
				outcome = "failed_client"
			}
			attrSet := metric.WithAttributeSet(attribute.NewSet(
				append([]attribute.KeyValue{
					conventions.HTTPResponseStatusCode(code),
					attribute.String("outcome", outcome),
				}, defaultMetaAttrs...)...,
			))
			tb.ElasticsearchDocsProcessed.Add(ctx, int64(itemsCount), attrSet)
			tb.ElasticsearchBulkRequestsCount.Add(ctx, int64(1), attrSet)
			tb.ElasticsearchBulkRequestsLatency.Record(ctx, latency, attrSet)
		default:
			attrSet := metric.WithAttributeSet(attribute.NewSet(
				append([]attribute.KeyValue{
					attribute.String("outcome", "internal_server_error"),
					conventions.HTTPResponseStatusCode(http.StatusInternalServerError),
				}, defaultMetaAttrs...)...,
			))
			tb.ElasticsearchDocsProcessed.Add(ctx, int64(itemsCount), attrSet)
			tb.ElasticsearchBulkRequestsCount.Add(ctx, int64(1), attrSet)
			tb.ElasticsearchBulkRequestsLatency.Record(ctx, latency, attrSet)
		}
	} else {
		// Record a successful completed bulk request
		successAttrSet := metric.WithAttributeSet(attribute.NewSet(
			append([]attribute.KeyValue{
				attribute.String("outcome", "success"),
				conventions.HTTPResponseStatusCode(http.StatusOK),
			}, defaultMetaAttrs...)...,
		))

		tb.ElasticsearchBulkRequestsCount.Add(ctx, int64(1), successAttrSet)
		tb.ElasticsearchBulkRequestsLatency.Record(ctx, latency, successAttrSet)
	}

	for _, resp := range stat.FailedDocs {
		// Collect telemetry
		var outcome string
		switch {
		case resp.Status == http.StatusTooManyRequests:
			outcome = "too_many"
		case resp.Status >= 500:
			outcome = "failed_server"
		case resp.Status >= 400:
			outcome = "failed_client"
		}

		tb.ElasticsearchDocsProcessed.Add(
			ctx,
			int64(1),
			metric.WithAttributeSet(attribute.NewSet(
				append([]attribute.KeyValue{
					attribute.String("outcome", outcome),
					attribute.String("error.type", resp.Error.Type),
				}, defaultMetaAttrs...)...,
			)),
		)

		if resp.Error.Type == "version_conflict_engine_exception" &&
			(strings.HasPrefix(resp.Index, ".profiling-stackframes-") ||
				strings.HasPrefix(resp.Index, ".profiling-stacktraces-")) {
			// For the Profiling indices .profiling-[stacktraces|stackframes]- the
			// rejection of duplicates are expected from Elasticsearch. So we do not want
			// to log these here.
			continue
		}

		// Log failed docs
		fields = append(fields,
			zap.String("index", resp.Index),
			zap.String("error.type", resp.Error.Type),
			zap.String("error.reason", resp.Error.Reason),
		)

		if hint := getErrorHint(resp.Index, resp.Error.Type); hint != "" {
			fields = append(fields, zap.String("hint", hint))
		}
		logger.Error("failed to index document", fields...)

		if resp.Input != "" {
			fields = append(fields, zap.String("input", resp.Input))
		}
		failedDocsInputLogger.Debug("failed to index document; input may contain sensitive data", fields...)
	}
	if stat.Indexed > 0 {
		tb.ElasticsearchDocsProcessed.Add(
			ctx,
			stat.Indexed,
			metric.WithAttributeSet(attribute.NewSet(
				append([]attribute.KeyValue{
					attribute.String("outcome", "success"),
				}, defaultMetaAttrs...)...,
			)),
		)
	}
	if stat.FailureStoreDocs.Used > 0 {
		tb.ElasticsearchDocsProcessed.Add(
			ctx,
			stat.FailureStoreDocs.Used,
			metric.WithAttributeSet(attribute.NewSet(
				append([]attribute.KeyValue{
					attribute.String("outcome", "failure_store"),
					attribute.String("failure_store", string(docappender.FailureStoreStatusUsed)),
				}, defaultMetaAttrs...)...,
			)),
		)
	}
	if stat.FailureStoreDocs.Failed > 0 {
		tb.ElasticsearchDocsProcessed.Add(
			ctx,
			stat.FailureStoreDocs.Failed,
			metric.WithAttributeSet(attribute.NewSet(
				append([]attribute.KeyValue{
					attribute.String("outcome", "failure_store"),
					attribute.String("failure_store", string(docappender.FailureStoreStatusFailed)),
				}, defaultMetaAttrs...)...,
			)),
		)
	}
	if stat.FailureStoreDocs.NotEnabled > 0 {
		tb.ElasticsearchDocsProcessed.Add(
			ctx,
			stat.FailureStoreDocs.NotEnabled,
			metric.WithAttributeSet(attribute.NewSet(
				append([]attribute.KeyValue{
					attribute.String("outcome", "failure_store"),
					attribute.String("failure_store", string(docappender.FailureStoreStatusNotEnabled)),
				}, defaultMetaAttrs...)...,
			)),
		)
	}
	if stat.RetriedDocs > 0 {
		tb.ElasticsearchDocsRetried.Add(ctx, stat.RetriedDocs, metric.WithAttributeSet(defaultAttrsSet))
	}
	return err
}

func getAttributesFromMetadataKeys(ctx context.Context, keys []string) []attribute.KeyValue {
	clientInfo := client.FromContext(ctx)
	attrs := make([]attribute.KeyValue, 0, len(keys))
	for _, k := range keys {
		if values := clientInfo.Metadata.Get(k); len(values) != 0 {
			attrs = append(attrs, attribute.StringSlice(k, values))
		}
	}
	return attrs
}

func getErrorHint(index, errorType string) string {
	if strings.HasPrefix(index, ".ds-metrics-") && errorType == "version_conflict_engine_exception" {
		return errorHintKnownIssues
	}
	// Detect illegal_argument_exception related to require_data_stream when using OTel mapping mode
	// with Elasticsearch < 8.12. OTel mapping mode indices contain ".otel-" as a dataset suffix.
	if errorType == "illegal_argument_exception" && otelDatasetSuffixRegex.MatchString(index) {
		return errorHintOTelMappingMode
	}
	return ""
}

func newFailedDocsInputLogger(logger *zap.Logger, config *Config) *zap.Logger {
	if !config.LogFailedDocsInput {
		return zap.NewNop()
	}
	return logger.WithOptions(logging.WithRateLimit(config.LogFailedDocsInputRateLimit))
}

type bulkIndexers struct {
	// wg tracks active sessions
	wg sync.WaitGroup

	// NOTE(axw) we have removed async bulk indexer and there should be
	// no reason for having one per mode or for different document types.
	// Instead, the caller can create separate sessions as needed, and we
	// can either have one for required_data_stream=true and one for false,
	// or callers can set this per document.

	modes                [NumMappingModes]bulkIndexer
	profilingEvents      bulkIndexer // For profiling-events-*
	profilingStackTraces bulkIndexer // For profiling-stacktraces
	profilingStackFrames bulkIndexer // For profiling-stackframes
	profilingExecutables bulkIndexer // For profiling-executables

	telemetryBuilder *metadata.TelemetryBuilder
}

func (b *bulkIndexers) start(
	ctx context.Context,
	cfg *Config,
	set exporter.Settings,
	host component.Host,
	allowedMappingModes map[string]MappingMode,
) error {
	userAgent := fmt.Sprintf(
		"%s/%s (%s/%s)",
		set.BuildInfo.Description,
		set.BuildInfo.Version,
		runtime.GOOS,
		runtime.GOARCH,
	)

	esClient, err := newElasticsearchClient(ctx, cfg, host, set.TelemetrySettings, userAgent)
	if err != nil {
		return err
	}

	for _, mode := range allowedMappingModes {
		bi := newBulkIndexer(esClient, cfg, mode == MappingOTel, b.telemetryBuilder, set.Logger)
		b.modes[mode] = &wgTrackingBulkIndexer{bulkIndexer: bi, wg: &b.wg}
	}

	profilingEvents := newBulkIndexer(esClient, cfg, true, b.telemetryBuilder, set.Logger)
	b.profilingEvents = &wgTrackingBulkIndexer{bulkIndexer: profilingEvents, wg: &b.wg}

	profilingStackTraces := newBulkIndexer(esClient, cfg, false, b.telemetryBuilder, set.Logger)
	b.profilingStackTraces = &wgTrackingBulkIndexer{bulkIndexer: profilingStackTraces, wg: &b.wg}

	profilingStackFrames := newBulkIndexer(esClient, cfg, false, b.telemetryBuilder, set.Logger)
	b.profilingStackFrames = &wgTrackingBulkIndexer{bulkIndexer: profilingStackFrames, wg: &b.wg}

	profilingExecutables := newBulkIndexer(esClient, cfg, false, b.telemetryBuilder, set.Logger)
	b.profilingExecutables = &wgTrackingBulkIndexer{bulkIndexer: profilingExecutables, wg: &b.wg}
	return nil
}

func (b *bulkIndexers) shutdown(ctx context.Context) error {
	for _, bi := range b.modes {
		if bi == nil {
			continue
		}
		if err := bi.Close(ctx); err != nil {
			return err
		}
	}
	if b.profilingEvents != nil {
		if err := b.profilingEvents.Close(ctx); err != nil {
			return err
		}
	}
	if b.profilingStackTraces != nil {
		if err := b.profilingStackTraces.Close(ctx); err != nil {
			return err
		}
	}
	if b.profilingStackFrames != nil {
		if err := b.profilingStackFrames.Close(ctx); err != nil {
			return err
		}
	}
	if b.profilingExecutables != nil {
		if err := b.profilingExecutables.Close(ctx); err != nil {
			return err
		}
	}

	doneCh := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(doneCh)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-doneCh:
	}
	return nil
}

type wgTrackingBulkIndexer struct {
	bulkIndexer
	wg *sync.WaitGroup
}

func (w *wgTrackingBulkIndexer) StartSession(ctx context.Context) bulkIndexerSession {
	w.wg.Add(1)
	session := w.bulkIndexer.StartSession(ctx)
	return &wgTrackingBulkIndexerSession{bulkIndexerSession: session, wg: w.wg}
}

type wgTrackingBulkIndexerSession struct {
	bulkIndexerSession
	wg *sync.WaitGroup
}

func (w *wgTrackingBulkIndexerSession) End() {
	defer w.wg.Done()
	w.bulkIndexerSession.End()
}

type errBulkIndexerSession struct {
	err error
}

func (s errBulkIndexerSession) Add(context.Context, string, string, string, io.WriterTo, map[string]string, string) error {
	return fmt.Errorf("creating bulk indexer session failed, cannot add item: %w", s.err)
}

func (errBulkIndexerSession) End() {}

func (s errBulkIndexerSession) Flush(context.Context) error {
	return fmt.Errorf("creating bulk indexer session failed, cannot flush: %w", s.err)
}
