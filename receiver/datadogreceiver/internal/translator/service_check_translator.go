// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver/internal/translator"

import (
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV1"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
)

type ServiceCheck struct {
	Check     string                       `json:"check"`
	HostName  string                       `json:"host_name"`
	Status    datadogV1.ServiceCheckStatus `json:"status"`
	Timestamp int64                        `json:"timestamp,omitempty"`
	Tags      []string                     `json:"tags,omitempty"`
}

// More information on Datadog service checks: https://docs.datadoghq.com/api/latest/service-checks/
func (mt *MetricsTranslator) TranslateServices(services []ServiceCheck) pmetric.Metrics {
	bt := newBatcher()
	bt.Metrics = pmetric.NewMetrics()

	for _, service := range services {
		metricProperties := parseSeriesProperties(service.Check, "service_check", service.Tags, service.HostName, mt.buildInfo.Version, mt.stringPool)
		metric, metricID := bt.Lookup(metricProperties) // TODO(alexg): proper name

		dps := metric.Gauge().DataPoints()
		dps.EnsureCapacity(1)

		dp := dps.AppendEmpty()
		dp.SetTimestamp(pcommon.Timestamp(service.Timestamp * time.Second.Nanoseconds())) // OTel uses nanoseconds, while Datadog uses seconds
		metricProperties.dpAttrs.CopyTo(dp.Attributes())
		dp.SetIntValue(int64(service.Status))

		// TODO(alexg): Do this stream thing for service check metrics?
		stream := identity.OfStream(metricID, dp)
		ts, ok := mt.streamHasTimestamp(stream)
		if ok {
			dp.SetStartTimestamp(ts)
		}
		mt.updateLastTsForStream(stream, dp.Timestamp())
	}
	return bt.Metrics
}
