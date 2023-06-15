// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows
// +build !windows

package podmanreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/podmanreceiver"

import "time"

type container struct {
	AutoRemove bool
	Command    []string
	Created    string
	CreatedAt  string
	ExitCode   int
	Exited     bool
	ExitedAt   int
	ID         string
	Image      string
	ImageID    string
	IsInfra    bool
	Labels     map[string]string
	Mounts     []string
	Names      []string
	Namespaces map[string]string
	Networks   []string
	Pid        int
	Pod        string
	PodName    string
	Ports      []map[string]interface{}
	Size       map[string]string
	StartedAt  int
	State      string
	Status     string
}

type event struct {
	ID     string
	Status string
}

type containerStats struct {
	AvgCPU        float64
	ContainerID   string
	Name          string
	PerCPU        []uint64
	CPU           float64
	CPUNano       uint64
	CPUSystemNano uint64
	DataPoints    int64
	SystemNano    uint64
	MemUsage      uint64
	MemLimit      uint64
	MemPerc       float64
	NetInput      uint64
	NetOutput     uint64
	BlockInput    uint64
	BlockOutput   uint64
	PIDs          uint64
	UpTime        time.Duration
	Duration      uint64
}

type containerStatsReportError struct {
	Cause    string
	Message  string
	Response int64
}

type containerStatsReport struct {
	Error containerStatsReportError
	Stats []containerStats
}
