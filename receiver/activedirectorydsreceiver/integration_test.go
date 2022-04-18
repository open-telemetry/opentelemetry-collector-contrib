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

//go:build windows && integration

package activedirectorydsreceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest/golden"
)

/*
	TestIntegration test scraping metrics from a running Active Directory domain controller.
	The domain controller must be set up locally outside of this test in order for it to pass.
*/
func TestIntegration(t *testing.T){
	t.Parallel()
	
	fact := NewFactory()

	consumer := &consumertest.MetricsSink{}
	recv, err := fact.CreateMetricsReceiver(context.Background(), componenttest.NewNopReceiverCreateSettings(), fact.CreateDefaultConfig(), consumer)

	require.NoError(t, err)
	
	err = recv.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return len(consumer.AllMetrics()) > 0
	}, 2*time.Minute, 1*time.Second, "failed to receive any metrics")

	actualMetrics := consumer.AllMetrics()[0]
	expectedMetrics, err := golden.ReadMetrics(goldenScrapePath)
	require.NoError(t, err)

	err = scrapertest.CompareMetrics(expectedMetrics, actualMetrics, scrapertest.IgnoreMetricValues())
	require.NoError(t, err)

	err = recv.Shutdown(context.Background())
	require.NoError(t, err)

}
