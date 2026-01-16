// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Taken from https://github.com/signalfx/golib/blob/master/metadata/hostmetadata/host.go
// with minor modifications.

package hostmetadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/hostmetadata"

import (
	"context"
	"strconv"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
)

const cpuStatsTimeout = 10 * time.Second

// Map library functions to unexported package variables for testing purposes.
var (
	cpuInfo          = cpu.InfoWithContext
	cpuCounts        = cpu.CountsWithContext
	memVirtualMemory = mem.VirtualMemoryWithContext
	hostInfo         = host.InfoWithContext
)

// hostCPU information about the host
type hostCPU struct {
	HostPhysicalCPUs int
	HostLogicalCPUs  int
	HostCPUCores     int64
	HostCPUModel     string
	HostMachine      string
	HostProcessor    string
}

// toStringMap returns the hostCPU as a string map
func (c *hostCPU) toStringMap() map[string]string {
	return map[string]string{
		"host_physical_cpus": strconv.Itoa(c.HostPhysicalCPUs),
		"host_cpu_cores":     strconv.FormatInt(c.HostCPUCores, 10),
		"host_cpu_model":     c.HostCPUModel,
		"host_logical_cpus":  strconv.Itoa(c.HostLogicalCPUs),
		"host_processor":     c.HostProcessor,
		"host_machine":       c.HostMachine,
	}
}

// getCPU - adds information about the host cpu to the supplied map
func getCPU(ctx context.Context) (info *hostCPU, err error) {
	info = &hostCPU{}

	// get physical cpu stats
	var cpus []cpu.InfoStat

	// On Windows this can sometimes take longer than the default timeout (10 seconds).
	ctx, cancel := context.WithTimeout(ctx, cpuStatsTimeout)
	defer cancel()

	cpus, err = cpuInfo(ctx)
	if err != nil {
		return info, err
	}

	info.HostPhysicalCPUs = len(cpus)

	// get logical cpu stats
	info.HostLogicalCPUs, err = cpuCounts(ctx, true)
	if err != nil {
		return info, err
	}

	// total number of cpu cores
	for i := range cpus {
		info.HostCPUCores += int64(cpus[i].Cores)
		// TODO: This is not ideal... if there are different processors
		// we will only report one of the models... This is unlikely to happen,
		// but it could
		info.HostCPUModel = cpus[i].ModelName
	}

	err = fillPlatformSpecificCPUData(info)
	return info, err
}

// hostOS is a struct containing information about the host os
type hostOS struct {
	HostOSName        string
	HostKernelName    string
	HostKernelRelease string
	HostKernelVersion string
	HostLinuxVersion  string
}

// toStringMap returns a map of key/value metadata about the host os
func (o *hostOS) toStringMap() map[string]string {
	return map[string]string{
		"host_kernel_name":    o.HostKernelName,
		"host_kernel_release": o.HostKernelRelease,
		"host_kernel_version": o.HostKernelVersion,
		"host_os_name":        o.HostOSName,
		"host_linux_version":  o.HostLinuxVersion,
	}
}

// getOS returns a struct with information about the host os
func getOS(ctx context.Context) (info *hostOS, err error) {
	info = &hostOS{}
	hInfo, err := hostInfo(ctx)
	if err != nil {
		return info, err
	}

	info.HostOSName = hInfo.Platform
	info.HostKernelName = hInfo.OS
	// in gopsutil KernelVersion returns what we would expect for Kernel Release
	info.HostKernelRelease = hInfo.KernelVersion
	err = fillPlatformSpecificOSData(ctx, info)
	return info, err
}

// Memory stores memory collected from the host
type Memory struct {
	Total int
}

// toStringMap returns a map of key/value metadata about the host memory
// where memory sizes are reported in Kb
func (m *Memory) toStringMap() map[string]string {
	return map[string]string{
		"host_mem_total": strconv.Itoa(bytesToKilobytes(m.Total)),
	}
}

// getMemory returns the amount of memory on the host as datatype.USize
func getMemory(ctx context.Context) (*Memory, error) {
	m := &Memory{}
	memoryStat, err := memVirtualMemory(ctx)
	if err == nil {
		m.Total = int(memoryStat.Total)
	}
	return m, err
}

func bytesToKilobytes(b int) int {
	return b / 1024
}
