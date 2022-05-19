// Copyright 2020, OpenTelemetry Authors
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

//go:build windows
// +build windows

package sqlserverreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestSqlServerScraper(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	logger, obsLogs := observer.New(zap.WarnLevel)
	s := newSqlServerScraper(componenttest.NewNopReceiverCreateSettings(), cfg)

	s.start(context.Background(), nil)
	assert.Equal(t, 0, len(s.watcherRecorders))
	assert.Equal(t, 21, obsLogs.Len())
	assert.Equal(t, 21, obsLogs.FilterMessageSnippet("failed to create perf counter with path \\SQLServer:").Len())
	assert.Equal(t, 21, obsLogs.FilterMessageSnippet("The specified object was not found on the computer.").Len())
	assert.Equal(t, 1, obsLogs.FilterMessageSnippet("\\SQLServer:General Statistics\\").Len())
	assert.Equal(t, 3, obsLogs.FilterMessageSnippet("\\SQLServer:SQL Statistics\\").Len())
	assert.Equal(t, 2, obsLogs.FilterMessageSnippet("\\SQLServer:Locks(_Total)\\").Len())
	assert.Equal(t, 6, obsLogs.FilterMessageSnippet("\\SQLServer:Buffer Manager\\").Len())
	assert.Equal(t, 1, obsLogs.FilterMessageSnippet("\\SQLServer:Access Methods(_Total)\\").Len())
	assert.Equal(t, 8, obsLogs.FilterMessageSnippet("\\SQLServer:Databases(*)\\").Len())

	metrics, err := s.scrape(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, metrics.ResourceMetrics().Len())

	err = s.shutdown(context.Background())
	require.NoError(t, err)
}
