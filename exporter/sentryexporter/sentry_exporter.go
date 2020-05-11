// Copyright 2020 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sentryexporter

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"github.com/open-telemetry/opentelemetry-collector/component"
	"github.com/open-telemetry/opentelemetry-collector/consumer/pdata"
	"github.com/open-telemetry/opentelemetry-collector/exporter/exporterhelper"
)

// SentryExporter defines the Sentry Exporter
type SentryExporter struct {
	Dsn string
}

func (s *SentryExporter) pushTraceData(ctx context.Context, td pdata.Traces) (droppedSpans int, err error) {
	resourceSpans := td.ResourceSpans()

	if resourceSpans.Len() == 0 {
		return 0, nil
	}

	// Create a transaction for each resource
	// TODO: Create batches? Idk
	// transactions := make([]*SentryTransaction, 0, resourceSpans.Len())

	for i := 0; i < resourceSpans.Len(); i++ {
		rs := resourceSpans.At(i)
		if rs.IsNil() {
			continue
		}

		// TODO: Grab resource attributes to add to transaction
		// resource := rs.Resource() && check resource.IsNil() -> attr := resource.Attributes()

		// instrumentationLibrarySpans := rs.InstrumentationLibrarySpans()
	}

	// TODO: figure out how to send dropped spans
	return 0, nil
}

func spanToSentrySpan(span pdata.Span) (*SentrySpan, error) {
	if span.IsNil() {
		return nil, nil
	}

	// traceID, err := generateSentryTraceID(span.TraceID())
	// if err != nil {
	// 	return nil, err
	// }

	return nil, nil
}

// pdata.TraceID are
func generateSentryTraceID(traceID pdata.TraceID) (string, error) {
	secondHalf := make([]byte, 16)
	rand.Read(secondHalf)

	sentryTraceID := append(traceID, secondHalf...)

	return hex.EncodeToString(sentryTraceID), nil
}

// CreateSentryExporter returns a new Sentry Exporter
func CreateSentryExporter(config *Config) (component.TraceExporter, error) {
	s := &SentryExporter{
		Dsn: config.Dsn,
	}

	exp, err := exporterhelper.NewTraceExporter(config, s.pushTraceData)

	return exp, err
}

// // Start the exporter
// func (s *SentryExporter) Start(ctx context.Context, host component.Host) error {}

// // ConsumeTraces receives pdata.Traces and sends them to Sentry
// func (s *SentryExporter) ConsumeTraces(ctx context.Context, td pdata.Traces) error {}

// // Shutdown the exporter
// func (s *SentryExporter) Shutdown(ctx context.Context) error {}
