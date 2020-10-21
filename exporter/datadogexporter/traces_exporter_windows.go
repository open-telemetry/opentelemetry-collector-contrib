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

// +build windows

package datadogexporter

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/config"
)

type traceExporterStub struct {}

func newTraceExporter(logger *zap.Logger, cfg *config.Config) (*traceExporterStub, error) {
	return &traceExporterStub{}, errors.New("datadog trace export is currently not supported on Windows")
}

func (exp *traceExporterStub) pushTraceData(
	_ context.Context,
	_ pdata.Traces,
) (int, error) {
	return 0, errors.New("datadog trace export is currently not supported on Windows")
}
