// Copyright 2021, OpenTelemetry Authors
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

package uptraceexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

func TestNewTraceExporterEmptyConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	exp, err := newTraceExporter(cfg, zap.NewNop())
	require.Error(t, err)
	require.Nil(t, exp)
}

func TestNewTraceExporterEndpoint(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.HTTPClientSettings.Endpoint = "_"
	exp, err := newTraceExporter(cfg, zap.NewNop())
	require.Error(t, err)
	require.Nil(t, exp)
}

func TestTraceExporterEmptyTraces(t *testing.T) {
	ctx := context.Background()

	cfg := createDefaultConfig().(*Config)
	cfg.DSN = "https://key@api.uptrace.dev/1"

	exp, err := newTraceExporter(cfg, zap.NewNop())
	require.NoError(t, err)
	require.NotNil(t, exp)

	dropped, err := exp.pushTraceData(ctx, pdata.NewTraces())
	require.NoError(t, err)
	require.Zero(t, dropped)

	err = exp.Shutdown(ctx)
	require.NoError(t, err)
}
