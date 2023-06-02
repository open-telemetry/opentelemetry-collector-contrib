// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build !windows
// +build !windows

// TODO review if tests should succeed on Windows

package dockerstatsreceiver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver/internal/metadata"
)

var mockFolder = filepath.Join("testdata", "mock")

var (
	metricEnabled     = metadata.MetricConfig{Enabled: true}
	allMetricsEnabled = metadata.MetricsConfig{
		ContainerBlockioIoMergedRecursive:          metricEnabled,
		ContainerBlockioIoQueuedRecursive:          metricEnabled,
		ContainerBlockioIoServiceBytesRecursive:    metricEnabled,
		ContainerBlockioIoServiceTimeRecursive:     metricEnabled,
		ContainerBlockioIoServicedRecursive:        metricEnabled,
		ContainerBlockioIoTimeRecursive:            metricEnabled,
		ContainerBlockioIoWaitTimeRecursive:        metricEnabled,
		ContainerBlockioSectorsRecursive:           metricEnabled,
		ContainerCPUPercent:                        metricEnabled,
		ContainerCPUThrottlingDataPeriods:          metricEnabled,
		ContainerCPUThrottlingDataThrottledPeriods: metricEnabled,
		ContainerCPUThrottlingDataThrottledTime:    metricEnabled,
		ContainerCPUUsageKernelmode:                metricEnabled,
		ContainerCPUUsagePercpu:                    metricEnabled,
		ContainerCPUUsageSystem:                    metricEnabled,
		ContainerCPUUsageTotal:                     metricEnabled,
		ContainerCPUUsageUsermode:                  metricEnabled,
		ContainerMemoryActiveAnon:                  metricEnabled,
		ContainerMemoryActiveFile:                  metricEnabled,
		ContainerMemoryCache:                       metricEnabled,
		ContainerMemoryDirty:                       metricEnabled,
		ContainerMemoryHierarchicalMemoryLimit:     metricEnabled,
		ContainerMemoryHierarchicalMemswLimit:      metricEnabled,
		ContainerMemoryInactiveAnon:                metricEnabled,
		ContainerMemoryInactiveFile:                metricEnabled,
		ContainerMemoryMappedFile:                  metricEnabled,
		ContainerMemoryPercent:                     metricEnabled,
		ContainerMemoryPgfault:                     metricEnabled,
		ContainerMemoryPgmajfault:                  metricEnabled,
		ContainerMemoryPgpgin:                      metricEnabled,
		ContainerMemoryPgpgout:                     metricEnabled,
		ContainerMemoryRss:                         metricEnabled,
		ContainerMemoryRssHuge:                     metricEnabled,
		ContainerMemorySwap:                        metricEnabled,
		ContainerMemoryTotalActiveAnon:             metricEnabled,
		ContainerMemoryTotalActiveFile:             metricEnabled,
		ContainerMemoryTotalCache:                  metricEnabled,
		ContainerMemoryTotalDirty:                  metricEnabled,
		ContainerMemoryTotalInactiveAnon:           metricEnabled,
		ContainerMemoryTotalInactiveFile:           metricEnabled,
		ContainerMemoryTotalMappedFile:             metricEnabled,
		ContainerMemoryTotalPgfault:                metricEnabled,
		ContainerMemoryTotalPgmajfault:             metricEnabled,
		ContainerMemoryTotalPgpgin:                 metricEnabled,
		ContainerMemoryTotalPgpgout:                metricEnabled,
		ContainerMemoryTotalRss:                    metricEnabled,
		ContainerMemoryTotalRssHuge:                metricEnabled,
		ContainerMemoryTotalSwap:                   metricEnabled,
		ContainerMemoryTotalUnevictable:            metricEnabled,
		ContainerMemoryTotalWriteback:              metricEnabled,
		ContainerMemoryUnevictable:                 metricEnabled,
		ContainerMemoryUsageLimit:                  metricEnabled,
		ContainerMemoryUsageMax:                    metricEnabled,
		ContainerMemoryUsageTotal:                  metricEnabled,
		ContainerMemoryWriteback:                   metricEnabled,
		ContainerNetworkIoUsageRxBytes:             metricEnabled,
		ContainerNetworkIoUsageRxDropped:           metricEnabled,
		ContainerNetworkIoUsageRxErrors:            metricEnabled,
		ContainerNetworkIoUsageRxPackets:           metricEnabled,
		ContainerNetworkIoUsageTxBytes:             metricEnabled,
		ContainerNetworkIoUsageTxDropped:           metricEnabled,
		ContainerNetworkIoUsageTxErrors:            metricEnabled,
		ContainerNetworkIoUsageTxPackets:           metricEnabled,
	}
)

func TestNewReceiver(t *testing.T) {
	cfg := &Config{
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			CollectionInterval: 1 * time.Second,
		},
		Endpoint:         "unix:///run/some.sock",
		DockerAPIVersion: defaultDockerAPIVersion,
	}
	mr := newReceiver(receivertest.NewNopCreateSettings(), cfg)
	assert.NotNil(t, mr)
}

func TestErrorsInStart(t *testing.T) {
	unreachable := "unix:///not/a/thing.sock"
	cfg := &Config{
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			CollectionInterval: 1 * time.Second,
		},
		Endpoint:         unreachable,
		DockerAPIVersion: defaultDockerAPIVersion,
	}
	recv := newReceiver(receivertest.NewNopCreateSettings(), cfg)
	assert.NotNil(t, recv)

	cfg.Endpoint = "..not/a/valid/endpoint"
	err := recv.start(context.Background(), componenttest.NewNopHost())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unable to parse docker host")

	cfg.Endpoint = unreachable
	err = recv.start(context.Background(), componenttest.NewNopHost())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestScrapeV2(t *testing.T) {
	containerIDs := []string{
		"10b703fb312b25e8368ab5a3bce3a1610d1cee5d71a94920f1a7adbc5b0cb326",
		"89d28931fd8b95c8806343a532e9e76bf0a0b76ee8f19452b8f75dee1ebcebb7",
		"a359c0fc87c546b42d2ad32db7c978627f1d89b49cb3827a7b19ba97a1febcce"}

	singleContainerEngineMock, err := dockerMockServer(&map[string]string{
		"/v1.22/containers/json":                          filepath.Join(mockFolder, "single_container", "containers.json"),
		"/v1.22/containers/" + containerIDs[0] + "/json":  filepath.Join(mockFolder, "single_container", "container.json"),
		"/v1.22/containers/" + containerIDs[0] + "/stats": filepath.Join(mockFolder, "single_container", "stats.json"),
	})
	assert.NoError(t, err)
	defer singleContainerEngineMock.Close()

	twoContainerEngineMock, err := dockerMockServer(&map[string]string{
		"/v1.22/containers/json":                          filepath.Join(mockFolder, "two_containers", "containers.json"),
		"/v1.22/containers/" + containerIDs[1] + "/json":  filepath.Join(mockFolder, "two_containers", "container1.json"),
		"/v1.22/containers/" + containerIDs[2] + "/json":  filepath.Join(mockFolder, "two_containers", "container2.json"),
		"/v1.22/containers/" + containerIDs[1] + "/stats": filepath.Join(mockFolder, "two_containers", "stats1.json"),
		"/v1.22/containers/" + containerIDs[2] + "/stats": filepath.Join(mockFolder, "two_containers", "stats2.json"),
	})
	assert.NoError(t, err)
	defer twoContainerEngineMock.Close()

	testCases := []struct {
		desc                string
		expectedMetricsFile string
		mockDockerEngine    *httptest.Server
	}{
		{
			desc:                "scrapeV2_single_container",
			expectedMetricsFile: filepath.Join(mockFolder, "single_container", "expected_metrics.yaml"),
			mockDockerEngine:    singleContainerEngineMock,
		},
		{
			desc:                "scrapeV2_two_containers",
			expectedMetricsFile: filepath.Join(mockFolder, "two_containers", "expected_metrics.yaml"),
			mockDockerEngine:    twoContainerEngineMock,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.Endpoint = tc.mockDockerEngine.URL
			cfg.EnvVarsToMetricLabels = map[string]string{"ENV_VAR": "env-var-metric-label"}
			cfg.ContainerLabelsToMetricLabels = map[string]string{"container.label": "container-metric-label"}
			cfg.MetricsBuilderConfig.Metrics = allMetricsEnabled

			receiver := newReceiver(receivertest.NewNopCreateSettings(), cfg)
			err := receiver.start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err)

			actualMetrics, err := receiver.scrapeV2(context.Background())
			require.NoError(t, err)

			expectedMetrics, err := golden.ReadMetrics(tc.expectedMetricsFile)

			assert.NoError(t, err)
			assert.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics,
				pmetrictest.IgnoreResourceMetricsOrder(), pmetrictest.IgnoreStartTimestamp(), pmetrictest.IgnoreTimestamp()))
		})
	}
}

func dockerMockServer(urlToFile *map[string]string) (*httptest.Server, error) {
	urlToFileContents := make(map[string][]byte, len(*urlToFile))
	for urlPath, filePath := range *urlToFile {
		err := func() error {
			fileContents, err := os.ReadFile(filepath.Clean(filePath))
			if err != nil {
				return err
			}
			urlToFileContents[urlPath] = fileContents
			return nil
		}()
		if err != nil {
			return nil, err
		}
	}

	return httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		data, ok := urlToFileContents[req.URL.Path]
		if !ok {
			rw.WriteHeader(http.StatusNotFound)
			return
		}
		rw.WriteHeader(http.StatusOK)
		_, _ = rw.Write(data)
	})), nil
}
