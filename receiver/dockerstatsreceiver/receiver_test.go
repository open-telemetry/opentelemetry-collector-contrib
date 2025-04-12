// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

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

	ctypes "github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
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
		ContainerCPULimit:                          metricEnabled,
		ContainerCPUShares:                         metricEnabled,
		ContainerCPUUtilization:                    metricEnabled,
		ContainerCPUThrottlingDataPeriods:          metricEnabled,
		ContainerCPUThrottlingDataThrottledPeriods: metricEnabled,
		ContainerCPUThrottlingDataThrottledTime:    metricEnabled,
		ContainerCPUUsageKernelmode:                metricEnabled,
		ContainerCPUUsagePercpu:                    metricEnabled,
		ContainerCPUUsageSystem:                    metricEnabled,
		ContainerCPUUsageTotal:                     metricEnabled,
		ContainerCPUUsageUsermode:                  metricEnabled,
		ContainerCPULogicalCount:                   metricEnabled,
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
		ContainerMemoryTotalUnevictable:            metricEnabled,
		ContainerMemoryTotalWriteback:              metricEnabled,
		ContainerMemoryUnevictable:                 metricEnabled,
		ContainerMemoryUsageLimit:                  metricEnabled,
		ContainerMemoryUsageMax:                    metricEnabled,
		ContainerMemoryUsageTotal:                  metricEnabled,
		ContainerMemoryWriteback:                   metricEnabled,
		ContainerMemoryFails:                       metricEnabled,
		ContainerNetworkIoUsageRxBytes:             metricEnabled,
		ContainerNetworkIoUsageRxDropped:           metricEnabled,
		ContainerNetworkIoUsageRxErrors:            metricEnabled,
		ContainerNetworkIoUsageRxPackets:           metricEnabled,
		ContainerNetworkIoUsageTxBytes:             metricEnabled,
		ContainerNetworkIoUsageTxDropped:           metricEnabled,
		ContainerNetworkIoUsageTxErrors:            metricEnabled,
		ContainerNetworkIoUsageTxPackets:           metricEnabled,
		ContainerPidsCount:                         metricEnabled,
		ContainerPidsLimit:                         metricEnabled,
		ContainerUptime:                            metricEnabled,
		ContainerRestarts:                          metricEnabled,
		ContainerMemoryAnon:                        metricEnabled,
		ContainerMemoryFile:                        metricEnabled,
	}

	resourceAttributeEnabled     = metadata.ResourceAttributeConfig{Enabled: true}
	allResourceAttributesEnabled = metadata.ResourceAttributesConfig{
		ContainerCommandLine: resourceAttributeEnabled,
		ContainerHostname:    resourceAttributeEnabled,
		ContainerID:          resourceAttributeEnabled,
		ContainerImageID:     resourceAttributeEnabled,
		ContainerImageName:   resourceAttributeEnabled,
		ContainerName:        resourceAttributeEnabled,
		ContainerRuntime:     resourceAttributeEnabled,
	}
)

func TestNewReceiver(t *testing.T) {
	cfg := &Config{
		ControllerConfig: scraperhelper.ControllerConfig{
			CollectionInterval: 1 * time.Second,
		},
		Config: docker.Config{
			Endpoint:         "unix:///run/some.sock",
			DockerAPIVersion: defaultDockerAPIVersion,
		},
	}
	mr := newMetricsReceiver(receivertest.NewNopSettings(metadata.Type), cfg)
	assert.NotNil(t, mr)
}

func TestErrorsInStart(t *testing.T) {
	unreachable := "unix:///not/a/thing.sock"
	cfg := &Config{
		ControllerConfig: scraperhelper.ControllerConfig{
			CollectionInterval: 1 * time.Second,
		},
		Config: docker.Config{
			Endpoint:         unreachable,
			DockerAPIVersion: defaultDockerAPIVersion,
		},
	}
	recv := newMetricsReceiver(receivertest.NewNopSettings(metadata.Type), cfg)
	assert.NotNil(t, recv)

	cfg.Endpoint = "..not/a/valid/endpoint"
	err := recv.start(context.Background(), componenttest.NewNopHost())
	assert.ErrorContains(t, err, "unable to parse docker host")

	cfg.Endpoint = unreachable
	err = recv.start(context.Background(), componenttest.NewNopHost())
	assert.ErrorContains(t, err, "context deadline exceeded")
}

func TestScrapeV2(t *testing.T) {
	testCases := []struct {
		desc                string
		expectedMetricsFile string
		mockDockerEngine    func(t *testing.T) *httptest.Server
		cfgBuilder          *testConfigBuilder
	}{
		{
			desc:                "scrapeV2_single_container",
			expectedMetricsFile: filepath.Join(mockFolder, "single_container", "expected_metrics.yaml"),
			mockDockerEngine: func(t *testing.T) *httptest.Server {
				t.Helper()
				containerID := "10b703fb312b25e8368ab5a3bce3a1610d1cee5d71a94920f1a7adbc5b0cb326"
				mockServer, err := dockerMockServer(&map[string]string{
					"/v1.25/containers/json":                      filepath.Join(mockFolder, "single_container", "containers.json"),
					"/v1.25/containers/" + containerID + "/json":  filepath.Join(mockFolder, "single_container", "container.json"),
					"/v1.25/containers/" + containerID + "/stats": filepath.Join(mockFolder, "single_container", "stats.json"),
				})
				require.NoError(t, err)
				return mockServer
			},
			cfgBuilder: newTestConfigBuilder().
				withDefaultLabels().
				withMetrics(allMetricsEnabled),
		},
		{
			desc:                "scrapeV2_two_containers",
			expectedMetricsFile: filepath.Join(mockFolder, "two_containers", "expected_metrics.yaml"),
			mockDockerEngine: func(t *testing.T) *httptest.Server {
				t.Helper()
				containerIDs := []string{
					"89d28931fd8b95c8806343a532e9e76bf0a0b76ee8f19452b8f75dee1ebcebb7",
					"a359c0fc87c546b42d2ad32db7c978627f1d89b49cb3827a7b19ba97a1febcce",
				}
				mockServer, err := dockerMockServer(&map[string]string{
					"/v1.25/containers/json":                          filepath.Join(mockFolder, "two_containers", "containers.json"),
					"/v1.25/containers/" + containerIDs[0] + "/json":  filepath.Join(mockFolder, "two_containers", "container1.json"),
					"/v1.25/containers/" + containerIDs[1] + "/json":  filepath.Join(mockFolder, "two_containers", "container2.json"),
					"/v1.25/containers/" + containerIDs[0] + "/stats": filepath.Join(mockFolder, "two_containers", "stats1.json"),
					"/v1.25/containers/" + containerIDs[1] + "/stats": filepath.Join(mockFolder, "two_containers", "stats2.json"),
				})
				require.NoError(t, err)
				return mockServer
			},
			cfgBuilder: newTestConfigBuilder().
				withDefaultLabels().
				withMetrics(allMetricsEnabled),
		},
		{
			desc:                "scrapeV2_no_pids_stats",
			expectedMetricsFile: filepath.Join(mockFolder, "no_pids_stats", "expected_metrics.yaml"),
			mockDockerEngine: func(t *testing.T) *httptest.Server {
				t.Helper()
				containerID := "10b703fb312b25e8368ab5a3bce3a1610d1cee5d71a94920f1a7adbc5b0cb326"
				mockServer, err := dockerMockServer(&map[string]string{
					"/v1.25/containers/json":                      filepath.Join(mockFolder, "no_pids_stats", "containers.json"),
					"/v1.25/containers/" + containerID + "/json":  filepath.Join(mockFolder, "no_pids_stats", "container.json"),
					"/v1.25/containers/" + containerID + "/stats": filepath.Join(mockFolder, "no_pids_stats", "stats.json"),
				})
				require.NoError(t, err)
				return mockServer
			},
			cfgBuilder: newTestConfigBuilder().
				withDefaultLabels().
				withMetrics(allMetricsEnabled),
		},
		{
			desc:                "scrapeV2_pid_stats_max",
			expectedMetricsFile: filepath.Join(mockFolder, "pids_stats_max", "expected_metrics.yaml"),
			mockDockerEngine: func(t *testing.T) *httptest.Server {
				t.Helper()
				containerID := "78de07328afff50a9777b07dd36a28c709dffe081baaf67235db618843399643"
				mockServer, err := dockerMockServer(&map[string]string{
					"/v1.25/containers/json":                      filepath.Join(mockFolder, "pids_stats_max", "containers.json"),
					"/v1.25/containers/" + containerID + "/json":  filepath.Join(mockFolder, "pids_stats_max", "container.json"),
					"/v1.25/containers/" + containerID + "/stats": filepath.Join(mockFolder, "pids_stats_max", "stats.json"),
				})
				require.NoError(t, err)
				return mockServer
			},
			cfgBuilder: newTestConfigBuilder().
				withDefaultLabels().
				withMetrics(allMetricsEnabled),
		},
		{
			desc:                "scrapeV2_cpu_limit",
			expectedMetricsFile: filepath.Join(mockFolder, "cpu_limit", "expected_metrics.yaml"),
			mockDockerEngine: func(t *testing.T) *httptest.Server {
				t.Helper()
				containerID := "9b842c47c1c3e4ee931e2c9713cf4e77aa09acc2201aea60fba04b6dbba6c674"
				mockServer, err := dockerMockServer(&map[string]string{
					"/v1.25/containers/json":                      filepath.Join(mockFolder, "cpu_limit", "containers.json"),
					"/v1.25/containers/" + containerID + "/json":  filepath.Join(mockFolder, "cpu_limit", "container.json"),
					"/v1.25/containers/" + containerID + "/stats": filepath.Join(mockFolder, "cpu_limit", "stats.json"),
				})
				require.NoError(t, err)
				return mockServer
			},
			cfgBuilder: newTestConfigBuilder().
				withDefaultLabels().
				withMetrics(allMetricsEnabled),
		},
		{
			desc:                "cgroups_v2_container",
			expectedMetricsFile: filepath.Join(mockFolder, "cgroups_v2", "expected_metrics.yaml"),
			mockDockerEngine: func(t *testing.T) *httptest.Server {
				containerID := "f97ed5bca0a5a0b85bfd52c4144b96174e825c92a138bc0458f0e196f2c7c1b4"
				mockServer, err := dockerMockServer(&map[string]string{
					"/v1.25/containers/json":                      filepath.Join(mockFolder, "cgroups_v2", "containers.json"),
					"/v1.25/containers/" + containerID + "/json":  filepath.Join(mockFolder, "cgroups_v2", "container.json"),
					"/v1.25/containers/" + containerID + "/stats": filepath.Join(mockFolder, "cgroups_v2", "stats.json"),
				})
				require.NoError(t, err)
				return mockServer
			},
			cfgBuilder: newTestConfigBuilder().
				withDefaultLabels().
				withMetrics(allMetricsEnabled),
		},
		{
			desc:                "scrapeV2_single_container_with_optional_resource_attributes",
			expectedMetricsFile: filepath.Join(mockFolder, "single_container_with_optional_resource_attributes", "expected_metrics.yaml"),
			mockDockerEngine: func(t *testing.T) *httptest.Server {
				containerID := "73364842ef014441cac89fed05df19463b1230db25a31252cdf82e754f1ec581"
				mockServer, err := dockerMockServer(&map[string]string{
					"/v1.25/containers/json":                      filepath.Join(mockFolder, "single_container_with_optional_resource_attributes", "containers.json"),
					"/v1.25/containers/" + containerID + "/json":  filepath.Join(mockFolder, "single_container_with_optional_resource_attributes", "container.json"),
					"/v1.25/containers/" + containerID + "/stats": filepath.Join(mockFolder, "single_container_with_optional_resource_attributes", "stats.json"),
				})
				require.NoError(t, err)
				return mockServer
			},
			cfgBuilder: newTestConfigBuilder().
				withDefaultLabels().
				withMetrics(allMetricsEnabled).
				withResourceAttributes(allResourceAttributesEnabled),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			mockDockerEngine := tc.mockDockerEngine(t)
			defer mockDockerEngine.Close()

			receiver := newMetricsReceiver(
				receivertest.NewNopSettings(metadata.Type), tc.cfgBuilder.withEndpoint(mockDockerEngine.URL).build())
			err := receiver.start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err)
			defer func() { require.NoError(t, receiver.shutdown(context.Background())) }()

			actualMetrics, err := receiver.scrapeV2(context.Background())
			require.NoError(t, err)

			// Uncomment to regenerate 'expected_metrics.yaml' files
			// golden.WriteMetrics(t, tc.expectedMetricsFile, actualMetrics)

			expectedMetrics, err := golden.ReadMetrics(tc.expectedMetricsFile)

			assert.NoError(t, err)
			assert.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, actualMetrics,
				pmetrictest.IgnoreMetricDataPointsOrder(),
				pmetrictest.IgnoreResourceMetricsOrder(),
				pmetrictest.IgnoreStartTimestamp(),
				pmetrictest.IgnoreTimestamp(),
				pmetrictest.IgnoreMetricValues(
					"container.uptime", // value depends on time.Now(), making it unpredictable as far as tests go
				),
			))
		})
	}
}

func TestRecordBaseMetrics(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Metrics = metadata.MetricsConfig{
		ContainerUptime: metricEnabled,
	}
	r := newMetricsReceiver(receivertest.NewNopSettings(metadata.Type), cfg)
	now := time.Now()
	started := now.Add(-2 * time.Second).Format(time.RFC3339)

	t.Run("ok", func(t *testing.T) {
		err := r.recordBaseMetrics(
			pcommon.NewTimestampFromTime(now),
			&ctypes.ContainerJSONBase{
				State: &ctypes.State{
					StartedAt: started,
				},
			},
		)
		require.NoError(t, err)
		m := r.mb.Emit().ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
		assert.Equal(t, "container.uptime", m.Name())
		dp := m.Gauge().DataPoints()
		assert.Equal(t, 1, dp.Len())
		assert.Equal(t, 2, int(dp.At(0).DoubleValue()))
	})

	t.Run("error", func(t *testing.T) {
		err := r.recordBaseMetrics(
			pcommon.NewTimestampFromTime(now),
			&ctypes.ContainerJSONBase{
				State: &ctypes.State{
					StartedAt: "bad date",
				},
			},
		)
		require.Error(t, err)
	})
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

type testConfigBuilder struct {
	config *Config
}

func newTestConfigBuilder() *testConfigBuilder {
	return &testConfigBuilder{config: createDefaultConfig().(*Config)}
}

func (cb *testConfigBuilder) withEndpoint(endpoint string) *testConfigBuilder {
	cb.config.Endpoint = endpoint
	return cb
}

func (cb *testConfigBuilder) withMetrics(ms metadata.MetricsConfig) *testConfigBuilder {
	cb.config.Metrics = ms
	return cb
}

func (cb *testConfigBuilder) withResourceAttributes(ras metadata.ResourceAttributesConfig) *testConfigBuilder {
	cb.config.ResourceAttributes = ras
	return cb
}

func (cb *testConfigBuilder) withDefaultLabels() *testConfigBuilder {
	cb.config.EnvVarsToMetricLabels = map[string]string{
		"ENV_VAR":   "env-var-metric-label",
		"ENV_VAR_2": "env-var-metric-label-2",
	}
	cb.config.ContainerLabelsToMetricLabels = map[string]string{
		"container.label":   "container-metric-label",
		"container.label.2": "container-metric-label-2",
	}
	return cb
}

func (cb *testConfigBuilder) build() *Config {
	return cb.config
}
