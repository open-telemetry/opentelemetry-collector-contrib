// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter"

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter/internal/metadata"
	translator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/splunk"
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

// heartbeatCounter returns the counter defined in metadata.yaml unless the user
// configured a custom name for it through telemetry.override_metrics_names, in
// which case a counter with the overridden name is created on the meter. The
// override mechanism is retained for backwards compatibility and is expected to
// be removed together with telemetry.enabled.
func heartbeatCounter(meter metric.Meter, defaultCounter metric.Int64Counter, overrides map[string]string, defaultName, description string) (metric.Int64Counter, error) {
	if name := getMetricsName(overrides, defaultName); name != defaultName {
		return meter.Int64Counter(
			name,
			metric.WithDescription(description),
			metric.WithUnit("1"),
		)
	}
	return defaultCounter, nil
}

func newHeartbeater(config *Config, buildInfo component.BuildInfo, pushLogFn func(ctx context.Context, ld plog.Logs) error, telemetryBuilder *metadata.TelemetryBuilder, meter metric.Meter) *heartbeater {
	interval := config.Heartbeat.Interval
	if interval == 0 {
		return nil
	}

	var heartbeatsSent, heartbeatsFailed metric.Int64Counter
	var attrs attribute.Set
	if config.Telemetry.Enabled {
		overrides := config.Telemetry.OverrideMetricsNames
		extraAttributes := config.Telemetry.ExtraAttributes
		var tags []attribute.KeyValue
		for key, val := range extraAttributes {
			tags = append(tags, attribute.String(key, val))
		}
		attrs = attribute.NewSet(tags...)
		var err error
		heartbeatsSent, err = heartbeatCounter(meter, telemetryBuilder.ExporterSplunkhecHeartbeatsSent, overrides, defaultHBSentMetricsName, "number of heartbeats sent")
		if err != nil {
			return nil
		}

		heartbeatsFailed, err = heartbeatCounter(meter, telemetryBuilder.ExporterSplunkhecHeartbeatsFailed, overrides, defaultHBFailedMetricsName, "number of heartbeats failed")
		if err != nil {
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
				err := hbter.sendHeartbeat(config, buildInfo, pushLogFn)
				if config.Telemetry.Enabled {
					observe(heartbeatsSent, heartbeatsFailed, attrs, err)
				}
			}
		}
	}()
	return hbter
}

func (h *heartbeater) shutdown() {
	close(h.hbDoneChan)
}

func (*heartbeater) sendHeartbeat(config *Config, buildInfo component.BuildInfo, pushLogFn func(ctx context.Context, ld plog.Logs) error) error {
	return pushLogFn(context.Background(), generateHeartbeatLog(config.OtelAttrsToHec, buildInfo))
}

// there is only use case for open census metrics recording for now. Extend to use open telemetry in the future.
func observe(heartbeatsSent, heartbeatsFailed metric.Int64Counter, attrs attribute.Set, err error) {
	if err == nil {
		heartbeatsSent.Add(context.Background(), 1, metric.WithAttributeSet(attrs))
	} else {
		heartbeatsFailed.Add(context.Background(), 1, metric.WithAttributeSet(attrs))
	}
}

func generateHeartbeatLog(hecToOtelAttrs translator.HecToOtelAttrs, buildInfo component.BuildInfo) plog.Logs {
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
		runtime.GOARCH,
	))
	return ret
}
