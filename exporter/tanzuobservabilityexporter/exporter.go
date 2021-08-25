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

package tanzuobservabilityexporter

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"

	"github.com/google/uuid"
	"github.com/wavefronthq/wavefront-sdk-go/senders"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

const (
	defaultApplicationName = "defaultApp"
	defaultServiceName     = "defaultService"
	labelApplication       = "application"
	labelError             = "error"
	labelEventName         = "name"
	labelService           = "service"
	labelSpanKind          = "span.kind"
	labelStatusMessage     = "status.message"
	labelStatusCode        = "status.code"
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

func newTracesExporter(l *zap.Logger, c config.Exporter) (*tracesExporter, error) {
	cfg, ok := c.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config: %#v", c)
	}

	endpoint, err := url.Parse(cfg.Traces.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse traces.endpoint: %v", err)
	}
	tracingPort, err := strconv.Atoi(endpoint.Port())
	if err != nil {
		// the port is empty, otherwise url.Parse would have failed above
		return nil, fmt.Errorf("traces.endpoint requires a port")
	}

	// we specify a MetricsPort so the SDK can report its internal metrics
	// but don't currently export any metrics from the pipeline
	s, err := senders.NewProxySender(&senders.ProxyConfiguration{
		Host:                 endpoint.Hostname(),
		MetricsPort:          2878,
		TracingPort:          tracingPort,
		FlushIntervalSeconds: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create proxy sender: %v", err)
	}

	return &tracesExporter{
		cfg:    cfg,
		sender: s,
		logger: l,
	}, nil
}

func (e *tracesExporter) pushTraceData(ctx context.Context, td pdata.Traces) error {
	var errs []error

	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rspans := td.ResourceSpans().At(i)
		resource := rspans.Resource()
		for j := 0; j < rspans.InstrumentationLibrarySpans().Len(); j++ {
			ispans := rspans.InstrumentationLibrarySpans().At(j)
			transform := newTraceTransformer(resource)
			for k := 0; k < ispans.Spans().Len(); k++ {
				select {
				case <-ctx.Done():
					return consumererror.Combine(append(errs, errors.New("context canceled")))
				default:
					transformedSpan, err := transform.Span(ispans.Spans().At(k))
					if err != nil {
						errs = append(errs, err)
						continue
					}

					if err := e.recordSpan(transformedSpan); err != nil {
						errs = append(errs, err)
						continue
					}
				}
			}
		}
	}

	if err := e.sender.Flush(); err != nil {
		errs = append(errs, err)
	}
	return consumererror.Combine(errs)
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
		"",
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
