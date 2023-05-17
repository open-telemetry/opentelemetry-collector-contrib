// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tanzuobservabilityexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tanzuobservabilityexporter"

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/wavefronthq/wavefront-sdk-go/senders"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

const (
	defaultApplicationName  = "defaultApp"
	defaultServiceName      = "defaultService"
	defaultMetricsPort      = 2878
	labelApplication        = "application"
	labelCluster            = "cluster"
	labelShard              = "shard"
	labelError              = "error"
	labelEventName          = "name"
	labelService            = "service"
	labelSpanKind           = "span.kind"
	labelSource             = "source"
	labelDroppedEventsCount = "otel.dropped_events_count"
	labelDroppedLinksCount  = "otel.dropped_links_count"
	labelDroppedAttrsCount  = "otel.dropped_attributes_count"
	labelOtelScopeName      = "otel.scope.name"
	labelOtelScopeVersion   = "otel.scope.version"
)

// spanSender Interface for sending tracing spans to Tanzu Observability
type spanSender interface {
	// SendSpan mirrors sender.SpanSender from wavefront-sdk-go.
	// traceId, spanId, parentIds and preceding spanIds are expected to be UUID strings.
	// parents and preceding spans can be empty for a root span.
	// span tag keys can be repeated (example: "user"="foo" and "user"="bar")
	SendSpan(name string, startMillis, durationMillis int64, source, traceID, spanID string, parents, followsFrom []string, tags []senders.SpanTag, spanLogs []senders.SpanLog) error
	Flush() error
	Close()
}

type tracesExporter struct {
	cfg    *Config
	sender spanSender
	logger *zap.Logger
}

func newTracesExporter(settings exporter.CreateSettings, c component.Config) (*tracesExporter, error) {
	cfg, ok := c.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config: %#v", c)
	}
	if !cfg.hasTracesEndpoint() {
		return nil, fmt.Errorf("traces.endpoint required")
	}
	_, _, err := cfg.parseTracesEndpoint()
	if err != nil {
		return nil, fmt.Errorf("failed to parse traces.endpoint: %w", err)
	}
	metricsPort := defaultMetricsPort
	if cfg.hasMetricsEndpoint() {
		_, metricsPort, err = cfg.parseMetricsEndpoint()
		if err != nil {
			return nil, fmt.Errorf("failed to parse metrics.endpoint: %w", err)
		}
	}

	// we specify a MetricsPort so the SDK can report its internal metrics
	// but don't currently export any metrics from the pipeline
	s, err := senders.NewSender(cfg.Traces.Endpoint,
		senders.MetricsPort(metricsPort),
		senders.FlushIntervalSeconds(60),
		senders.SDKMetricsTags(map[string]string{"otel.traces.collector_version": settings.BuildInfo.Version}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create proxy sender: %w", err)
	}

	return &tracesExporter{
		cfg:    cfg,
		sender: s,
		logger: settings.Logger,
	}, nil
}

func (e *tracesExporter) pushTraceData(ctx context.Context, td ptrace.Traces) error {
	var errs error

	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rspans := td.ResourceSpans().At(i)
		resource := rspans.Resource()
		for j := 0; j < rspans.ScopeSpans().Len(); j++ {
			ispans := rspans.ScopeSpans().At(j)
			transform := newTraceTransformer(resource)

			libraryName := ispans.Scope().Name()
			libraryVersion := ispans.Scope().Version()

			for k := 0; k < ispans.Spans().Len(); k++ {
				select {
				case <-ctx.Done():
					return multierr.Append(errs, errors.New("context canceled"))
				default:
					transformedSpan, err := transform.Span(ispans.Spans().At(k))
					if err != nil {
						errs = multierr.Append(errs, err)
						continue
					}

					if libraryName != "" {
						transformedSpan.Tags[labelOtelScopeName] = libraryName
					}

					if libraryVersion != "" {
						transformedSpan.Tags[labelOtelScopeVersion] = libraryVersion
					}

					if err := e.recordSpan(transformedSpan); err != nil {
						errs = multierr.Append(errs, err)
						continue
					}
				}
			}
		}
	}

	errs = multierr.Append(errs, e.sender.Flush())
	return errs
}

func (e *tracesExporter) recordSpan(span span) error {
	var parents []string
	if span.ParentSpanID != uuid.Nil {
		parents = []string{span.ParentSpanID.String()}
	}

	return e.sender.SendSpan(
		span.Name,
		span.StartMillis,
		span.DurationMillis,
		span.Source,
		span.TraceID.String(),
		span.SpanID.String(),
		parents,
		nil,
		mapToSpanTags(span.Tags),
		span.SpanLogs,
	)
}

func (e *tracesExporter) shutdown(_ context.Context) error {
	e.sender.Close()
	return nil
}

func mapToSpanTags(tags map[string]string) []senders.SpanTag {
	var spanTags []senders.SpanTag
	for k, v := range tags {
		spanTags = append(spanTags, senders.SpanTag{
			Key:   k,
			Value: v,
		})
	}
	return spanTags
}
