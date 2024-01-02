// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kubeletstatsreceiver

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

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
	r, err := newKubletScraper(
		&fakeRestClient{},
		receivertest.NewNopCreateSettings(),
		options,
		metadata.DefaultMetricsBuilderConfig(),
	)
	require.NoError(t, err)

	md, err := r.Scrape(context.Background())
	require.NoError(t, err)
	require.Equal(t, dataLen, md.DataPointCount())
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
			name:           "Container Metadata",
			metadataLabels: []kubelet.MetadataLabel{kubelet.MetadataLabelContainerID},
			metricGroups: map[kubelet.MetricGroup]bool{
				kubelet.ContainerMetricGroup: true,
			},
			dataLen:       numContainers * containerMetrics,
			metricPrefix:  "container.",
			requiredLabel: "container.id",
		},
		{
			name:           "Volume Metadata",
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
			r, err := newKubletScraper(
				&fakeRestClient{},
				receivertest.NewNopCreateSettings(),
				options,
				metadata.DefaultMetricsBuilderConfig(),
			)
			require.NoError(t, err)

			md, err := r.Scrape(context.Background())
			require.NoError(t, err)
			require.Equal(t, tt.dataLen, md.DataPointCount())

			for i := 0; i < md.ResourceMetrics().Len(); i++ {
				rm := md.ResourceMetrics().At(i)
				for j := 0; j < rm.ScopeMetrics().Len(); j++ {
					ilm := rm.ScopeMetrics().At(j)
					require.Equal(t, "otelcol/kubeletstatsreceiver", ilm.Scope().Name())
					for k := 0; k < ilm.Metrics().Len(); k++ {
						m := ilm.Metrics().At(k)
						if strings.HasPrefix(m.Name(), tt.metricPrefix) {
							_, ok := rm.Resource().Attributes().Get(tt.requiredLabel)
							require.True(t, ok)
							continue
						}
					}
				}
			}
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
	r, err := newKubletScraper(
		&fakeRestClient{},
		receivertest.NewNopCreateSettings(),
		options,
		metricsConfig,
	)
	require.NoError(t, err)

	md, err := r.Scrape(context.Background())
	require.NoError(t, err)
	require.Equal(t, 8, md.DataPointCount())

	currentMetric := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
	assert.Equal(t, "k8s.pod.cpu_limit_utilization", currentMetric.Name())
	assert.True(t, currentMetric.Gauge().DataPoints().At(0).DoubleValue() <= 1)
	assert.True(t, currentMetric.Gauge().DataPoints().At(0).DoubleValue() >= 0)

	currentMetric = md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(1)
	assert.Equal(t, "k8s.pod.cpu_request_utilization", currentMetric.Name())
	assert.True(t, currentMetric.Gauge().DataPoints().At(0).DoubleValue() > 1)

	currentMetric = md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(2)
	assert.Equal(t, "k8s.pod.memory_limit_utilization", currentMetric.Name())
	assert.True(t, currentMetric.Gauge().DataPoints().At(0).DoubleValue() <= 1)
	assert.True(t, currentMetric.Gauge().DataPoints().At(0).DoubleValue() >= 0)

	currentMetric = md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(3)
	assert.Equal(t, "k8s.pod.memory_request_utilization", currentMetric.Name())
	assert.True(t, currentMetric.Gauge().DataPoints().At(0).DoubleValue() > 1)

	currentMetric = md.ResourceMetrics().At(1).ScopeMetrics().At(0).Metrics().At(0)
	assert.Equal(t, "k8s.container.cpu_limit_utilization", currentMetric.Name())
	assert.True(t, currentMetric.Gauge().DataPoints().At(0).DoubleValue() <= 1)
	assert.True(t, currentMetric.Gauge().DataPoints().At(0).DoubleValue() >= 0)

	currentMetric = md.ResourceMetrics().At(1).ScopeMetrics().At(0).Metrics().At(1)
	assert.Equal(t, "k8s.container.cpu_request_utilization", currentMetric.Name())
	assert.True(t, currentMetric.Gauge().DataPoints().At(0).DoubleValue() > 1)

	currentMetric = md.ResourceMetrics().At(1).ScopeMetrics().At(0).Metrics().At(2)
	assert.Equal(t, "k8s.container.memory_limit_utilization", currentMetric.Name())
	assert.True(t, currentMetric.Gauge().DataPoints().At(0).DoubleValue() <= 1)
	assert.True(t, currentMetric.Gauge().DataPoints().At(0).DoubleValue() >= 0)

	currentMetric = md.ResourceMetrics().At(1).ScopeMetrics().At(0).Metrics().At(3)
	assert.Equal(t, "k8s.container.memory_request_utilization", currentMetric.Name())
	assert.True(t, currentMetric.Gauge().DataPoints().At(0).DoubleValue() > 1)

}

func TestScraperWithMetricGroups(t *testing.T) {
	tests := []struct {
		name         string
		metricGroups map[kubelet.MetricGroup]bool
		dataLen      int
	}{
		{
			name:         "all groups",
			metricGroups: allMetricGroups,
			dataLen:      dataLen,
		},
		{
			name: "only container group",
			metricGroups: map[kubelet.MetricGroup]bool{
				kubelet.ContainerMetricGroup: true,
			},
			dataLen: numContainers * containerMetrics,
		},
		{
			name: "only pod group",
			metricGroups: map[kubelet.MetricGroup]bool{
				kubelet.PodMetricGroup: true,
			},
			dataLen: numPods * podMetrics,
		},
		{
			name: "only node group",
			metricGroups: map[kubelet.MetricGroup]bool{
				kubelet.NodeMetricGroup: true,
			},
			dataLen: numNodes * nodeMetrics,
		},
		{
			name: "only volume group",
			metricGroups: map[kubelet.MetricGroup]bool{
				kubelet.VolumeMetricGroup: true,
			},
			dataLen: numVolumes * volumeMetrics,
		},
		{
			name: "pod and node groups",
			metricGroups: map[kubelet.MetricGroup]bool{
				kubelet.PodMetricGroup:  true,
				kubelet.NodeMetricGroup: true,
			},
			dataLen: numNodes*nodeMetrics + numPods*podMetrics,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r, err := newKubletScraper(
				&fakeRestClient{},
				receivertest.NewNopCreateSettings(),
				&scraperOptions{
					extraMetadataLabels:   []kubelet.MetadataLabel{kubelet.MetadataLabelContainerID},
					metricGroupsToCollect: test.metricGroups,
				},
				metadata.DefaultMetricsBuilderConfig(),
			)
			require.NoError(t, err)

			md, err := r.Scrape(context.Background())
			require.NoError(t, err)
			require.Equal(t, test.dataLen, md.DataPointCount())
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
			name:         "pvc doesn't exist",
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
			name:         "empty volume name in pvc",
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
			name:         "non existent volume in pvc",
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
			name:    "don't collect detailed labels",
			dataLen: numVolumes,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r, err := newKubletScraper(
				&fakeRestClient{},
				receivertest.NewNopCreateSettings(),
				&scraperOptions{
					extraMetadataLabels: []kubelet.MetadataLabel{kubelet.MetadataLabelVolumeType},
					metricGroupsToCollect: map[kubelet.MetricGroup]bool{
						kubelet.VolumeMetricGroup: true,
					},
					k8sAPIClient: test.k8sAPIClient,
				},
				metadata.DefaultMetricsBuilderConfig(),
			)
			require.NoError(t, err)

			md, err := r.Scrape(context.Background())
			require.NoError(t, err)
			require.Equal(t, test.dataLen*volumeMetrics, md.DataPointCount())

			// If Kubernetes API is set, assert additional labels as well.
			if test.k8sAPIClient != nil && len(test.expectedVolumes) > 0 {
				rms := md.ResourceMetrics()
				for i := 0; i < rms.Len(); i++ {
					resource := rms.At(i).Resource()
					claimName, ok := resource.Attributes().Get("k8s.persistentvolumeclaim.name")
					// claimName will be non empty only when PVCs are used, all test cases
					// in this method are interested only in such cases.
					if !ok {
						continue
					}

					ev := test.expectedVolumes[claimName.Str()]
					requireExpectedVolume(t, ev, resource)

					// Assert metrics from certain volume claims expected to be missed
					// are not collected.
					if test.volumeClaimsToMiss != nil {
						for c := range test.volumeClaimsToMiss {
							val, ok := resource.Attributes().Get("k8s.persistentvolumeclaim.name")
							require.True(t, !ok || val.Str() != c)
						}
					}
				}
			}
		})
	}
}

func requireExpectedVolume(t *testing.T, ev expectedVolume, resource pcommon.Resource) {
	require.NotNil(t, ev)

	requireAttribute(t, resource.Attributes(), "k8s.volume.name", ev.name)
	requireAttribute(t, resource.Attributes(), "k8s.volume.type", ev.typ)
	for k, v := range ev.labels {
		requireAttribute(t, resource.Attributes(), k, v)
	}
}

func requireAttribute(t *testing.T, attr pcommon.Map, key string, value string) {
	val, ok := attr.Get(key)
	require.True(t, ok)
	require.Equal(t, value, val.Str())

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
			settings := receivertest.NewNopCreateSettings()
			settings.Logger = logger
			options := &scraperOptions{
				extraMetadataLabels:   test.extraMetadataLabels,
				metricGroupsToCollect: test.metricGroupsToCollect,
			}
			r, err := newKubletScraper(
				&fakeRestClient{
					statsSummaryFail: test.statsSummaryFail,
					podsFail:         test.podsFail,
				},
				settings,
				options,
				metadata.DefaultMetricsBuilderConfig(),
			)
			require.NoError(t, err)

			_, err = r.Scrape(context.Background())
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
