// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Taken from https://github.com/signalfx/golib/blob/master/metadata/hostmetadata/host_test.go
// with minor modifications.

package hostmetadata

import (
	"context"
	"errors"
	"testing"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetCPU(t *testing.T) {
	type testfixture struct {
		cpuInfo   func(context.Context) ([]cpu.InfoStat, error)
		cpuCounts func(context.Context, bool) (int, error)
	}
	tests := []struct {
		name     string
		fixtures testfixture
		wantInfo map[string]string
		wantErr  bool
	}{
		{
			name: "successful host cpu info",
			fixtures: testfixture{
				cpuInfo: func(context.Context) ([]cpu.InfoStat, error) {
					return []cpu.InfoStat{
						{
							ModelName: "testmodelname",
							Cores:     4,
						},
						{
							ModelName: "testmodelname2",
							Cores:     4,
						},
					}, nil
				},
				cpuCounts: func(context.Context, bool) (int, error) {
					return 2, nil
				},
			},
			wantInfo: map[string]string{
				"host_physical_cpus": "2",
				"host_cpu_cores":     "8",
				"host_cpu_model":     "testmodelname2",
				"host_logical_cpus":  "2",
			},
		},
		{
			name: "unsuccessful host cpu info (missing cpu info)",
			fixtures: testfixture{
				cpuInfo: func(context.Context) ([]cpu.InfoStat, error) {
					return nil, errors.New("bad cpu info")
				},
			},
			wantInfo: map[string]string{
				"host_physical_cpus": "0",
				"host_cpu_cores":     "0",
				"host_cpu_model":     "",
				"host_logical_cpus":  "0",
			},
			wantErr: true,
		},
		{
			name: "unsuccessful host cpu info (missing cpu counts)",
			fixtures: testfixture{
				cpuInfo: func(context.Context) ([]cpu.InfoStat, error) {
					return []cpu.InfoStat{
						{
							ModelName: "testmodelname",
							Cores:     4,
						},
						{
							ModelName: "testmodelname2",
							Cores:     4,
						},
					}, nil
				},
				cpuCounts: func(context.Context, bool) (int, error) {
					return 0, errors.New("bad cpu counts")
				},
			},
			wantInfo: map[string]string{
				"host_physical_cpus": "2",
				"host_cpu_cores":     "0",
				"host_cpu_model":     "",
				"host_logical_cpus":  "0",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cpuInfo = tt.fixtures.cpuInfo
			cpuCounts = tt.fixtures.cpuCounts
			gotInfo, err := getCPU()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Subset(t, gotInfo.toStringMap(), tt.wantInfo)
		})
	}
}

func TestGetOS(t *testing.T) {
	type testfixture struct {
		hostInfo func() (*host.InfoStat, error)
		hostEtc  string
	}
	tests := []struct {
		name         string
		testfixtures testfixture
		wantInfo     map[string]string
		wantErr      bool
	}{
		{
			name: "get kernel info",
			testfixtures: testfixture{
				hostInfo: func() (*host.InfoStat, error) {
					return &host.InfoStat{
						OS:              "linux",
						KernelVersion:   "4.4.0-112-generic",
						Platform:        "ubuntu",
						PlatformVersion: "16.04",
					}, nil
				},
				hostEtc: "./testdata/lsb-release",
			},
			wantInfo: map[string]string{
				"host_kernel_name":    "linux",
				"host_kernel_release": "4.4.0-112-generic",
				"host_os_name":        "ubuntu",
			},
		},
		{
			name: "get kernel info error",
			testfixtures: testfixture{
				hostInfo: func() (*host.InfoStat, error) {
					return nil, errors.New("no host info")
				},
				hostEtc: "./testdata/lsb-release",
			},
			wantInfo: map[string]string{
				"host_kernel_name":    "",
				"host_kernel_version": "",
				"host_os_name":        "",
				"host_linux_version":  "",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hostInfo = tt.testfixtures.hostInfo
			t.Setenv("HOST_ETC", tt.testfixtures.hostEtc)
			gotInfo, err := getOS()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Subset(t, gotInfo.toStringMap(), tt.wantInfo)
		})
	}
}

func Test_GetLinuxVersion(t *testing.T) {
	tests := []struct {
		name    string
		etc     string
		want    string
		wantErr bool
	}{
		{
			name: "lsb-release",
			etc:  "./testdata/lsb-release",
			want: "Ubuntu 18.04 LTS",
		},
		{
			name: "os-release",
			etc:  "./testdata/os-release",
			want: "Debian GNU/Linux 9 (stretch)",
		},
		{
			name: "centos-release",
			etc:  "./testdata/centos-release",
			want: "CentOS Linux release 7.5.1804 (Core)",
		},
		{
			name: "redhat-release",
			etc:  "./testdata/redhat-release",
			want: "Red Hat Enterprise Linux Server release 7.5 (Maipo)",
		},
		{
			name: "system-release",
			etc:  "./testdata/system-release",
			want: "CentOS Linux release 7.5.1804 (Core)",
		},
		{
			name:    "no release returns error",
			etc:     "./testdata",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("HOST_ETC", tt.etc)
			got, err := getLinuxVersion()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetMemory(t *testing.T) {
	tests := []struct {
		name             string
		memVirtualMemory func() (*mem.VirtualMemoryStat, error)
		want             map[string]string
	}{
		{
			name: "host_mem_total",
			memVirtualMemory: func() (*mem.VirtualMemoryStat, error) {
				return &mem.VirtualMemoryStat{
					Total: 2048,
				}, nil
			},
			want: map[string]string{"host_mem_total": "2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			memVirtualMemory = tt.memVirtualMemory
			mem, err := getMemory()
			require.NoError(t, err)
			assert.Equal(t, tt.want, mem.toStringMap())
		})
	}
}

func TestEtcPath(t *testing.T) {
	tests := []struct {
		name string
		etc  string
		want string
	}{
		{
			name: "test default host etc",
			etc:  "",
			want: "/etc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.etc != "" {
				t.Setenv("HOST_ETC", tt.etc)
			}
			assert.Equal(t, tt.want, etcPath())
		})
	}

}
