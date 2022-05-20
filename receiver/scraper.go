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

package expvarreceiver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/expvarreceiver/internal/metadata"
)

type expVar struct {
	// Use the existing runtime struct for decoding the JSON.
	MemStats *runtime.MemStats `json:"memstats"`
}

type expVarScraper struct {
	cfg        *Config
	set        *component.ReceiverCreateSettings
	httpClient *http.Client
	mb         *metadata.MetricsBuilder
}

func newExpVarScraper(cfg *Config, set component.ReceiverCreateSettings) *expVarScraper {
	return &expVarScraper{
		cfg: cfg,
		set: &set,
		mb:  metadata.NewMetricsBuilder(cfg.MetricsConfig),
	}
}

func (e *expVarScraper) start(_ context.Context, host component.Host) error {
	httpClient, err := e.cfg.HTTPClientSettings.ToClient(host.GetExtensions(), e.set.TelemetrySettings)
	if err != nil {
		return err
	}
	e.httpClient = httpClient
	return nil
}

func (e *expVarScraper) scrape(_ context.Context) (pmetric.Metrics, error) {
	emptyMetrics := pmetric.NewMetrics()
	resp, err := e.httpClient.Get(e.cfg.Endpoint)
	if err != nil {
		return emptyMetrics, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return emptyMetrics, fmt.Errorf("expected 200 but received %d status code", resp.StatusCode)
	}

	result, err := decodeResponseBody(resp.Body)
	if err != nil {
		return emptyMetrics, err
	}
	memStats := result.MemStats
	if memStats == nil {
		return emptyMetrics, fmt.Errorf("unmarshalled memstats data is nil")
	}

	now := pcommon.NewTimestampFromTime(time.Now())

	e.mb.RecordProcessRuntimeMemstatsTotalAllocDataPoint(now, int64(memStats.TotalAlloc))
	e.mb.RecordProcessRuntimeMemstatsSysDataPoint(now, int64(memStats.Sys))
	e.mb.RecordProcessRuntimeMemstatsLookupsDataPoint(now, int64(memStats.Lookups))
	e.mb.RecordProcessRuntimeMemstatsMallocsDataPoint(now, int64(memStats.Mallocs))
	e.mb.RecordProcessRuntimeMemstatsFreesDataPoint(now, int64(memStats.Frees))
	e.mb.RecordProcessRuntimeMemstatsHeapAllocDataPoint(now, int64(memStats.HeapAlloc))
	e.mb.RecordProcessRuntimeMemstatsHeapSysDataPoint(now, int64(memStats.HeapSys))
	e.mb.RecordProcessRuntimeMemstatsHeapIdleDataPoint(now, int64(memStats.HeapIdle))
	e.mb.RecordProcessRuntimeMemstatsHeapInuseDataPoint(now, int64(memStats.HeapInuse))
	e.mb.RecordProcessRuntimeMemstatsHeapReleasedDataPoint(now, int64(memStats.HeapReleased))
	e.mb.RecordProcessRuntimeMemstatsHeapObjectsDataPoint(now, int64(memStats.HeapObjects))
	e.mb.RecordProcessRuntimeMemstatsStackInuseDataPoint(now, int64(memStats.StackInuse))
	e.mb.RecordProcessRuntimeMemstatsStackSysDataPoint(now, int64(memStats.StackSys))
	e.mb.RecordProcessRuntimeMemstatsMspanInuseDataPoint(now, int64(memStats.MSpanInuse))
	e.mb.RecordProcessRuntimeMemstatsMspanSysDataPoint(now, int64(memStats.MSpanSys))
	e.mb.RecordProcessRuntimeMemstatsMcacheInuseDataPoint(now, int64(memStats.MCacheInuse))
	e.mb.RecordProcessRuntimeMemstatsMspanSysDataPoint(now, int64(memStats.MCacheSys))
	e.mb.RecordProcessRuntimeMemstatsBuckHashSysDataPoint(now, int64(memStats.BuckHashSys))
	e.mb.RecordProcessRuntimeMemstatsGcSysDataPoint(now, int64(memStats.GCSys))
	e.mb.RecordProcessRuntimeMemstatsOtherSysDataPoint(now, int64(memStats.OtherSys))
	e.mb.RecordProcessRuntimeMemstatsNextGcDataPoint(now, int64(memStats.NextGC))
	e.mb.RecordProcessRuntimeMemstatsPauseTotalDataPoint(now, int64(memStats.PauseTotalNs))
	e.mb.RecordProcessRuntimeMemstatsLastPauseDataPoint(now, int64(memStats.PauseNs[(memStats.NumGC+255)%256]))
	e.mb.RecordProcessRuntimeMemstatsNumGcDataPoint(now, int64(memStats.NumGC))
	e.mb.RecordProcessRuntimeMemstatsNumForcedGcDataPoint(now, int64(memStats.NumForcedGC))
	e.mb.RecordProcessRuntimeMemstatsGcCPUFractionDataPoint(now, memStats.GCCPUFraction)

	return e.mb.Emit(), nil
}

func decodeResponseBody(body io.ReadCloser) (*expVar, error) {
	var result expVar
	if err := json.NewDecoder(body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}
