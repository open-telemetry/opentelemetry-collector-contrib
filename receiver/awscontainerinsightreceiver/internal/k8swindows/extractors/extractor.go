// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package extractors // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/k8swindows/extractors"

import (
	"time"

	"github.com/Microsoft/hcsshim"

	cExtractor "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/cadvisor/extractors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver/internal/stores"
)

// CPUStat for Pod, Container and Node.
type CPUStat struct {
	Time                 time.Time
	UsageNanoCores       uint64
	UsageCoreNanoSeconds uint64
	UserCPUUsage         uint64
	SystemCPUUsage       uint64
}

// MemoryStat for Pod, Container and Node
type MemoryStat struct {
	Time            time.Time
	AvailableBytes  uint64
	UsageBytes      uint64
	WorkingSetBytes uint64
	RSSBytes        uint64
	PageFaults      uint64
	MajorPageFaults uint64
}

// FileSystemStat for Container and Node.
type FileSystemStat struct {
	Time           time.Time
	Name           string
	Device         string
	Type           string
	AvailableBytes uint64
	CapacityBytes  uint64
	UsedBytes      uint64
}

// NetworkStat for Pod and Node.
type NetworkStat struct {
	Time            time.Time
	Name            string
	RxBytes         uint64
	RxErrors        uint64
	TxBytes         uint64
	TxErrors        uint64
	DroppedIncoming uint64
	DroppedOutgoing uint64
}

// HCSNetworkStat Network Stat from HCS.
type HCSNetworkStat struct {
	Name                   string
	BytesReceived          uint64
	BytesSent              uint64
	DroppedPacketsIncoming uint64
	DroppedPacketsOutgoing uint64
}

// HCSStat Stats from HCS.
type HCSStat struct {
	Time time.Time
	Id   string //nolint:revive
	Name string

	CPU *hcsshim.ProcessorStats

	Network *[]HCSNetworkStat
}

// RawMetric Represent Container, Pod, Node Metric  Extractors.
// Kubelet summary and HNS stats will be converted to Raw Metric for parsing by Extractors.
type RawMetric struct {
	Id              string //nolint:revive
	Name            string
	Namespace       string
	Time            time.Time
	CPUStats        CPUStat
	MemoryStats     MemoryStat
	FileSystemStats []FileSystemStat
	NetworkStats    []NetworkStat
}

type MetricExtractor interface {
	HasValue(summary RawMetric) bool
	GetValue(summary RawMetric, mInfo cExtractor.CPUMemInfoProvider, containerType string) []*stores.CIMetricImpl
	Shutdown() error
}
