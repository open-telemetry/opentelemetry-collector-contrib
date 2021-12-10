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

package memcachedreceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest"
)

func TestScraper(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	sc := newMemcachedScraper(zap.NewNop(), cfg)
	sc.newClient = func(endpoint string, timeout time.Duration) (client, error) {
		return &fakeClient{}, nil
	}

	ms, err := sc.scrape(context.Background())
	require.NoError(t, err)

	expectedMetrics, err := scrapertest.ReadExpected("./testdata/expected_metrics/test_scraper/expected.json")
	require.NoError(t, err)

	eMetricSlice := expectedMetrics.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0).Metrics()
	aMetricSlice := ms.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0).Metrics()
	require.NoError(t, scrapertest.CompareMetricSlices(eMetricSlice, aMetricSlice))
}
