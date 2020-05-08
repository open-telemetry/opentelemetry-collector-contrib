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

	"github.com/open-telemetry/opentelemetry-collector/component"
	"github.com/open-telemetry/opentelemetry-collector/consumer/pdata"
)

// SentryExporter defines the Sentry Exporter
type SentryExporter struct{}

// Start the exporter
func (e *SentryExporter) Start(ctx context.Context, host component.Host) error {
	// instantiate the Sentry SDK here, and store it on the SentryExporter
	return nil
}

// ConsumeTraces receives pdata.Traces and sends them to Sentry
func (e *SentryExporter) ConsumeTraces(ctx context.Context, td pdata.Traces) error {
	return nil
}

// Shutdown the exporter
func (e *SentryExporter) Shutdown(ctx context.Context) error {
	// cleanup sentry sdk, close channels
	return nil
}
