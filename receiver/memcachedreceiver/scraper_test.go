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

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testing/container"
)

func TestScraper(t *testing.T) {
	cs := container.New(t)
	c := cs.StartImage("memcached:1.6-alpine", container.WithPortReady(11211))

	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Endpoint = c.AddrForPort(11211)

	sc := newMemcachedScraper(zap.NewNop(), cfg)

	err := sc.Start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err)
	metrics, err := sc.Scrape(context.Background(), cfg.ID())
	require.Nil(t, err)
	rms := metrics.ResourceMetrics()
	require.Equal(t, 1, rms.Len())
	rm := rms.At(0)

	ilms := rm.InstrumentationLibraryMetrics()
	require.Equal(t, 1, ilms.Len())

	ilm := ilms.At(0)
	ms := ilm.Metrics()

	require.Equal(t, 10, ms.Len())

	for i := 0; i < ms.Len(); i++ {
		m := ms.At(i)
		switch m.Name() {
		case "memcached.bytes":
			require.Equal(t, 1, m.IntGauge().DataPoints().Len())
		case "memcached.current_connections":
			require.Equal(t, 1, m.IntGauge().DataPoints().Len())
		case "memcached.total_connections":
			require.Equal(t, 1, m.IntSum().DataPoints().Len())
		case "memcached.command_count":
			require.Equal(t, 4, m.IntSum().DataPoints().Len())
		case "memcached.current_items":
			require.Equal(t, 1, m.DoubleGauge().DataPoints().Len())
		case "memcached.eviction_count":
			require.Equal(t, 1, m.IntSum().DataPoints().Len())
		case "memcached.network":
			require.Equal(t, 2, m.IntSum().DataPoints().Len())
		case "memcached.operation_count":
			require.Equal(t, 6, m.IntSum().DataPoints().Len())
		case "memcached.rusage":
			require.Equal(t, 2, m.DoubleGauge().DataPoints().Len())
		case "memcached.threads":
			require.Equal(t, 1, m.DoubleGauge().DataPoints().Len())
		default:
			t.Error("Incorrect name or untracked metric name.")
		}
	}
}
