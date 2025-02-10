// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hostmetadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/hostmetadata"

import (
	"context"
	"time"

	"github.com/DataDog/opentelemetry-mapping-go/pkg/inframetadata"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes/source"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/hostmetadata"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func GetSourceProvider(set component.TelemetrySettings, configHostname string, timeout time.Duration) (source.Provider, error) {
	return hostmetadata.GetSourceProvider(set, configHostname, timeout)
}

type PusherConfig = hostmetadata.PusherConfig

func NewPusher(params exporter.Settings, pcfg PusherConfig) inframetadata.Pusher {
	return hostmetadata.NewPusher(params, hostmetadata.PusherConfig(pcfg))
}

func RunPusher(ctx context.Context, params exporter.Settings, pcfg PusherConfig, p source.Provider, attrs pcommon.Map, reporter *inframetadata.Reporter) {
	hostmetadata.RunPusher(ctx, params, pcfg, p, attrs, reporter)
}
