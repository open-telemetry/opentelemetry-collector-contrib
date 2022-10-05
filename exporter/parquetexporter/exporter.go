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

package parquetexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/parquetexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type parquetExporter struct {
	path string
}

func (e parquetExporter) start(ctx context.Context, host component.Host) error {
	return nil
}

func (e parquetExporter) shutdown(ctx context.Context) error {
	return nil
}

func (e parquetExporter) consumeMetrics(ctx context.Context, ld pmetric.Metrics) error {
	return nil
}

func (e parquetExporter) consumeTraces(ctx context.Context, ld ptrace.Traces) error {
	return nil
}

func (e parquetExporter) consumeLogs(ctx context.Context, ld plog.Logs) error {
	return nil
}
