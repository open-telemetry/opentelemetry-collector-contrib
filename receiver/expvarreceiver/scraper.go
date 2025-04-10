// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package expvarreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/expvarreceiver"

import (
	"context"
	"encoding/json"
	"errors"
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
	set    *receiver.Settings
	client *http.Client
	mb     *metadata.MetricsBuilder
}

func newExpVarScraper(cfg *Config, set receiver.Settings) *expVarScraper {
	return &expVarScraper{
		cfg: cfg,
		set: &set,
		mb:  metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, set),
	}
}

func (e *expVarScraper) start(ctx context.Context, host component.Host) error {
	client, err := e.cfg.ToClient(ctx, host, e.set.TelemetrySettings)
	if err != nil {
		return err
	}
	e.client = client
	return nil
}

func (e *expVarScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	emptyMetrics := pmetric.NewMetrics()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.cfg.Endpoint, nil)
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
		return emptyMetrics, errors.New("unmarshalled memstats data is nil")
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
	e.mb.RecordProcessRuntimeMemstatsMcacheSysDataPoint(now, int64(memStats.MCacheSys))
	e.mb.RecordProcessRuntimeMemstatsBuckHashSysDataPoint(now, int64(memStats.BuckHashSys))
	e.mb.RecordProcessRuntimeMemstatsGcSysDataPoint(now, int64(memStats.GCSys))
	e.mb.RecordProcessRuntimeMemstatsOtherSysDataPoint(now, int64(memStats.OtherSys))
	e.mb.RecordProcessRuntimeMemstatsNextGcDataPoint(now, int64(memStats.NextGC))
	e.mb.RecordProcessRuntimeMemstatsPauseTotalDataPoint(now, int64(memStats.PauseTotalNs))
	e.mb.RecordProcessRuntimeMemstatsNumGcDataPoint(now, int64(memStats.NumGC))
	e.mb.RecordProcessRuntimeMemstatsNumForcedGcDataPoint(now, int64(memStats.NumForcedGC))
	e.mb.RecordProcessRuntimeMemstatsGcCPUFractionDataPoint(now, memStats.GCCPUFraction)
	// Memstats exposes a circular buffer of recent GC stop-the-world pause times.
	// The most recent pause is at PauseNs[(NumGC+255)%256].
	e.mb.RecordProcessRuntimeMemstatsLastPauseDataPoint(now, int64(memStats.PauseNs[(memStats.NumGC+255)%256]))

	return e.mb.Emit(), nil
}

func decodeResponseBody(body io.ReadCloser) (*expVar, error) {
	var result expVar
	if err := json.NewDecoder(body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}
