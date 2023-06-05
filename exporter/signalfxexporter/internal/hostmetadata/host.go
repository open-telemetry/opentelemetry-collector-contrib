// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Taken from https://github.com/signalfx/golib/blob/master/metadata/hostmetadata/host.go
// with minor modifications.

package hostmetadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/hostmetadata"

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
)

// etcPath is the path to host etc and can be set using the env var "HOST_ETC"
// this is to maintain consistency with gopsutil
var etcPath = func() string {
	if etcPath := os.Getenv("HOST_ETC"); etcPath != "" {
		return etcPath
	}
	return "/etc"
}

const cpuStatsTimeout = 10 * time.Second

// Map library functions to unexported package variables for testing purposes.
var cpuInfo = cpu.InfoWithContext
var cpuCounts = cpu.CountsWithContext
var memVirtualMemory = mem.VirtualMemory
var hostInfo = host.Info

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
func getCPU() (info *hostCPU, err error) {
	info = &hostCPU{}

	// get physical cpu stats
	var cpus []cpu.InfoStat

	// On Windows this can sometimes take longer than the default timeout (10 seconds).
	ctx, cancel := context.WithTimeout(context.Background(), cpuStatsTimeout)
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
func getOS() (info *hostOS, err error) {
	info = &hostOS{}
	hInfo, err := hostInfo()
	if err != nil {
		return info, err
	}

	info.HostOSName = hInfo.Platform
	info.HostKernelName = hInfo.OS
	// in gopsutil KernelVersion returns what we would expect for Kernel Release
	info.HostKernelRelease = hInfo.KernelVersion
	err = fillPlatformSpecificOSData(info)
	return info, err
}

// getLinuxVersion - adds information about the host linux version to the supplied map
func getLinuxVersion() (string, error) {
	etc := etcPath()
	if value, err := getStringFromFile(`DISTRIB_DESCRIPTION="(.*)"`, filepath.Join(etc, "lsb-release")); err == nil {
		return value, nil
	}
	if value, err := getStringFromFile(`PRETTY_NAME="(.*)"`, filepath.Join(etc, "os-release")); err == nil {
		return value, nil
	}
	if value, err := os.ReadFile(filepath.Join(etc, "centos-release")); err == nil {
		return string(value), nil
	}
	if value, err := os.ReadFile(filepath.Join(etc, "redhat-release")); err == nil {
		return string(value), nil
	}
	if value, err := os.ReadFile(filepath.Join(etc, "system-release")); err == nil {
		return string(value), nil
	}
	return "", errors.New("unable to find linux version")
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
func getMemory() (*Memory, error) {
	m := &Memory{}
	memoryStat, err := memVirtualMemory()
	if err == nil {
		m.Total = int(memoryStat.Total)
	}
	return m, err
}

func getStringFromFile(pattern string, path string) (string, error) {
	var err error
	var file []byte
	var reg = regexp.MustCompile(pattern)
	if file, err = os.ReadFile(path); err == nil {
		if match := reg.FindSubmatch(file); len(match) > 1 {
			return string(match[1]), nil
		}
	}
	return "", err
}

func bytesToKilobytes(b int) int {
	return b / 1024
}
