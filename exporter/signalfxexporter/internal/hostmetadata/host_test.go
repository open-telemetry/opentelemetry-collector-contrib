// Copyright OpenTelemetry Authors
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

// Taken from https://github.com/signalfx/golib/blob/master/metadata/hostmetadata/host_test.go
// with minor modifications.

package hostmetadata

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
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
			if (err != nil) != tt.wantErr {
				t.Errorf("getCPU() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			gotMap := gotInfo.toStringMap()
			for k, v := range tt.wantInfo {
				if gv, ok := gotMap[k]; !ok || v != gv {
					t.Errorf("getCPU() expected key '%s' was found %v.  Expected value '%s' and got '%s'.", k, ok, v, gv)
				}
			}
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
			if err := os.Setenv("HOST_ETC", tt.testfixtures.hostEtc); err != nil {
				t.Errorf("getOS() error = %v failed to set HOST_ETC env var", err)
				return
			}
			gotInfo, err := getOS()
			if (err != nil) != tt.wantErr {
				t.Errorf("getOS() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			gotMap := gotInfo.toStringMap()
			for k, v := range tt.wantInfo {
				if gv, ok := gotMap[k]; !ok || v != gv {
					t.Errorf("getOS() expected key '%s' was found %v.  Expected value '%s' and got '%s'.", k, ok, v, gv)
				}
			}
		})
		os.Unsetenv("HOST_ETC")
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
			if err := os.Setenv("HOST_ETC", tt.etc); err != nil {
				t.Errorf("getLinuxVersion() error = %v failed to set HOST_ETC env var", err)
				return
			}
			got, err := getLinuxVersion()
			if (err != nil) != tt.wantErr {
				t.Errorf("getLinuxVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getLinuxVersion() = %v, want %v", got, tt.want)
			}
		})
		os.Unsetenv("HOST_ETC")
	}
}

func TestGetMemory(t *testing.T) {
	tests := []struct {
		name             string
		memVirtualMemory func() (*mem.VirtualMemoryStat, error)
		want             map[string]string
		wantErr          bool
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
			if (err != nil) != tt.wantErr {
				t.Errorf("getMemory() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			got := mem.toStringMap()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getMemory() = %v, want %v", got, tt.want)
			}
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
				if err := os.Setenv("HOST_ETC", tt.etc); err != nil {
					t.Errorf("etcPath error = %v failed to set HOST_ETC env var", err)
					return
				}
			}
			if got := etcPath(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("etcPath = %v, want %v", got, tt.want)
			}
		})
		os.Unsetenv("HOST_ETC")
	}

}

func TestInt8ArrayToByteArray(t *testing.T) {
	type args struct {
		in []int8
	}
	tests := []struct {
		name string
		args args
		want []byte
	}{
		{
			name: "convert int8 array to byte array",
			args: args{
				in: []int8{72, 69, 76, 76, 79, 32, 87, 79, 82, 76, 68},
			},
			want: []byte{72, 69, 76, 76, 79, 32, 87, 79, 82, 76, 68},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := int8ArrayToByteArray(tt.args.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("int8ArrayToByteArray() = %v, want %v", got, tt.want)
			}
		})
	}
}
