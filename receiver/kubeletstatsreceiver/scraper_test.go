// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubeletstatsreceiver

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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

const (
	dataLen = numContainers*containerMetrics + numPods*podMetrics + numNodes*nodeMetrics + numVolumes*volumeMetrics

	// Number of resources by type in testdata/stats-summary.json
	numContainers = 9
	numPods       = 9
	numNodes      = 1
	numVolumes    = 8

	// Number of metrics by resource
	nodeMetrics      = 15
	podMetrics       = 15
	containerMetrics = 11
	volumeMetrics    = 5
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
		metadata.DefaultMetricsBuilderConfig(),
		"worker-42",
	)
	require.NoError(t, err)

	md, err := r.ScrapeMetrics(context.Background())
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
		metadata.DefaultMetricsBuilderConfig(),
		"worker-42",
	)
	require.NoError(t, err)

	md, err := r.ScrapeMetrics(context.Background())
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
	client := fake.NewSimpleClientset()
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
				K8sContainerCPUNodeUtilization: metadata.MetricConfig{
					Enabled: true,
				},
				K8sPodCPUNodeUtilization: metadata.MetricConfig{
					Enabled: true,
				},
			},
			ResourceAttributes: metadata.DefaultResourceAttributesConfig(),
		},
		"worker-42",
	)
	require.NoError(t, err)

	err = r.Start(context.Background(), nil)
	require.NoError(t, err)

	// we wait until the watcher starts
	<-watcherStarted
	// Inject an event node into the fake client.
	node := getNodeWithCPUCapacity("worker-42", 8)
	_, err = client.CoreV1().Nodes().Create(context.TODO(), node, metav1.CreateOptions{})
	if err != nil {
		require.NoError(t, err)
	}

	var md pmetric.Metrics
	require.Eventually(t, func() bool {
		md, err = r.ScrapeMetrics(context.Background())
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

	err = r.Shutdown(context.Background())
	require.NoError(t, err)
}

func TestScraperWithMemoryNodeUtilization(t *testing.T) {
	watcherStarted := make(chan struct{})
	// Create the fake client.
	client := fake.NewSimpleClientset()
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
				K8sContainerMemoryNodeUtilization: metadata.MetricConfig{
					Enabled: true,
				}, K8sPodMemoryNodeUtilization: metadata.MetricConfig{
					Enabled: true,
				},
			},
			ResourceAttributes: metadata.DefaultResourceAttributesConfig(),
		},
		"worker-42",
	)
	require.NoError(t, err)

	err = r.Start(context.Background(), nil)
	require.NoError(t, err)

	// we wait until the watcher starts
	<-watcherStarted
	// Inject an event node into the fake client.
	node := getNodeWithMemoryCapacity("worker-42", "32564740Ki")
	_, err = client.CoreV1().Nodes().Create(context.TODO(), node, metav1.CreateOptions{})
	if err != nil {
		require.NoError(t, err)
	}

	var md pmetric.Metrics
	require.Eventually(t, func() bool {
		md, err = r.ScrapeMetrics(context.Background())
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

	err = r.Shutdown(context.Background())
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
				metadata.DefaultMetricsBuilderConfig(),
				"worker-42",
			)
			require.NoError(t, err)

			md, err := r.ScrapeMetrics(context.Background())
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
			ContainerCPUTime: metadata.MetricConfig{
				Enabled: false,
			},
			ContainerCPUUtilization: metadata.MetricConfig{
				Enabled: false,
			},
			ContainerFilesystemAvailable: metadata.MetricConfig{
				Enabled: false,
			},
			ContainerFilesystemCapacity: metadata.MetricConfig{
				Enabled: false,
			},
			ContainerFilesystemUsage: metadata.MetricConfig{
				Enabled: false,
			},
			ContainerMemoryAvailable: metadata.MetricConfig{
				Enabled: false,
			},
			ContainerMemoryMajorPageFaults: metadata.MetricConfig{
				Enabled: false,
			},
			ContainerMemoryPageFaults: metadata.MetricConfig{
				Enabled: false,
			},
			ContainerMemoryRss: metadata.MetricConfig{
				Enabled: false,
			},
			ContainerMemoryUsage: metadata.MetricConfig{
				Enabled: false,
			},
			K8sContainerCPULimitUtilization: metadata.MetricConfig{
				Enabled: true,
			},
			K8sContainerCPURequestUtilization: metadata.MetricConfig{
				Enabled: true,
			},
			K8sContainerMemoryLimitUtilization: metadata.MetricConfig{
				Enabled: true,
			},
			K8sContainerMemoryRequestUtilization: metadata.MetricConfig{
				Enabled: true,
			},
			ContainerMemoryWorkingSet: metadata.MetricConfig{
				Enabled: false,
			},
			K8sNodeCPUTime: metadata.MetricConfig{
				Enabled: false,
			},
			K8sNodeCPUUtilization: metadata.MetricConfig{
				Enabled: false,
			},
			K8sNodeFilesystemAvailable: metadata.MetricConfig{
				Enabled: false,
			},
			K8sNodeFilesystemCapacity: metadata.MetricConfig{
				Enabled: false,
			},
			K8sNodeFilesystemUsage: metadata.MetricConfig{
				Enabled: false,
			},
			K8sNodeMemoryAvailable: metadata.MetricConfig{
				Enabled: false,
			},
			K8sNodeMemoryMajorPageFaults: metadata.MetricConfig{
				Enabled: false,
			},
			K8sNodeMemoryPageFaults: metadata.MetricConfig{
				Enabled: false,
			},
			K8sNodeMemoryRss: metadata.MetricConfig{
				Enabled: false,
			},
			K8sNodeMemoryUsage: metadata.MetricConfig{
				Enabled: false,
			},
			K8sNodeMemoryWorkingSet: metadata.MetricConfig{
				Enabled: false,
			},
			K8sNodeNetworkErrors: metadata.MetricConfig{
				Enabled: false,
			},
			K8sNodeNetworkIo: metadata.MetricConfig{
				Enabled: false,
			},
			K8sPodCPUTime: metadata.MetricConfig{
				Enabled: false,
			},
			K8sPodCPUUtilization: metadata.MetricConfig{
				Enabled: false,
			},
			K8sPodFilesystemAvailable: metadata.MetricConfig{
				Enabled: false,
			},
			K8sPodFilesystemCapacity: metadata.MetricConfig{
				Enabled: false,
			},
			K8sPodFilesystemUsage: metadata.MetricConfig{
				Enabled: false,
			},
			K8sPodMemoryAvailable: metadata.MetricConfig{
				Enabled: false,
			},
			K8sPodMemoryMajorPageFaults: metadata.MetricConfig{
				Enabled: false,
			},
			K8sPodMemoryPageFaults: metadata.MetricConfig{
				Enabled: false,
			},
			K8sPodMemoryRss: metadata.MetricConfig{
				Enabled: false,
			},
			K8sPodMemoryUsage: metadata.MetricConfig{
				Enabled: false,
			},
			K8sPodCPULimitUtilization: metadata.MetricConfig{
				Enabled: true,
			},
			K8sPodCPURequestUtilization: metadata.MetricConfig{
				Enabled: true,
			},
			K8sPodMemoryLimitUtilization: metadata.MetricConfig{
				Enabled: true,
			},
			K8sPodMemoryRequestUtilization: metadata.MetricConfig{
				Enabled: true,
			},
			K8sPodMemoryWorkingSet: metadata.MetricConfig{
				Enabled: false,
			},
			K8sPodNetworkErrors: metadata.MetricConfig{
				Enabled: false,
			},
			K8sPodNetworkIo: metadata.MetricConfig{
				Enabled: false,
			},
			K8sVolumeAvailable: metadata.MetricConfig{
				Enabled: false,
			},
			K8sVolumeCapacity: metadata.MetricConfig{
				Enabled: false,
			},
			K8sVolumeInodes: metadata.MetricConfig{
				Enabled: false,
			},
			K8sVolumeInodesFree: metadata.MetricConfig{
				Enabled: false,
			},
			K8sVolumeInodesUsed: metadata.MetricConfig{
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

	md, err := r.ScrapeMetrics(context.Background())
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
				metadata.DefaultMetricsBuilderConfig(),
				"worker-42",
			)
			require.NoError(t, err)

			md, err := r.ScrapeMetrics(context.Background())
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
			k8sAPIClient: fake.NewSimpleClientset(getValidMockedObjects()...),
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
			k8sAPIClient: fake.NewSimpleClientset(),
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
			k8sAPIClient: fake.NewSimpleClientset(getMockedObjectsWithEmptyVolumeName()...),
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
			k8sAPIClient: fake.NewSimpleClientset(getMockedObjectsWithNonExistentVolumeName()...),
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
				metadata.DefaultMetricsBuilderConfig(),
				"worker-42",
			)
			require.NoError(t, err)

			md, err := r.ScrapeMetrics(context.Background())
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
				metadata.DefaultMetricsBuilderConfig(),
				"",
			)
			require.NoError(t, err)

			_, err = r.ScrapeMetrics(context.Background())
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
