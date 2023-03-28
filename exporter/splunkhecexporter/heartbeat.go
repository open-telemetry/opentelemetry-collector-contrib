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

package splunkhecexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter"

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
)

const (
	metricsPrefix              = "otelcol_exporter_splunkhec_heartbeat"
	defaultHBSentMetricsName   = metricsPrefix + "_sent"
	defaultHBFailedMetricsName = metricsPrefix + "_failed"
)

type heartbeater struct {
	config     *Config
	pushLogFn  func(ctx context.Context, ld plog.Logs) error
	hbRunOnce  sync.Once
	hbDoneChan chan struct{}

	// Observability
	heartbeatSuccessTotal *stats.Int64Measure
	heartbeatErrorTotal   *stats.Int64Measure
	tagMutators           []tag.Mutator
}

func getMetricsName(overrides map[string]string, metricName string) string {
	if name, ok := overrides[metricName]; ok {
		return name
	}
	return metricName
}

func newHeartbeater(config *Config, pushLogFn func(ctx context.Context, ld plog.Logs) error) *heartbeater {
	interval := config.HecHeartbeat.Interval
	if interval == 0 {
		return nil
	}

	var heartbeatSentTotal, heartbeatFailedTotal *stats.Int64Measure
	var tagMutators []tag.Mutator
	if config.HecTelemetry.Enabled {
		overrides := config.HecTelemetry.OverrideMetricsNames
		extraAttributes := config.HecTelemetry.ExtraAttributes
		var tags []tag.Key
		tagMutators = []tag.Mutator{}
		for key, val := range extraAttributes {
			newTag, _ := tag.NewKey(key)
			tags = append(tags, newTag)
			tagMutators = append(tagMutators, tag.Insert(newTag, val))
		}

		heartbeatSentTotal = stats.Int64(
			getMetricsName(overrides, defaultHBSentMetricsName),
			"number of heartbeats made to the destination",
			stats.UnitDimensionless)

		heartbeatSuccessTotalView := &view.View{
			Name:        heartbeatSentTotal.Name(),
			Description: heartbeatSentTotal.Description(),
			TagKeys:     tags,
			Measure:     heartbeatSentTotal,
			Aggregation: view.Sum(),
		}

		heartbeatFailedTotal = stats.Int64(
			getMetricsName(overrides, defaultHBFailedMetricsName),
			"number of heartbeats made to destination that failed",
			stats.UnitDimensionless)

		heartbeatErrorTotalView := &view.View{
			Name:        heartbeatFailedTotal.Name(),
			Description: heartbeatFailedTotal.Description(),
			TagKeys:     tags,
			Measure:     heartbeatFailedTotal,
			Aggregation: view.Sum(),
		}

		if err := view.Register(heartbeatSuccessTotalView, heartbeatErrorTotalView); err != nil {
			return nil
		}
	}

	return &heartbeater{
		config:                config,
		pushLogFn:             pushLogFn,
		hbDoneChan:            make(chan struct{}),
		heartbeatSuccessTotal: heartbeatSentTotal,
		heartbeatErrorTotal:   heartbeatFailedTotal,
		tagMutators:           tagMutators,
	}
}

func (h *heartbeater) shutdown() {
	close(h.hbDoneChan)
}

func (h *heartbeater) initHeartbeat(buildInfo component.BuildInfo) {
	interval := h.config.HecHeartbeat.Interval
	if interval == 0 {
		return
	}

	h.hbRunOnce.Do(func() {
		heartbeatLog := h.generateHeartbeatLog(buildInfo)
		go func() {
			ticker := time.NewTicker(interval)
			for {
				select {
				case <-h.hbDoneChan:
					return
				case <-ticker.C:
					err := h.pushLogFn(context.Background(), heartbeatLog)
					h.observe(err)
				}
			}
		}()
	})
}

// there is only use case for open census metrics recording for now. Extend to use open telemetry in the future.
func (h *heartbeater) observe(err error) {
	if !h.config.HecTelemetry.Enabled {
		return
	}

	var counter *stats.Int64Measure
	if err == nil {
		counter = h.heartbeatSuccessTotal
	} else {
		counter = h.heartbeatErrorTotal
	}
	_ = stats.RecordWithTags(context.Background(), h.tagMutators, counter.M(1))
}

func (h *heartbeater) generateHeartbeatLog(buildInfo component.BuildInfo) plog.Logs {
	host, err := os.Hostname()
	if err != nil {
		host = "unknownhost"
	}

	ret := plog.NewLogs()
	resourceLogs := ret.ResourceLogs().AppendEmpty()

	resourceAttrs := resourceLogs.Resource().Attributes()
	resourceAttrs.PutStr(h.config.HecToOtelAttrs.Index, "_internal")
	resourceAttrs.PutStr(h.config.HecToOtelAttrs.Source, "otelcol")
	resourceAttrs.PutStr(h.config.HecToOtelAttrs.SourceType, "heartbeat")
	resourceAttrs.PutStr(h.config.HecToOtelAttrs.Host, host)

	logRecord := resourceLogs.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	logRecord.Body().SetStr(fmt.Sprintf(
		"HeartbeatInfo version=%s description=%s os=%s arch=%s",
		buildInfo.Version,
		buildInfo.Description,
		runtime.GOOS,
		runtime.GOARCH))
	return ret
}
