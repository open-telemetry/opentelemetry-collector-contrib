// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubeletstatsreceiver

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
)

func init() {
	// Disable WatchListClient feature gate for tests as fake clientset doesn't support bookmark events
	// See: https://github.com/kubernetes/kubernetes/issues/129408
	os.Setenv("KUBE_FEATURE_WatchListClient", "false")
}

const (
	// Number of resources by type in testdata/stats-summary.json
	numContainers       = 9
	numPods             = 9
	numNodes            = 1
	numVolumes          = 8
	numSystemContainers = 3

	// Number of metrics by resource
	nodeMetrics            = 15
	podMetrics             = 15
	containerMetrics       = 11
	volumeMetrics          = 5
	systemContainerMetrics = 4

	dataLen = numContainers*containerMetrics + numPods*podMetrics + numNodes*nodeMetrics + numVolumes*volumeMetrics
)

var allMetricGroups = map[kubelet.MetricGroup]bool{
	kubelet.ContainerMetricGroup: true,
	kubelet.PodMetricGroup:       true,
	kubelet.NodeMetricGroup:      true,
	kubelet.VolumeMetricGroup:    true,
}

func TestScraper(t *testing.T) {
	options := &scraperOptions{
		metricGroupsToCollect: allMetricGroups,
	}
	r, err := newKubeletScraper(
		&fakeRestClient{},
		receivertest.NewNopSettings(metadata.Type),
		options,
		metadata.NewDefaultMetricsBuilderConfig(),
		"worker-42",
	)
	require.NoError(t, err)

	md, err := r.ScrapeMetrics(t.Context())
	require.NoError(t, err)
	require.Equal(t, dataLen, md.DataPointCount())
	expectedFile := filepath.Join("testdata", "scraper", "test_scraper_expected.yaml")

	// Uncomment to regenerate '*_expected.yaml' files
	// golden.WriteMetrics(t, expectedFile, md)

	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)
	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, md,
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreMetricsOrder()))
}

func TestScraperWithSystemContainerMetrics(t *testing.T) {
	options := &scraperOptions{
		metricGroupsToCollect: allMetricGroups,
	}
	metricsConfig := metadata.NewDefaultMetricsBuilderConfig()
	metricsConfig.Metrics.K8sNodeSystemContainerCPUTime.Enabled = true
	metricsConfig.Metrics.K8sNodeSystemContainerCPUUsage.Enabled = true
	metricsConfig.Metrics.K8sNodeSystemContainerMemoryUsage.Enabled = true
	metricsConfig.Metrics.K8sNodeSystemContainerMemoryWorkingSet.Enabled = true

	r, err := newKubeletScraper(
		&fakeRestClient{},
		receivertest.NewNopSettings(metadata.Type),
		options,
		metricsConfig,
		"worker-42",
	)
	require.NoError(t, err)

	md, err := r.ScrapeMetrics(t.Context())
	require.NoError(t, err)

	require.Equal(t, dataLen+numSystemContainers*systemContainerMetrics, md.DataPointCount())
	expectedFile := filepath.Join("testdata", "scraper", "test_scraper_with_system_container_expected.yaml")

	// Uncomment to regenerate '*_expected.yaml' files
	// golden.WriteMetrics(t, expectedFile, md)

	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)
	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, md,
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreMetricsOrder()))
}

func TestScraperWithEphemeralStorageMetrics(t *testing.T) {
	options := &scraperOptions{
		metricGroupsToCollect: allMetricGroups,
	}
	metricsConfig := metadata.NewDefaultMetricsBuilderConfig()
	metricsConfig.Metrics.K8sContainerEphemeralStorageUsage.Enabled = true

	r, err := newKubeletScraper(
		&fakeRestClient{},
		receivertest.NewNopSettings(metadata.Type),
		options,
		metricsConfig,
		"worker-42",
	)
	require.NoError(t, err)

	md, err := r.ScrapeMetrics(t.Context())
	require.NoError(t, err)

	require.Equal(t, dataLen+numContainers*2, md.DataPointCount())
	expectedFile := filepath.Join("testdata", "scraper", "test_scraper_with_ephemeral_storage_expected.yaml")

	// Uncomment to regenerate '*_expected.yaml' files
	// golden.WriteMetrics(t, expectedFile, md)

	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)
	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, md,
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreMetricsOrder()))
}

func TestScraperWithInterfacesMetrics(t *testing.T) {
	options := &scraperOptions{
		metricGroupsToCollect: allMetricGroups,
		allNetworkInterfaces: map[kubelet.MetricGroup]bool{
			kubelet.NodeMetricGroup: true,
			kubelet.PodMetricGroup:  true,
		},
	}
	r, err := newKubeletScraper(
		&fakeRestClient{},
		receivertest.NewNopSettings(metadata.Type),
		options,
		metadata.NewDefaultMetricsBuilderConfig(),
		"worker-42",
	)
	require.NoError(t, err)

	md, err := r.ScrapeMetrics(t.Context())
	require.NoError(t, err)

	require.Equal(t, dataLen+numPods*4+numNodes*4, md.DataPointCount())
	expectedFile := filepath.Join("testdata", "scraper", "test_scraper_with_interfaces_metrics.yaml")

	// Uncomment to regenerate '*_expected.yaml' files
	// golden.WriteMetrics(t, expectedFile, md)

	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)
	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, md,
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreMetricsOrder()))
}

func TestScraperWithCPUNodeUtilization(t *testing.T) {
	watcherStarted := make(chan struct{})
	// Create the fake client.
	client := fake.NewClientset()
	// A catch-all watch reactor that allows us to inject the watcherStarted channel.
	client.PrependWatchReactor("*", func(action clienttesting.Action) (handled bool, ret watch.Interface, err error) {
		gvr := action.GetResource()
		ns := action.GetNamespace()
		watch, err := client.Tracker().Watch(gvr, ns)
		if err != nil {
			return false, nil, err
		}
		close(watcherStarted)
		return true, watch, nil
	})

	options := &scraperOptions{
		metricGroupsToCollect: map[kubelet.MetricGroup]bool{
			kubelet.ContainerMetricGroup: true,
			kubelet.PodMetricGroup:       true,
		},
		k8sAPIClient: client,
	}
	r, err := newKubeletScraper(
		&fakeRestClient{},
		receivertest.NewNopSettings(metadata.Type),
		options,
		metadata.MetricsBuilderConfig{
			Metrics: metadata.MetricsConfig{
				K8sContainerCPUNodeUtilization: metadata.K8sContainerCPUNodeUtilizationMetricConfig{
					Enabled: true,
				},
				K8sPodCPUNodeUtilization: metadata.K8sPodCPUNodeUtilizationMetricConfig{
					Enabled: true,
				},
			},
			ResourceAttributes: metadata.DefaultResourceAttributesConfig(),
		},
		"worker-42",
	)
	require.NoError(t, err)

	err = r.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	// we wait until the watcher starts
	<-watcherStarted
	// Inject an event node into the fake client.
	node := getNodeWithCPUCapacity("worker-42", 8)
	_, err = client.CoreV1().Nodes().Create(t.Context(), node, metav1.CreateOptions{})
	if err != nil {
		require.NoError(t, err)
	}

	var md pmetric.Metrics
	require.Eventually(t, func() bool {
		md, err = r.ScrapeMetrics(t.Context())
		require.NoError(t, err)
		return numContainers+numPods == md.DataPointCount()
	}, 10*time.Second, 100*time.Millisecond,
		"metrics not collected")

	expectedFile := filepath.Join("testdata", "scraper", "test_scraper_cpu_util_nodelimit_expected.yaml")

	// Uncomment to regenerate '*_expected.yaml' files
	// golden.WriteMetrics(t, expectedFile, md)

	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)
	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, md,
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreMetricsOrder()))

	err = r.Shutdown(t.Context())
	require.NoError(t, err)
}

func TestScraperWithMemoryNodeUtilization(t *testing.T) {
	watcherStarted := make(chan struct{})
	// Create the fake client.
	client := fake.NewClientset()
	// A catch-all watch reactor that allows us to inject the watcherStarted channel.
	client.PrependWatchReactor("*", func(action clienttesting.Action) (handled bool, ret watch.Interface, err error) {
		gvr := action.GetResource()
		ns := action.GetNamespace()
		watch, err := client.Tracker().Watch(gvr, ns)
		if err != nil {
			return false, nil, err
		}
		close(watcherStarted)
		return true, watch, nil
	})

	options := &scraperOptions{
		metricGroupsToCollect: map[kubelet.MetricGroup]bool{
			kubelet.ContainerMetricGroup: true,
			kubelet.PodMetricGroup:       true,
		},
		k8sAPIClient: client,
	}
	r, err := newKubeletScraper(
		&fakeRestClient{},
		receivertest.NewNopSettings(metadata.Type),
		options,
		metadata.MetricsBuilderConfig{
			Metrics: metadata.MetricsConfig{
				K8sContainerMemoryNodeUtilization: metadata.K8sContainerMemoryNodeUtilizationMetricConfig{
					Enabled: true,
				}, K8sPodMemoryNodeUtilization: metadata.K8sPodMemoryNodeUtilizationMetricConfig{
					Enabled: true,
				},
			},
			ResourceAttributes: metadata.DefaultResourceAttributesConfig(),
		},
		"worker-42",
	)
	require.NoError(t, err)

	err = r.Start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	// we wait until the watcher starts
	<-watcherStarted
	// Inject an event node into the fake client.
	node := getNodeWithMemoryCapacity("worker-42", "32564740Ki")
	_, err = client.CoreV1().Nodes().Create(t.Context(), node, metav1.CreateOptions{})
	if err != nil {
		require.NoError(t, err)
	}

	var md pmetric.Metrics
	require.Eventually(t, func() bool {
		md, err = r.ScrapeMetrics(t.Context())
		require.NoError(t, err)
		return numContainers+numPods == md.DataPointCount()
	}, 10*time.Second, 100*time.Millisecond,
		"metrics not collected")
	expectedFile := filepath.Join("testdata", "scraper", "test_scraper_memory_util_nodelimit_expected.yaml")

	// Uncomment to regenerate '*_expected.yaml' files
	// golden.WriteMetrics(t, expectedFile, md)

	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)
	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, md,
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreResourceMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreMetricsOrder()))

	err = r.Shutdown(t.Context())
	require.NoError(t, err)
}

func TestScraperWithMetadata(t *testing.T) {
	tests := []struct {
		name           string
		metadataLabels []kubelet.MetadataLabel
		metricGroups   map[kubelet.MetricGroup]bool
		dataLen        int
		metricPrefix   string
		requiredLabel  string
	}{
		{
			name:           "Container_Metadata",
			metadataLabels: []kubelet.MetadataLabel{kubelet.MetadataLabelContainerID},
			metricGroups: map[kubelet.MetricGroup]bool{
				kubelet.ContainerMetricGroup: true,
			},
			dataLen:       numContainers * containerMetrics,
			metricPrefix:  "container.",
			requiredLabel: "container.id",
		},
		{
			name:           "Volume_Metadata",
			metadataLabels: []kubelet.MetadataLabel{kubelet.MetadataLabelVolumeType},
			metricGroups: map[kubelet.MetricGroup]bool{
				kubelet.VolumeMetricGroup: true,
			},
			dataLen:       numVolumes * volumeMetrics,
			metricPrefix:  "k8s.volume.",
			requiredLabel: "k8s.volume.type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := &scraperOptions{
				extraMetadataLabels:   tt.metadataLabels,
				metricGroupsToCollect: tt.metricGroups,
			}
			r, err := newKubeletScraper(
				&fakeRestClient{},
				receivertest.NewNopSettings(metadata.Type),
				options,
				metadata.NewDefaultMetricsBuilderConfig(),
				"worker-42",
			)
			require.NoError(t, err)

			md, err := r.ScrapeMetrics(t.Context())
			require.NoError(t, err)

			filename := "test_scraper_with_metadata_" + tt.name + "_expected.yaml"
			expectedFile := filepath.Join("testdata", "scraper", filename)

			// Uncomment to regenerate '*_expected.yaml' files
			// golden.WriteMetrics(t, expectedFile, md)

			expectedMetrics, err := golden.ReadMetrics(expectedFile)
			require.NoError(t, err)
			require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, md,
				pmetrictest.IgnoreStartTimestamp(),
				pmetrictest.IgnoreResourceMetricsOrder(),
				pmetrictest.IgnoreMetricDataPointsOrder(),
				pmetrictest.IgnoreTimestamp(),
				pmetrictest.IgnoreMetricsOrder()))
		})
	}
}

func TestScraperWithPercentMetrics(t *testing.T) {
	options := &scraperOptions{
		metricGroupsToCollect: map[kubelet.MetricGroup]bool{
			kubelet.ContainerMetricGroup: true,
			kubelet.PodMetricGroup:       true,
		},
	}
	metricsConfig := metadata.MetricsBuilderConfig{
		Metrics: metadata.MetricsConfig{
			ContainerCPUTime: metadata.ContainerCPUTimeMetricConfig{
				Enabled: false,
			},
			ContainerFilesystemAvailable: metadata.ContainerFilesystemAvailableMetricConfig{
				Enabled: false,
			},
			ContainerFilesystemCapacity: metadata.ContainerFilesystemCapacityMetricConfig{
				Enabled: false,
			},
			ContainerFilesystemUsage: metadata.ContainerFilesystemUsageMetricConfig{
				Enabled: false,
			},
			ContainerMemoryAvailable: metadata.ContainerMemoryAvailableMetricConfig{
				Enabled: false,
			},
			ContainerMemoryMajorPageFaults: metadata.ContainerMemoryMajorPageFaultsMetricConfig{
				Enabled: false,
			},
			ContainerMemoryPageFaults: metadata.ContainerMemoryPageFaultsMetricConfig{
				Enabled: false,
			},
			ContainerMemoryRss: metadata.ContainerMemoryRssMetricConfig{
				Enabled: false,
			},
			ContainerMemoryUsage: metadata.ContainerMemoryUsageMetricConfig{
				Enabled: false,
			},
			K8sContainerCPULimitUtilization: metadata.K8sContainerCPULimitUtilizationMetricConfig{
				Enabled: true,
			},
			K8sContainerCPURequestUtilization: metadata.K8sContainerCPURequestUtilizationMetricConfig{
				Enabled: true,
			},
			K8sContainerMemoryLimitUtilization: metadata.K8sContainerMemoryLimitUtilizationMetricConfig{
				Enabled: true,
			},
			K8sContainerMemoryRequestUtilization: metadata.K8sContainerMemoryRequestUtilizationMetricConfig{
				Enabled: true,
			},
			ContainerMemoryWorkingSet: metadata.ContainerMemoryWorkingSetMetricConfig{
				Enabled: false,
			},
			K8sNodeCPUTime: metadata.K8sNodeCPUTimeMetricConfig{
				Enabled: false,
			},
			K8sNodeFilesystemAvailable: metadata.K8sNodeFilesystemAvailableMetricConfig{
				Enabled: false,
			},
			K8sNodeFilesystemCapacity: metadata.K8sNodeFilesystemCapacityMetricConfig{
				Enabled: false,
			},
			K8sNodeFilesystemUsage: metadata.K8sNodeFilesystemUsageMetricConfig{
				Enabled: false,
			},
			K8sNodeMemoryAvailable: metadata.K8sNodeMemoryAvailableMetricConfig{
				Enabled: false,
			},
			K8sNodeMemoryMajorPageFaults: metadata.K8sNodeMemoryMajorPageFaultsMetricConfig{
				Enabled: false,
			},
			K8sNodeMemoryPageFaults: metadata.K8sNodeMemoryPageFaultsMetricConfig{
				Enabled: false,
			},
			K8sNodeMemoryRss: metadata.K8sNodeMemoryRssMetricConfig{
				Enabled: false,
			},
			K8sNodeMemoryUsage: metadata.K8sNodeMemoryUsageMetricConfig{
				Enabled: false,
			},
			K8sNodeMemoryWorkingSet: metadata.K8sNodeMemoryWorkingSetMetricConfig{
				Enabled: false,
			},
			K8sNodeNetworkErrors: metadata.K8sNodeNetworkErrorsMetricConfig{
				Enabled: false,
			},
			K8sNodeNetworkIo: metadata.K8sNodeNetworkIoMetricConfig{
				Enabled: false,
			},
			K8sPodCPUTime: metadata.K8sPodCPUTimeMetricConfig{
				Enabled: false,
			},
			K8sPodFilesystemAvailable: metadata.K8sPodFilesystemAvailableMetricConfig{
				Enabled: false,
			},
			K8sPodFilesystemCapacity: metadata.K8sPodFilesystemCapacityMetricConfig{
				Enabled: false,
			},
			K8sPodFilesystemUsage: metadata.K8sPodFilesystemUsageMetricConfig{
				Enabled: false,
			},
			K8sPodMemoryAvailable: metadata.K8sPodMemoryAvailableMetricConfig{
				Enabled: false,
			},
			K8sPodMemoryMajorPageFaults: metadata.K8sPodMemoryMajorPageFaultsMetricConfig{
				Enabled: false,
			},
			K8sPodMemoryPageFaults: metadata.K8sPodMemoryPageFaultsMetricConfig{
				Enabled: false,
			},
			K8sPodMemoryRss: metadata.K8sPodMemoryRssMetricConfig{
				Enabled: false,
			},
			K8sPodMemoryUsage: metadata.K8sPodMemoryUsageMetricConfig{
				Enabled: false,
			},
			K8sPodCPULimitUtilization: metadata.K8sPodCPULimitUtilizationMetricConfig{
				Enabled: true,
			},
			K8sPodCPURequestUtilization: metadata.K8sPodCPURequestUtilizationMetricConfig{
				Enabled: true,
			},
			K8sPodMemoryLimitUtilization: metadata.K8sPodMemoryLimitUtilizationMetricConfig{
				Enabled: true,
			},
			K8sPodMemoryRequestUtilization: metadata.K8sPodMemoryRequestUtilizationMetricConfig{
				Enabled: true,
			},
			K8sPodMemoryWorkingSet: metadata.K8sPodMemoryWorkingSetMetricConfig{
				Enabled: false,
			},
			K8sPodNetworkErrors: metadata.K8sPodNetworkErrorsMetricConfig{
				Enabled: false,
			},
			K8sPodNetworkIo: metadata.K8sPodNetworkIoMetricConfig{
				Enabled: false,
			},
			K8sVolumeAvailable: metadata.K8sVolumeAvailableMetricConfig{
				Enabled: false,
			},
			K8sVolumeCapacity: metadata.K8sVolumeCapacityMetricConfig{
				Enabled: false,
			},
			K8sPodVolumeUsage: metadata.K8sPodVolumeUsageMetricConfig{
				Enabled: false,
			},
			K8sVolumeInodes: metadata.K8sVolumeInodesMetricConfig{
				Enabled: false,
			},
			K8sVolumeInodesFree: metadata.K8sVolumeInodesFreeMetricConfig{
				Enabled: false,
			},
			K8sVolumeInodesUsed: metadata.K8sVolumeInodesUsedMetricConfig{
				Enabled: false,
			},
		},
		ResourceAttributes: metadata.DefaultResourceAttributesConfig(),
	}
	r, err := newKubeletScraper(
		&fakeRestClient{},
		receivertest.NewNopSettings(metadata.Type),
		options,
		metricsConfig,
		"worker-42",
	)
	require.NoError(t, err)

	md, err := r.ScrapeMetrics(t.Context())
	require.NoError(t, err)

	expectedFile := filepath.Join("testdata", "scraper", "test_scraper_with_percent_expected.yaml")

	// Uncomment to regenerate '*_expected.yaml' files
	// golden.WriteMetrics(t, expectedFile, md)

	expectedMetrics, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)
	require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, md,
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreMetricsOrder()))
}

func TestScraperWithMetricGroups(t *testing.T) {
	tests := []struct {
		name         string
		metricGroups map[kubelet.MetricGroup]bool
		dataLen      int
	}{
		{
			name:         "all_groups",
			metricGroups: allMetricGroups,
			dataLen:      dataLen,
		},
		{
			name: "only_container_group",
			metricGroups: map[kubelet.MetricGroup]bool{
				kubelet.ContainerMetricGroup: true,
			},
			dataLen: numContainers * containerMetrics,
		},
		{
			name: "only_pod_group",
			metricGroups: map[kubelet.MetricGroup]bool{
				kubelet.PodMetricGroup: true,
			},
			dataLen: numPods * podMetrics,
		},
		{
			name: "only_node_group",
			metricGroups: map[kubelet.MetricGroup]bool{
				kubelet.NodeMetricGroup: true,
			},
			dataLen: numNodes * nodeMetrics,
		},
		{
			name: "only_volume_group",
			metricGroups: map[kubelet.MetricGroup]bool{
				kubelet.VolumeMetricGroup: true,
			},
			dataLen: numVolumes * volumeMetrics,
		},
		{
			name: "pod_and_node_groups",
			metricGroups: map[kubelet.MetricGroup]bool{
				kubelet.PodMetricGroup:  true,
				kubelet.NodeMetricGroup: true,
			},
			dataLen: numNodes*nodeMetrics + numPods*podMetrics,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r, err := newKubeletScraper(
				&fakeRestClient{},
				receivertest.NewNopSettings(metadata.Type),
				&scraperOptions{
					extraMetadataLabels:   []kubelet.MetadataLabel{kubelet.MetadataLabelContainerID},
					metricGroupsToCollect: test.metricGroups,
				},
				metadata.NewDefaultMetricsBuilderConfig(),
				"worker-42",
			)
			require.NoError(t, err)

			md, err := r.ScrapeMetrics(t.Context())
			require.NoError(t, err)

			filename := "test_scraper_with_metric_groups_" + test.name + "_expected.yaml"
			expectedFile := filepath.Join("testdata", "scraper", filename)

			// Uncomment to regenerate '*_expected.yaml' files
			// golden.WriteMetrics(t, expectedFile, md)

			expectedMetrics, err := golden.ReadMetrics(expectedFile)
			require.NoError(t, err)
			require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, md,
				pmetrictest.IgnoreStartTimestamp(),
				pmetrictest.IgnoreResourceMetricsOrder(),
				pmetrictest.IgnoreMetricDataPointsOrder(),
				pmetrictest.IgnoreTimestamp(),
				pmetrictest.IgnoreMetricsOrder()))
		})
	}
}

type expectedVolume struct {
	name   string
	typ    string
	labels map[string]string
}

func TestScraperWithPVCDetailedLabels(t *testing.T) {
	tests := []struct {
		name               string
		k8sAPIClient       kubernetes.Interface
		expectedVolumes    map[string]expectedVolume
		volumeClaimsToMiss map[string]bool
		dataLen            int
		numLogs            int
	}{
		{
			name:         "successful",
			k8sAPIClient: fake.NewClientset(getValidMockedObjects()...),
			expectedVolumes: map[string]expectedVolume{
				"volume_claim_1": {
					name: "storage-provisioner-token-qzlx6",
					typ:  "awsElasticBlockStore",
					labels: map[string]string{
						"aws.volume.id": "volume_id",
						"fs.type":       "fs_type",
						"partition":     "10",
					},
				},
				"volume_claim_2": {
					name: "kube-proxy",
					typ:  "gcePersistentDisk",
					labels: map[string]string{
						"gce.pd.name": "pd_name",
						"fs.type":     "fs_type",
						"partition":   "10",
					},
				},
				"volume_claim_3": {
					name: "coredns-token-dzc5t",
					typ:  "glusterfs",
					labels: map[string]string{
						"glusterfs.endpoints.name": "endpoints_name",
						"glusterfs.path":           "path",
					},
				},
			},
			dataLen: numVolumes,
		},
		{
			name:         "nonexistent_pvc",
			k8sAPIClient: fake.NewClientset(),
			dataLen:      numVolumes - 3,
			volumeClaimsToMiss: map[string]bool{
				"volume_claim_1": true,
				"volume_claim_2": true,
				"volume_claim_3": true,
			},
			numLogs: 3,
		},
		{
			name:         "empty_volume_name_in_pvc",
			k8sAPIClient: fake.NewClientset(getMockedObjectsWithEmptyVolumeName()...),
			expectedVolumes: map[string]expectedVolume{
				"volume_claim_1": {
					name: "storage-provisioner-token-qzlx6",
					typ:  "awsElasticBlockStore",
					labels: map[string]string{
						"aws.volume.id": "volume_id",
						"fs.type":       "fs_type",
						"partition":     "10",
					},
				},
				"volume_claim_2": {
					name: "kube-proxy",
					typ:  "gcePersistentDisk",
					labels: map[string]string{
						"gce.pd.name": "pd_name",
						"fs.type":     "fs_type",
						"partition":   "10",
					},
				},
			},
			// Two of mocked volumes are invalid and do not match any of volumes in
			// testdata/pods.json. Hence, don't expected to see metrics from them.
			dataLen: numVolumes - 1,
			volumeClaimsToMiss: map[string]bool{
				"volume_claim_3": true,
			},
			numLogs: 1,
		},
		{
			name:         "non_existent_volume_in_pvc",
			k8sAPIClient: fake.NewClientset(getMockedObjectsWithNonExistentVolumeName()...),
			expectedVolumes: map[string]expectedVolume{
				"volume_claim_1": {
					name: "storage-provisioner-token-qzlx6",
					typ:  "awsElasticBlockStore",
					labels: map[string]string{
						"aws.volume.id": "volume_id",
						"fs.type":       "fs_type",
						"partition":     "10",
					},
				},
				"volume_claim_2": {
					name: "kube-proxy",
					typ:  "gcePersistentDisk",
					labels: map[string]string{
						"gce.pd.name": "pd_name",
						"fs.type":     "fs_type",
						"partition":   "10",
					},
				},
			},
			// Two of mocked volumes are invalid and do not match any of volumes in
			// testdata/pods.json. Hence, don't expected to see metrics from them.
			dataLen: numVolumes - 1,
			volumeClaimsToMiss: map[string]bool{
				"volume_claim_3": true,
			},
			numLogs: 1,
		},
		{
			name:    "do_not_collect_detailed_labels",
			dataLen: numVolumes,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mbc := metadata.NewDefaultMetricsBuilderConfig()
			mbc.ResourceAttributes.AwsVolumeID.Enabled = true
			mbc.ResourceAttributes.FsType.Enabled = true
			mbc.ResourceAttributes.GcePdName.Enabled = true
			mbc.ResourceAttributes.GlusterfsEndpointsName.Enabled = true
			mbc.ResourceAttributes.GlusterfsPath.Enabled = true
			mbc.ResourceAttributes.Partition.Enabled = true

			r, err := newKubeletScraper(
				&fakeRestClient{},
				receivertest.NewNopSettings(metadata.Type),
				&scraperOptions{
					extraMetadataLabels: []kubelet.MetadataLabel{kubelet.MetadataLabelVolumeType},
					metricGroupsToCollect: map[kubelet.MetricGroup]bool{
						kubelet.VolumeMetricGroup: true,
					},
					k8sAPIClient: test.k8sAPIClient,
				},
				mbc,
				"worker-42",
			)
			require.NoError(t, err)

			md, err := r.ScrapeMetrics(t.Context())
			require.NoError(t, err)

			filename := "test_scraper_with_pvc_labels_" + test.name + "_expected.yaml"
			expectedFile := filepath.Join("testdata", "scraper", filename)

			// Uncomment to regenerate '*_expected.yaml' files
			// golden.WriteMetrics(t, expectedFile, md)

			expectedMetrics, err := golden.ReadMetrics(expectedFile)
			require.NoError(t, err)
			require.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, md,
				pmetrictest.IgnoreStartTimestamp(),
				pmetrictest.IgnoreResourceMetricsOrder(),
				pmetrictest.IgnoreMetricDataPointsOrder(),
				pmetrictest.IgnoreTimestamp(),
				pmetrictest.IgnoreMetricsOrder()))
		})
	}
}

func TestClientErrors(t *testing.T) {
	tests := []struct {
		name                  string
		statsSummaryFail      bool
		podsFail              bool
		extraMetadataLabels   []kubelet.MetadataLabel
		metricGroupsToCollect map[kubelet.MetricGroup]bool
		numLogs               int
	}{
		{
			name:                  "no_errors_without_metadata",
			statsSummaryFail:      false,
			podsFail:              false,
			extraMetadataLabels:   nil,
			metricGroupsToCollect: allMetricGroups,
			numLogs:               0,
		},
		{
			name:                  "no_errors_with_metadata",
			statsSummaryFail:      false,
			podsFail:              false,
			extraMetadataLabels:   []kubelet.MetadataLabel{kubelet.MetadataLabelContainerID},
			metricGroupsToCollect: allMetricGroups,
			numLogs:               0,
		},
		{
			name:                  "stats_summary_endpoint_error",
			statsSummaryFail:      true,
			podsFail:              false,
			extraMetadataLabels:   nil,
			metricGroupsToCollect: allMetricGroups,
			numLogs:               1,
		},
		{
			name:                  "pods_endpoint_error",
			statsSummaryFail:      false,
			podsFail:              true,
			extraMetadataLabels:   []kubelet.MetadataLabel{kubelet.MetadataLabelContainerID},
			metricGroupsToCollect: allMetricGroups,
			numLogs:               1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			core, observedLogs := observer.New(zap.ErrorLevel)
			logger := zap.New(core)
			settings := receivertest.NewNopSettings(metadata.Type)
			settings.Logger = logger
			options := &scraperOptions{
				extraMetadataLabels:   test.extraMetadataLabels,
				metricGroupsToCollect: test.metricGroupsToCollect,
			}
			r, err := newKubeletScraper(
				&fakeRestClient{
					statsSummaryFail: test.statsSummaryFail,
					podsFail:         test.podsFail,
				},
				settings,
				options,
				metadata.NewDefaultMetricsBuilderConfig(),
				"",
			)
			require.NoError(t, err)

			_, err = r.ScrapeMetrics(t.Context())
			if test.numLogs == 0 {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			require.Equal(t, test.numLogs, observedLogs.Len())
		})
	}
}

var _ kubelet.RestClient = (*fakeRestClient)(nil)

type fakeRestClient struct {
	statsSummaryFail bool
	podsFail         bool
}

func (f *fakeRestClient) StatsSummary() ([]byte, error) {
	if f.statsSummaryFail {
		return nil, errors.New("")
	}
	return os.ReadFile("testdata/stats-summary.json")
}

func (f *fakeRestClient) Pods() ([]byte, error) {
	if f.podsFail {
		return nil, errors.New("")
	}
	return os.ReadFile("testdata/pods.json")
}
