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
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
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

	var telemetryBuilder *metadata.TelemetryBuilder
	var attrs attribute.Set
	if config.Telemetry.Enabled {
		// TODO: handle overrides, this could be done w/ views in configuration
		// overrides := config.Telemetry.OverrideMetricsNames
		extraAttributes := config.Telemetry.ExtraAttributes
		var attributes []attribute.KeyValue
		for key, val := range extraAttributes {
			attributes = append(attributes, attribute.String(key, val))
		}
		attrs = attribute.NewSet(attributes...)

		var err error
		if telemetryBuilder, err = metadata.NewTelemetryBuilder(); err != nil {
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
					observe(telemetryBuilder, attrs, err)
				}
			}
		}
	}()
	return hbter
}

func (h *heartbeater) shutdown() {
	close(h.hbDoneChan)
}

func (h *heartbeater) sendHeartbeat(config *Config, buildInfo component.BuildInfo, pushLogFn func(ctx context.Context, ld plog.Logs) error) error {
	return pushLogFn(context.Background(), generateHeartbeatLog(config.HecToOtelAttrs, buildInfo))
}

// there is only use case for open census metrics recording for now. Extend to use open telemetry in the future.
func observe(telemetryBuilder *metadata.TelemetryBuilder, attrs attribute.Set, err error) {
	if err == nil {
		telemetryBuilder.OtelcolExporterSplunkhecHeartbeatsSent.Add(context.Background(), 1, metric.WithAttributeSet(attrs))
	}
	telemetryBuilder.OtelcolExporterSplunkhecHeartbeatsFailed.Add(context.Background(), 1, metric.WithAttributeSet(attrs))
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
