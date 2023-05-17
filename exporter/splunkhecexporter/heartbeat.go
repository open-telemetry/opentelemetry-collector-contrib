// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter"

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

const (
	metricsPrefix              = "otelcol_exporter_splunkhec_"
	defaultHBSentMetricsName   = metricsPrefix + "heartbeats_sent"
	defaultHBFailedMetricsName = metricsPrefix + "heartbeats_failed"
)

type heartbeater struct {
	hbDoneChan chan struct{}
}

func getMetricsName(overrides map[string]string, metricName string) string {
	if name, ok := overrides[metricName]; ok {
		return name
	}
	return metricName
}

func newHeartbeater(config *Config, buildInfo component.BuildInfo, pushLogFn func(ctx context.Context, ld plog.Logs) error) *heartbeater {
	interval := config.Heartbeat.Interval
	if interval == 0 {
		return nil
	}

	var heartbeatsSent, heartbeatsFailed *stats.Int64Measure
	var tagMutators []tag.Mutator
	if config.Telemetry.Enabled {
		overrides := config.Telemetry.OverrideMetricsNames
		extraAttributes := config.Telemetry.ExtraAttributes
		var tags []tag.Key
		tagMutators = []tag.Mutator{}
		for key, val := range extraAttributes {
			newTag, _ := tag.NewKey(key)
			tags = append(tags, newTag)
			tagMutators = append(tagMutators, tag.Insert(newTag, val))
		}

		heartbeatsSent = stats.Int64(
			getMetricsName(overrides, defaultHBSentMetricsName),
			"number of heartbeats sent",
			stats.UnitDimensionless)

		heartbeatsSentView := &view.View{
			Name:        heartbeatsSent.Name(),
			Description: heartbeatsSent.Description(),
			TagKeys:     tags,
			Measure:     heartbeatsSent,
			Aggregation: view.Sum(),
		}

		heartbeatsFailed = stats.Int64(
			getMetricsName(overrides, defaultHBFailedMetricsName),
			"number of heartbeats failed",
			stats.UnitDimensionless)

		heartbeatsFailedView := &view.View{
			Name:        heartbeatsFailed.Name(),
			Description: heartbeatsFailed.Description(),
			TagKeys:     tags,
			Measure:     heartbeatsFailed,
			Aggregation: view.Sum(),
		}

		if err := view.Register(heartbeatsSentView, heartbeatsFailedView); err != nil {
			return nil
		}
	}

	hbter := &heartbeater{
		hbDoneChan: make(chan struct{}),
	}

	go func() {
		ticker := time.NewTicker(interval)
		for {
			select {
			case <-hbter.hbDoneChan:
				return
			case <-ticker.C:
				err := pushLogFn(context.Background(), generateHeartbeatLog(config.HecToOtelAttrs, buildInfo))
				if config.Telemetry.Enabled {
					observe(heartbeatsSent, heartbeatsFailed, tagMutators, err)
				}
			}
		}
	}()
	return hbter
}

func (h *heartbeater) shutdown() {
	close(h.hbDoneChan)
}

// there is only use case for open census metrics recording for now. Extend to use open telemetry in the future.
func observe(heartbeatsSent *stats.Int64Measure, heartbeatsFailed *stats.Int64Measure, tagMutators []tag.Mutator, err error) {
	var counter *stats.Int64Measure
	if err == nil {
		counter = heartbeatsSent
	} else {
		counter = heartbeatsFailed
	}
	_ = stats.RecordWithTags(context.Background(), tagMutators, counter.M(1))
}

func generateHeartbeatLog(hecToOtelAttrs splunk.HecToOtelAttrs, buildInfo component.BuildInfo) plog.Logs {
	host, err := os.Hostname()
	if err != nil {
		host = "unknownhost"
	}

	ret := plog.NewLogs()
	resourceLogs := ret.ResourceLogs().AppendEmpty()

	resourceAttrs := resourceLogs.Resource().Attributes()
	resourceAttrs.PutStr(hecToOtelAttrs.Index, "_internal")
	resourceAttrs.PutStr(hecToOtelAttrs.Source, "otelcol")
	resourceAttrs.PutStr(hecToOtelAttrs.SourceType, "heartbeat")
	resourceAttrs.PutStr(hecToOtelAttrs.Host, host)

	logRecord := resourceLogs.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	logRecord.Body().SetStr(fmt.Sprintf(
		"HeartbeatInfo version=%s description=%s os=%s arch=%s",
		buildInfo.Version,
		buildInfo.Description,
		runtime.GOOS,
		runtime.GOARCH))
	return ret
}
