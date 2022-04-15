// Copyright  The OpenTelemetry Authors
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

//go:build windows
// +build windows

package sqlserverreceiver

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest/golden"
)

func TestSqlServerIntegration(t *testing.T) {
	t.Parallel()

	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.ScraperControllerSettings.CollectionInterval = 100 * time.Millisecond

	consumer := new(consumertest.MetricsSink)
	settings := componenttest.NewNopReceiverCreateSettings()
	rcvr, err := f.CreateMetricsReceiver(context.Background(), settings, cfg, consumer)
	require.NoError(t, err, "failed creating metrics receiver")
	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
	require.Eventuallyf(t, func() bool {
		return consumer.DataPointCount() > 0
	}, 2*time.Minute, 1*time.Second, "failed to receive more than 0 metrics")
	require.NoError(t, rcvr.Shutdown(context.Background()))

	actualMetrics := consumer.AllMetrics()[0]

	expectedFile := filepath.Join("testdata", "integration", "expected.json")
	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)

	require.NoError(t, scrapertest.CompareMetrics(expectedMetrics, actualMetrics, scrapertest.IgnoreMetricValues()))
}
