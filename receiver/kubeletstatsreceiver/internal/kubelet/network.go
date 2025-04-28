// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubelet // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
)

type getNetworkDataFunc func(s *stats.NetworkStats) (rx *uint64, tx *uint64)

type getInterfaceDataFunc func(s *stats.InterfaceStats) (rx *uint64, tx *uint64)

func addNetworkMetrics(mb *metadata.MetricsBuilder, networkMetrics metadata.NetworkMetrics, s *stats.NetworkStats, currentTime pcommon.Timestamp, allInterfaces bool) {
	if s == nil {
		return
	}

	if allInterfaces {
		for i := range s.Interfaces {
			recordInterfaceDataPoint(mb, networkMetrics.IO, &s.Interfaces[i], getInterfaceIO, currentTime)
			recordInterfaceDataPoint(mb, networkMetrics.Errors, &s.Interfaces[i], getInterfaceErrors, currentTime)
		}
		// Because stats.NetworkStats.Interfaces contains metrics for all interfaces, including default,
		// we don't need to iterate over stats.NetworkStats.InterfaceStats for it, hence we return here
		return
	}

	recordNetworkDataPoint(mb, networkMetrics.IO, s, getNetworkIO, currentTime)
	recordNetworkDataPoint(mb, networkMetrics.Errors, s, getNetworkErrors, currentTime)
}

func recordNetworkDataPoint(mb *metadata.MetricsBuilder, recordDataPoint metadata.RecordIntDataPointWithDirectionFunc, s *stats.NetworkStats, getData getNetworkDataFunc, currentTime pcommon.Timestamp) {
	rx, tx := getData(s)

	if rx != nil {
		recordDataPoint(mb, currentTime, int64(*rx), s.Name, metadata.AttributeDirectionReceive)
	}

	if tx != nil {
		recordDataPoint(mb, currentTime, int64(*tx), s.Name, metadata.AttributeDirectionTransmit)
	}
}

func getNetworkIO(s *stats.NetworkStats) (*uint64, *uint64) {
	return s.RxBytes, s.TxBytes
}

func getNetworkErrors(s *stats.NetworkStats) (*uint64, *uint64) {
	return s.RxErrors, s.TxErrors
}

func recordInterfaceDataPoint(mb *metadata.MetricsBuilder, recordDataPoint metadata.RecordIntDataPointWithDirectionFunc, s *stats.InterfaceStats, getData getInterfaceDataFunc, currentTime pcommon.Timestamp) {
	rx, tx := getData(s)

	if rx != nil {
		recordDataPoint(mb, currentTime, int64(*rx), s.Name, metadata.AttributeDirectionReceive)
	}

	if tx != nil {
		recordDataPoint(mb, currentTime, int64(*tx), s.Name, metadata.AttributeDirectionTransmit)
	}
}

func getInterfaceIO(s *stats.InterfaceStats) (*uint64, *uint64) {
	return s.RxBytes, s.TxBytes
}

func getInterfaceErrors(s *stats.InterfaceStats) (*uint64, *uint64) {
	return s.RxErrors, s.TxErrors
}
