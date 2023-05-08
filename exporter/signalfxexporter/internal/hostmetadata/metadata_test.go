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

package hostmetadata

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
	metadata "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata"
)

func TestSyncMetadata(t *testing.T) {
	tests := []struct {
		name               string
		cpuStat            cpu.InfoStat
		cpuStatErr         error
		memStat            mem.VirtualMemoryStat
		memStatErr         error
		hostStat           host.InfoStat
		hostStatErr        error
		pushFail           bool
		metricsData        pmetric.Metrics
		wantMetadataUpdate []*metadata.MetadataUpdate
		wantLogs           []string
	}{
		{
			name: "all_stats_available",
			cpuStat: cpu.InfoStat{
				Cores:     4,
				ModelName: "testprocessor",
			},
			cpuStatErr: nil,
			memStat: mem.VirtualMemoryStat{
				Total: 2048,
			},
			memStatErr: nil,
			hostStat: host.InfoStat{
				OS:            "linux",
				Platform:      "linux1",
				KernelVersion: "4.14.121",
			},
			hostStatErr: nil,
			pushFail:    false,
			metricsData: generateSampleMetricsData(map[string]string{conventions.AttributeHostName: "host1"}),
			wantMetadataUpdate: []*metadata.MetadataUpdate{
				{
					ResourceIDKey: conventions.AttributeHostName,
					ResourceID:    "host1",
					MetadataDelta: metadata.MetadataDelta{
						MetadataToUpdate: map[string]string{
							"host_cpu_cores":      "4",
							"host_cpu_model":      "testprocessor",
							"host_logical_cpus":   "1",
							"host_physical_cpus":  "1",
							"host_processor":      "",
							"host_machine":        "",
							"host_mem_total":      "2",
							"host_kernel_name":    "linux",
							"host_kernel_release": "4.14.121",
							"host_linux_version":  "",
							"host_kernel_version": "",
							"host_os_name":        "linux1",
						},
					},
				},
			},
			wantLogs: []string{},
		},
		{
			name: "no_host_stats",
			cpuStat: cpu.InfoStat{
				Cores:     4,
				ModelName: "testprocessor",
			},
			cpuStatErr: nil,
			memStat: mem.VirtualMemoryStat{
				Total: 2048,
			},
			memStatErr:  nil,
			hostStat:    host.InfoStat{},
			hostStatErr: errors.New("failed"),
			pushFail:    false,
			metricsData: generateSampleMetricsData(map[string]string{conventions.AttributeHostName: "host1"}),
			wantMetadataUpdate: []*metadata.MetadataUpdate{
				{
					ResourceIDKey: conventions.AttributeHostName,
					ResourceID:    "host1",
					MetadataDelta: metadata.MetadataDelta{
						MetadataToUpdate: map[string]string{
							"host_cpu_cores":     "4",
							"host_cpu_model":     "testprocessor",
							"host_logical_cpus":  "1",
							"host_physical_cpus": "1",
							"host_processor":     "",
							"host_machine":       "",
							"host_mem_total":     "2",
						},
					},
				},
			},
			wantLogs: []string{"Failed to scrape host hostOS metadata"},
		},
		{
			name:       "mem_stats_only",
			cpuStat:    cpu.InfoStat{},
			cpuStatErr: errors.New("failed"),
			memStat: mem.VirtualMemoryStat{
				Total: 2048,
			},
			memStatErr:  nil,
			hostStat:    host.InfoStat{},
			hostStatErr: errors.New("failed"),
			pushFail:    false,
			metricsData: generateSampleMetricsData(map[string]string{conventions.AttributeHostName: "host1"}),
			wantMetadataUpdate: []*metadata.MetadataUpdate{
				{
					ResourceIDKey: conventions.AttributeHostName,
					ResourceID:    "host1",
					MetadataDelta: metadata.MetadataDelta{
						MetadataToUpdate: map[string]string{
							"host_mem_total": "2",
						},
					},
				},
			},
			wantLogs: []string{
				"Failed to scrape host hostCPU metadata",
				"Failed to scrape host hostOS metadata",
			},
		},
		{
			name:               "no_stats",
			cpuStat:            cpu.InfoStat{},
			cpuStatErr:         errors.New("failed"),
			memStat:            mem.VirtualMemoryStat{},
			memStatErr:         errors.New("failed"),
			hostStat:           host.InfoStat{},
			hostStatErr:        errors.New("failed"),
			metricsData:        generateSampleMetricsData(map[string]string{conventions.AttributeHostName: "host1"}),
			wantMetadataUpdate: nil,
			wantLogs: []string{
				"Failed to scrape host hostCPU metadata",
				"Failed to scrape host memory metadata",
				"Failed to scrape host hostOS metadata",
				"Failed to fetch system properties. Host metadata synchronization skipped",
			},
		},
		{
			name: "failed_push",
			cpuStat: cpu.InfoStat{
				Cores: 1,
			},
			cpuStatErr: nil,
			memStat:    mem.VirtualMemoryStat{},
			memStatErr: nil,
			hostStat: host.InfoStat{
				OS: "linux",
			},
			hostStatErr:        nil,
			pushFail:           true,
			metricsData:        generateSampleMetricsData(map[string]string{conventions.AttributeHostName: "host1"}),
			wantMetadataUpdate: nil,
			wantLogs:           []string{"Failed to push host metadata update"},
		},
		{
			name:       "mem_stats_on_gcp",
			cpuStat:    cpu.InfoStat{},
			cpuStatErr: errors.New("failed"),
			memStat: mem.VirtualMemoryStat{
				Total: 2048,
			},
			memStatErr:  nil,
			hostStat:    host.InfoStat{},
			hostStatErr: errors.New("failed"),
			pushFail:    false,
			metricsData: generateSampleMetricsData(map[string]string{
				conventions.AttributeCloudProvider:  conventions.AttributeCloudProviderGCP,
				conventions.AttributeCloudAccountID: "1234",
				conventions.AttributeHostID:         "i-abc",
			}),
			wantMetadataUpdate: []*metadata.MetadataUpdate{
				{
					ResourceIDKey: string(splunk.HostIDKeyGCP),
					ResourceID:    "1234_i-abc",
					MetadataDelta: metadata.MetadataDelta{
						MetadataToUpdate: map[string]string{
							"host_mem_total": "2",
						},
					},
				},
			},
			wantLogs: []string{
				"Failed to scrape host hostCPU metadata",
				"Failed to scrape host hostOS metadata",
			},
		},
		{
			name:     "no_host_id_attrs",
			cpuStat:  cpu.InfoStat{},
			pushFail: false,
			metricsData: generateSampleMetricsData(map[string]string{
				"random": "attribute",
			}),
			wantMetadataUpdate: nil,
			wantLogs: []string{
				"Not found any host attributes. Host metadata synchronization skipped. " +
					"Make sure that \"resourcedetection\" processor is enabled in the pipeline with one of " +
					"the cloud provider detectors or environment variable detector setting \"host.name\" attribute",
			},
		},
		{
			name:               "empty_metrics_data",
			pushFail:           false,
			metricsData:        pmetric.NewMetrics(),
			wantMetadataUpdate: nil,
			wantLogs:           []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			observedLogger, logs := observer.New(zapcore.WarnLevel)
			logger := zap.New(observedLogger)
			dimClient := &fakeDimClient{fail: tt.pushFail}
			syncer := NewSyncer(logger, dimClient)

			// mock system stats calls.
			t.Setenv("HOST_ETC", ".")
			cpuInfo = func(context.Context) ([]cpu.InfoStat, error) {
				return []cpu.InfoStat{tt.cpuStat}, tt.cpuStatErr
			}
			cpuCounts = func(context.Context, bool) (int, error) { return 1, nil }
			memVirtualMemory = func() (*mem.VirtualMemoryStat, error) {
				return &tt.memStat, tt.memStatErr
			}
			hostInfo = func() (*host.InfoStat, error) {
				return &tt.hostStat, tt.hostStatErr
			}
			mockSyscallUname()

			syncer.Sync(tt.metricsData)

			if tt.wantMetadataUpdate != nil {
				require.Equal(t, 1, len(dimClient.getMetadataUpdates()))
				require.EqualValues(t, tt.wantMetadataUpdate, dimClient.getMetadataUpdates()[0])
			} else {
				require.Equal(t, 0, len(dimClient.getMetadataUpdates()))
			}

			require.Equal(t, len(tt.wantLogs), logs.Len())
			for i, log := range logs.All() {
				assert.Equal(t, tt.wantLogs[i], log.Message)
			}

		})
	}
}

type fakeDimClient struct {
	sync.Mutex
	fail            bool
	metadataUpdates [][]*metadata.MetadataUpdate
}

func (dc *fakeDimClient) PushMetadata(metadataUpdates []*metadata.MetadataUpdate) error {
	if dc.fail {
		return errors.New("failed")
	}
	dc.Lock()
	defer dc.Unlock()
	dc.metadataUpdates = append(dc.metadataUpdates, metadataUpdates)
	return nil
}

func (dc *fakeDimClient) getMetadataUpdates() [][]*metadata.MetadataUpdate {
	dc.Lock()
	defer dc.Unlock()
	return dc.metadataUpdates
}

func generateSampleMetricsData(attrs map[string]string) pmetric.Metrics {
	m := pmetric.NewMetrics()
	rm := m.ResourceMetrics()
	res := rm.AppendEmpty().Resource()
	for k, v := range attrs {
		res.Attributes().PutStr(k, v)
	}
	return m
}
