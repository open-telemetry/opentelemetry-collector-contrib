// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mockdatadogagentexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/mockdatasenders/mockdatadogagentexporter"

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net/http"

	"github.com/DataDog/datadog-agent/pkg/trace/exportable/pb"
	"github.com/tinylib/msgp/msgp"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type ddExporter struct {
	endpoint       string
	client         *http.Client
	clientSettings *confighttp.ClientConfig
}

func createExporter(c *Config) *ddExporter {
	dd := &ddExporter{
		endpoint:       c.Endpoint,
		clientSettings: &c.ClientConfig,
		client:         nil,
	}

	return dd
}

// start creates the http client
func (dd *ddExporter) start(ctx context.Context, host component.Host) (err error) {
	dd.client, err = dd.clientSettings.ToClient(ctx, host, componenttest.NewNopTelemetrySettings())
	return
}

func (dd *ddExporter) pushTraces(ctx context.Context, td ptrace.Traces) error {
	var traces pb.Traces

	for i := 0; i < td.ResourceSpans().Len(); i++ {
		resSpans := td.ResourceSpans().At(i)
		var trace pb.Trace
		for l := 0; l < resSpans.ScopeSpans().Len(); l++ {
			ils := resSpans.ScopeSpans().At(i)
			for s := 0; s < ils.Spans().Len(); s++ {
				span := ils.Spans().At(s)
				newSpan := pb.Span{
					Service:  "test",
					Name:     "test",
					Resource: "test",
					Start:    int64(span.StartTimestamp()),
					Duration: int64(span.EndTimestamp() - span.StartTimestamp()),
					Error:    0,
					Metrics:  nil,
					Meta:     map[string]string{},
					Type:     "custom",
				}
				for k, v := range span.Attributes().All() {
					newSpan.GetMeta()[k] = v.AsString()
				}
				var traceIDBytes [16]byte
				var spanIDBytes [8]byte
				var parentIDBytes [8]byte
				traceIDBytes = span.TraceID()
				spanIDBytes = span.SpanID()
				parentIDBytes = span.ParentSpanID()
				binary.BigEndian.PutUint64(traceIDBytes[:], newSpan.TraceID)
				binary.BigEndian.PutUint64(spanIDBytes[:], newSpan.SpanID)
				binary.BigEndian.PutUint64(parentIDBytes[:], newSpan.ParentID)
				trace = append(trace, &newSpan)
			}
			traces = append(traces, trace)
		}
	}
	var buf bytes.Buffer
	err := msgp.Encode(&buf, &traces)
	if err != nil {
		return consumererror.NewPermanent(fmt.Errorf("failed to encode msgp: %w", err))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, dd.endpoint, &buf)
	if err != nil {
		return fmt.Errorf("failed to push trace data via DD exporter: %w", err)
	}
	req.Header.Set("Content-Type", "application/msgpack")

	resp, err := dd.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to push trace data via DD exporter: %w", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("failed the request with status code %d", resp.StatusCode)
	}
	return nil
}
