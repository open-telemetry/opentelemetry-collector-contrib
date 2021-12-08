// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mockdatadogagentexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/mockdatareceivers/mockawsxrayreceiver"

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"net/http"

	"github.com/DataDog/datadog-agent/pkg/trace/exportable/pb"
	"github.com/tinylib/msgp/msgp"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/model/pdata"
)

type ddExporter struct {
	endpoint       string
	client         *http.Client
	clientSettings *confighttp.HTTPClientSettings
}

func createExporter(cfg *Config) *ddExporter {
	dd := &ddExporter{
		endpoint:       cfg.Endpoint,
		clientSettings: &cfg.HTTPClientSettings,
		client:         nil,
	}

	return dd
}

// start creates the http client
func (dd *ddExporter) start(_ context.Context, host component.Host) (err error) {
	dd.client, err = dd.clientSettings.ToClient(host.GetExtensions())
	return
}

func (dd *ddExporter) pushTraces(ctx context.Context, td pdata.Traces) error {
	var traces pb.Traces

	for i := 0; i < td.ResourceSpans().Len(); i++ {
		resSpans := td.ResourceSpans().At(i)
		var trace pb.Trace
		for l := 0; l < resSpans.InstrumentationLibrarySpans().Len(); l++ {
			ils := resSpans.InstrumentationLibrarySpans().At(i)
			for s := 0; s < ils.Spans().Len(); s++ {
				span := ils.Spans().At(s)
				var newSpan = pb.Span{
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
				span.Attributes().Range(func(k string, v pdata.AttributeValue) bool {
					newSpan.GetMeta()[k] = v.AsString()
					return true
				})
				var traceIDBytes = span.TraceID().Bytes()
				var spanIDBytes = span.SpanID().Bytes()
				var parentIDBytes = span.ParentSpanID().Bytes()
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

	req, err := http.NewRequestWithContext(ctx, "POST", dd.endpoint, &buf)
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
