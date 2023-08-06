// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package expvarreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/expvarreceiver"

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
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/expvarreceiver/internal/metadata"
)

type expVar struct {
	// Use the existing runtime struct for decoding the JSON.
	MemStats *runtime.MemStats `json:"memstats"`
}

type expVarScraper struct {
	cfg    *Config
	set    *receiver.CreateSettings
	client *http.Client
	mb     *metadata.MetricsBuilder
}

func newExpVarScraper(cfg *Config, set receiver.CreateSettings) *expVarScraper {
	return &expVarScraper{
		cfg: cfg,
		set: &set,
		mb:  metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, set),
	}
}

func (e *expVarScraper) start(_ context.Context, host component.Host) error {
	client, err := e.cfg.HTTPClientSettings.ToClient(host, e.set.TelemetrySettings)
	if err != nil {
		return err
	}
	e.client = client
	return nil
}

func (e *expVarScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	emptyMetrics := pmetric.NewMetrics()
	req, err := http.NewRequestWithContext(ctx, "GET", e.cfg.Endpoint, nil)
	if err != nil {
		return emptyMetrics, err
	}
	resp, err := e.client.Do(req)
	if err != nil {
		return emptyMetrics, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return emptyMetrics, fmt.Errorf("expected 200 but received %d status code", resp.StatusCode)
	}

	result, err := decodeResponseBody(resp.Body)
	if err != nil {
		return emptyMetrics, fmt.Errorf("could not decode response body to JSON: %w", err)
	}
	memStats := result.MemStats
	if memStats == nil {
		return emptyMetrics, fmt.Errorf("unmarshalled memstats data is nil")
	}

	now := pcommon.NewTimestampFromTime(time.Now())
	rmb := e.mb.ResourceMetricsBuilder(pcommon.NewResource())

	rmb.RecordProcessRuntimeMemstatsTotalAllocDataPoint(now, int64(memStats.TotalAlloc))
	rmb.RecordProcessRuntimeMemstatsSysDataPoint(now, int64(memStats.Sys))
	rmb.RecordProcessRuntimeMemstatsLookupsDataPoint(now, int64(memStats.Lookups))
	rmb.RecordProcessRuntimeMemstatsMallocsDataPoint(now, int64(memStats.Mallocs))
	rmb.RecordProcessRuntimeMemstatsFreesDataPoint(now, int64(memStats.Frees))
	rmb.RecordProcessRuntimeMemstatsHeapAllocDataPoint(now, int64(memStats.HeapAlloc))
	rmb.RecordProcessRuntimeMemstatsHeapSysDataPoint(now, int64(memStats.HeapSys))
	rmb.RecordProcessRuntimeMemstatsHeapIdleDataPoint(now, int64(memStats.HeapIdle))
	rmb.RecordProcessRuntimeMemstatsHeapInuseDataPoint(now, int64(memStats.HeapInuse))
	rmb.RecordProcessRuntimeMemstatsHeapReleasedDataPoint(now, int64(memStats.HeapReleased))
	rmb.RecordProcessRuntimeMemstatsHeapObjectsDataPoint(now, int64(memStats.HeapObjects))
	rmb.RecordProcessRuntimeMemstatsStackInuseDataPoint(now, int64(memStats.StackInuse))
	rmb.RecordProcessRuntimeMemstatsStackSysDataPoint(now, int64(memStats.StackSys))
	rmb.RecordProcessRuntimeMemstatsMspanInuseDataPoint(now, int64(memStats.MSpanInuse))
	rmb.RecordProcessRuntimeMemstatsMspanSysDataPoint(now, int64(memStats.MSpanSys))
	rmb.RecordProcessRuntimeMemstatsMcacheInuseDataPoint(now, int64(memStats.MCacheInuse))
	rmb.RecordProcessRuntimeMemstatsMcacheSysDataPoint(now, int64(memStats.MCacheSys))
	rmb.RecordProcessRuntimeMemstatsBuckHashSysDataPoint(now, int64(memStats.BuckHashSys))
	rmb.RecordProcessRuntimeMemstatsGcSysDataPoint(now, int64(memStats.GCSys))
	rmb.RecordProcessRuntimeMemstatsOtherSysDataPoint(now, int64(memStats.OtherSys))
	rmb.RecordProcessRuntimeMemstatsNextGcDataPoint(now, int64(memStats.NextGC))
	rmb.RecordProcessRuntimeMemstatsPauseTotalDataPoint(now, int64(memStats.PauseTotalNs))
	rmb.RecordProcessRuntimeMemstatsNumGcDataPoint(now, int64(memStats.NumGC))
	rmb.RecordProcessRuntimeMemstatsNumForcedGcDataPoint(now, int64(memStats.NumForcedGC))
	rmb.RecordProcessRuntimeMemstatsGcCPUFractionDataPoint(now, memStats.GCCPUFraction)
	// Memstats exposes a circular buffer of recent GC stop-the-world pause times.
	// The most recent pause is at PauseNs[(NumGC+255)%256].
	rmb.RecordProcessRuntimeMemstatsLastPauseDataPoint(now, int64(memStats.PauseNs[(memStats.NumGC+255)%256]))

	return e.mb.Emit(), nil
}

func decodeResponseBody(body io.ReadCloser) (*expVar, error) {
	var result expVar
	if err := json.NewDecoder(body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}
