// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/objmodel"
)

type elasticsearchExporter struct {
	component.TelemetrySettings
	userAgent string

	config         *Config
	index          string
	logstashFormat LogstashFormatSettings
	dynamicIndex   bool
	model          mappingModel
	otel           bool

	wg          sync.WaitGroup // active sessions
	bulkIndexer bulkIndexer
}

func newExporter(
	cfg *Config,
	set exporter.Settings,
	index string,
	dynamicIndex bool,
) (*elasticsearchExporter, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	model := &encodeModel{
		dedot: cfg.Mapping.Dedot,
		mode:  cfg.MappingMode(),
	}

	otel := model.mode == MappingOTel

	userAgent := fmt.Sprintf(
		"%s/%s (%s/%s)",
		set.BuildInfo.Description,
		set.BuildInfo.Version,
		runtime.GOOS,
		runtime.GOARCH,
	)

	return &elasticsearchExporter{
		TelemetrySettings: set.TelemetrySettings,
		userAgent:         userAgent,

		config:         cfg,
		index:          index,
		dynamicIndex:   dynamicIndex,
		model:          model,
		logstashFormat: cfg.LogstashFormat,
		otel:           otel,
	}, nil
}

func (e *elasticsearchExporter) Start(ctx context.Context, host component.Host) error {
	client, err := newElasticsearchClient(ctx, e.config, host, e.TelemetrySettings, e.userAgent)
	if err != nil {
		return err
	}
	bulkIndexer, err := newBulkIndexer(e.Logger, client, e.config)
	if err != nil {
		return err
	}
	e.bulkIndexer = bulkIndexer
	return nil
}

func (e *elasticsearchExporter) Shutdown(ctx context.Context) error {
	if e.bulkIndexer != nil {
		if err := e.bulkIndexer.Close(ctx); err != nil {
			return err
		}
	}

	doneCh := make(chan struct{})
	go func() {
		e.wg.Wait()
		close(doneCh)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-doneCh:
		return nil
	}
}

func (e *elasticsearchExporter) pushLogsData(ctx context.Context, ld plog.Logs) error {
	e.wg.Add(1)
	defer e.wg.Done()

	session, err := e.bulkIndexer.StartSession(ctx)
	if err != nil {
		return err
	}
	defer session.End()

	var errs []error
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)
		resource := rl.Resource()
		ills := rl.ScopeLogs()
		for j := 0; j < ills.Len(); j++ {
			ill := ills.At(j)
			scope := ill.Scope()
			logs := ill.LogRecords()
			for k := 0; k < logs.Len(); k++ {
				if err := e.pushLogRecord(ctx, resource, rl.SchemaUrl(), logs.At(k), scope, ill.SchemaUrl(), session); err != nil {
					if cerr := ctx.Err(); cerr != nil {
						return cerr
					}

					errs = append(errs, err)
				}
			}
		}
	}

	if err := session.Flush(ctx); err != nil {
		if cerr := ctx.Err(); cerr != nil {
			return cerr
		}
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (e *elasticsearchExporter) pushLogRecord(
	ctx context.Context,
	resource pcommon.Resource,
	resourceSchemaURL string,
	record plog.LogRecord,
	scope pcommon.InstrumentationScope,
	scopeSchemaURL string,
	bulkIndexerSession bulkIndexerSession,
) error {
	fIndex := e.index
	if e.dynamicIndex {
		fIndex = routeLogRecord(record, scope, resource, fIndex, e.otel)
	}

	if e.logstashFormat.Enabled {
		formattedIndex, err := generateIndexWithLogstashFormat(fIndex, &e.logstashFormat, time.Now())
		if err != nil {
			return err
		}
		fIndex = formattedIndex
	}

	document, err := e.model.encodeLog(resource, resourceSchemaURL, record, scope, scopeSchemaURL)
	if err != nil {
		return fmt.Errorf("failed to encode log event: %w", err)
	}
	return bulkIndexerSession.Add(ctx, fIndex, bytes.NewReader(document))
}

func (e *elasticsearchExporter) pushMetricsData(
	ctx context.Context,
	metrics pmetric.Metrics,
) error {
	e.wg.Add(1)
	defer e.wg.Done()

	session, err := e.bulkIndexer.StartSession(ctx)
	if err != nil {
		return err
	}
	defer session.End()

	var errs []error
	resourceMetrics := metrics.ResourceMetrics()
	for i := 0; i < resourceMetrics.Len(); i++ {
		resourceMetric := resourceMetrics.At(i)
		resource := resourceMetric.Resource()
		scopeMetrics := resourceMetric.ScopeMetrics()

		resourceDocs := make(map[string]map[uint32]objmodel.Document)

		for j := 0; j < scopeMetrics.Len(); j++ {
			scopeMetrics := scopeMetrics.At(j)
			scope := scopeMetrics.Scope()
			for k := 0; k < scopeMetrics.Metrics().Len(); k++ {
				metric := scopeMetrics.Metrics().At(k)

				upsertDataPoint := func(dp dataPoint, dpValue pcommon.Value) error {
					fIndex, err := e.getMetricDataPointIndex(resource, scope, dp)
					if err != nil {
						return err
					}
					if _, ok := resourceDocs[fIndex]; !ok {
						resourceDocs[fIndex] = make(map[uint32]objmodel.Document)
					}

					if err = e.model.upsertMetricDataPointValue(resourceDocs[fIndex], resource, scope, metric, dp, dpValue); err != nil {
						return err
					}
					return nil
				}

				// TODO: support exponential histogram
				switch metric.Type() {
				case pmetric.MetricTypeSum:
					dps := metric.Sum().DataPoints()
					for l := 0; l < dps.Len(); l++ {
						dp := dps.At(l)
						val, err := numberToValue(dp)
						if err != nil {
							errs = append(errs, err)
							continue
						}
						if err := upsertDataPoint(dp, val); err != nil {
							errs = append(errs, err)
							continue
						}
					}
				case pmetric.MetricTypeGauge:
					dps := metric.Gauge().DataPoints()
					for l := 0; l < dps.Len(); l++ {
						dp := dps.At(l)
						val, err := numberToValue(dp)
						if err != nil {
							errs = append(errs, err)
							continue
						}
						if err := upsertDataPoint(dp, val); err != nil {
							errs = append(errs, err)
							continue
						}
					}
				case pmetric.MetricTypeHistogram:
					dps := metric.Histogram().DataPoints()
					for l := 0; l < dps.Len(); l++ {
						dp := dps.At(l)
						val, err := histogramToValue(dp)
						if err != nil {
							errs = append(errs, err)
							continue
						}
						if err := upsertDataPoint(dp, val); err != nil {
							errs = append(errs, err)
							continue
						}
					}
				}
			}
		}

		for fIndex, docs := range resourceDocs {
			for _, doc := range docs {
				var (
					docBytes []byte
					err      error
				)
				docBytes, err = e.model.encodeDocument(doc)
				if err != nil {
					errs = append(errs, err)
					continue
				}
				if err := session.Add(ctx, fIndex, bytes.NewReader(docBytes)); err != nil {
					if cerr := ctx.Err(); cerr != nil {
						return cerr
					}
					errs = append(errs, err)
				}
			}
		}
	}

	if err := session.Flush(ctx); err != nil {
		if cerr := ctx.Err(); cerr != nil {
			return cerr
		}
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (e *elasticsearchExporter) getMetricDataPointIndex(
	resource pcommon.Resource,
	scope pcommon.InstrumentationScope,
	dataPoint dataPoint,
) (string, error) {
	fIndex := e.index
	if e.dynamicIndex {
		fIndex = routeDataPoint(dataPoint, scope, resource, fIndex, e.otel)
	}

	if e.logstashFormat.Enabled {
		formattedIndex, err := generateIndexWithLogstashFormat(fIndex, &e.logstashFormat, time.Now())
		if err != nil {
			return "", err
		}
		fIndex = formattedIndex
	}
	return fIndex, nil
}

func (e *elasticsearchExporter) pushTraceData(
	ctx context.Context,
	td ptrace.Traces,
) error {
	e.wg.Add(1)
	defer e.wg.Done()

	session, err := e.bulkIndexer.StartSession(ctx)
	if err != nil {
		return err
	}
	defer session.End()

	var errs []error
	resourceSpans := td.ResourceSpans()
	for i := 0; i < resourceSpans.Len(); i++ {
		il := resourceSpans.At(i)
		resource := il.Resource()
		scopeSpans := il.ScopeSpans()
		for j := 0; j < scopeSpans.Len(); j++ {
			scopeSpan := scopeSpans.At(j)
			scope := scopeSpan.Scope()
			spans := scopeSpan.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)
				if err := e.pushTraceRecord(ctx, resource, span, scope, session); err != nil {
					if cerr := ctx.Err(); cerr != nil {
						return cerr
					}
					errs = append(errs, err)
				}
			}
		}
	}

	if err := session.Flush(ctx); err != nil {
		if cerr := ctx.Err(); cerr != nil {
			return cerr
		}
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (e *elasticsearchExporter) pushTraceRecord(
	ctx context.Context,
	resource pcommon.Resource,
	span ptrace.Span,
	scope pcommon.InstrumentationScope,
	bulkIndexerSession bulkIndexerSession,
) error {
	fIndex := e.index
	if e.dynamicIndex {
		fIndex = routeSpan(span, scope, resource, fIndex, e.otel)
	}

	if e.logstashFormat.Enabled {
		formattedIndex, err := generateIndexWithLogstashFormat(fIndex, &e.logstashFormat, time.Now())
		if err != nil {
			return err
		}
		fIndex = formattedIndex
	}

	document, err := e.model.encodeSpan(resource, span, scope)
	if err != nil {
		return fmt.Errorf("failed to encode trace record: %w", err)
	}
	return bulkIndexerSession.Add(ctx, fIndex, bytes.NewReader(document))
}
