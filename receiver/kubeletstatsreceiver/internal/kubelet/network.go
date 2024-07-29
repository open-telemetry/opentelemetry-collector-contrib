// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubelet // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
)

type getNetworkDataFunc func(s *stats.InterfaceStats) (rx *uint64, tx *uint64)

func addNetworkMetrics(mb *metadata.MetricsBuilder, networkMetrics metadata.NetworkMetrics, s *stats.NetworkStats, currentTime pcommon.Timestamp) {
	if s == nil {
		return
	}

	for i := range s.Interfaces {
		recordNetworkDataPoint(mb, networkMetrics.IO, &s.Interfaces[i], getNetworkIO, currentTime)
		recordNetworkDataPoint(mb, networkMetrics.Errors, &s.Interfaces[i], getNetworkErrors, currentTime)
	}
}

func recordNetworkDataPoint(mb *metadata.MetricsBuilder, recordDataPoint metadata.RecordIntDataPointWithDirectionFunc, s *stats.InterfaceStats, getData getNetworkDataFunc, currentTime pcommon.Timestamp) {
	rx, tx := getData(s)

	if rx != nil {
		recordDataPoint(mb, currentTime, int64(*rx), s.Name, metadata.AttributeDirectionReceive)
	}

	if tx != nil {
		recordDataPoint(mb, currentTime, int64(*tx), s.Name, metadata.AttributeDirectionTransmit)
	}
}

func getNetworkIO(s *stats.InterfaceStats) (*uint64, *uint64) {
	return s.RxBytes, s.TxBytes
}

func getNetworkErrors(s *stats.InterfaceStats) (*uint64, *uint64) {
	return s.RxErrors, s.TxErrors
}
