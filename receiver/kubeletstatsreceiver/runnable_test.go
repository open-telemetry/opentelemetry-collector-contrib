// Copyright 2020, OpenTelemetry Authors
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

package kubeletstatsreceiver

import (
	"context"
	"errors"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/translator/internaldata"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/kubelet"
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

func TestRunnable(t *testing.T) {
	consumer := new(consumertest.MetricsSink)
	options := &receiverOptions{
		metricGroupsToCollect: allMetricGroups,
	}
	r := newRunnable(
		context.Background(),
		consumer,
		&fakeRestClient{},
		zap.NewNop(),
		options,
	)
	err := r.Setup()
	require.NoError(t, err)
	err = r.Run()
	require.NoError(t, err)
	require.Equal(t, dataLen, consumer.MetricsCount())
}

func TestRunnableWithMetadata(t *testing.T) {
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
			consumer := new(consumertest.MetricsSink)
			options := &receiverOptions{
				extraMetadataLabels:   tt.metadataLabels,
				metricGroupsToCollect: tt.metricGroups,
			}
			r := newRunnable(
				context.Background(),
				consumer,
				&fakeRestClient{},
				zap.NewNop(),
				options,
			)
			err := r.Setup()
			require.NoError(t, err)
			err = r.Run()
			require.NoError(t, err)
			require.Equal(t, tt.dataLen, consumer.MetricsCount())

			for _, metrics := range consumer.AllMetrics() {
				md := internaldata.MetricsToOC(metrics)[0]
				for _, m := range md.Metrics {
					if strings.HasPrefix(m.MetricDescriptor.GetName(), tt.metricPrefix) {
						_, ok := md.Resource.Labels[tt.requiredLabel]
						require.True(t, ok)
						continue
					}
				}
			}
		})
	}
}

func TestRunnableWithMetricGroups(t *testing.T) {
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
			consumer := new(consumertest.MetricsSink)
			r := newRunnable(
				context.Background(),
				consumer,
				&fakeRestClient{},
				zap.NewNop(),
				&receiverOptions{
					extraMetadataLabels:   []kubelet.MetadataLabel{kubelet.MetadataLabelContainerID},
					metricGroupsToCollect: test.metricGroups,
				},
			)

			err := r.Setup()
			require.NoError(t, err)

			err = r.Run()
			require.NoError(t, err)

			require.Equal(t, test.dataLen, consumer.MetricsCount())
		})
	}
}

type expectedVolume struct {
	name   string
	typ    string
	labels map[string]string
}

func TestRunnableWithPVCDetailedLabels(t *testing.T) {
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
			consumer := new(consumertest.MetricsSink)
			r := newRunnable(
				context.Background(),
				consumer,
				&fakeRestClient{},
				zap.NewNop(),
				&receiverOptions{
					extraMetadataLabels: []kubelet.MetadataLabel{kubelet.MetadataLabelVolumeType},
					metricGroupsToCollect: map[kubelet.MetricGroup]bool{
						kubelet.VolumeMetricGroup: true,
					},
					k8sAPIClient: test.k8sAPIClient,
				},
			)

			err := r.Setup()
			require.NoError(t, err)

			err = r.Run()
			require.NoError(t, err)
			require.Equal(t, test.dataLen*volumeMetrics, consumer.MetricsCount())

			// If Kubernetes API is set, assert additional labels as well.
			if test.k8sAPIClient != nil && len(test.expectedVolumes) > 0 {
				for _, m := range consumer.AllMetrics() {
					mds := internaldata.MetricsToOC(m)
					for _, md := range mds {
						claimName, ok := md.Resource.Labels["k8s.persistentvolumeclaim.name"]
						// claimName will be non empty only when PVCs are used, all test cases
						// in this method are interested only in such cases.
						if !ok {
							continue
						}

						ev := test.expectedVolumes[claimName]
						requireExpectedVolume(t, ev, md.Resource.Labels)

						// Assert metrics from certain volume claims expected to be missed
						// are not collected.
						if test.volumeClaimsToMiss != nil {
							for c := range test.volumeClaimsToMiss {
								require.False(t, md.Resource.Labels["k8s.persistentvolumeclaim.name"] == c)
							}
						}
					}
				}
			}
		})
	}
}

func requireExpectedVolume(t *testing.T, ev expectedVolume, resourceLabels map[string]string) {
	require.NotNil(t, ev)
	require.Equal(t, ev.name, resourceLabels["k8s.volume.name"])
	require.Equal(t, ev.typ, resourceLabels["k8s.volume.type"])
	for k, v := range ev.labels {
		require.Equal(t, resourceLabels[k], v)
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
			options := &receiverOptions{
				extraMetadataLabels:   test.extraMetadataLabels,
				metricGroupsToCollect: test.metricGroupsToCollect,
			}
			r := newRunnable(
				context.Background(),
				new(consumertest.MetricsSink),
				&fakeRestClient{
					statsSummaryFail: test.statsSummaryFail,
					podsFail:         test.podsFail,
				},
				zap.New(core),
				options,
			)
			err := r.Setup()
			require.NoError(t, err)
			err = r.Run()
			require.NoError(t, err)
			require.Equal(t, test.numLogs, observedLogs.Len())
		})
	}
}

func TestConsumerErrors(t *testing.T) {
	tests := []struct {
		name    string
		fail    bool
		numLogs int
	}{
		{"no error", false, 0},
		{"consume error", true, 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			core, observedLogs := observer.New(zap.ErrorLevel)
			options := &receiverOptions{
				extraMetadataLabels:   nil,
				metricGroupsToCollect: allMetricGroups,
			}
			sink := new(consumertest.MetricsSink)
			if test.fail {
				sink.SetConsumeError(errors.New("failed"))
			}
			r := newRunnable(
				context.Background(),
				sink,
				&fakeRestClient{},
				zap.New(core),
				options,
			)
			err := r.Setup()
			require.NoError(t, err)
			err = r.Run()
			require.NoError(t, err)
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
	return ioutil.ReadFile("testdata/stats-summary.json")
}

func (f *fakeRestClient) Pods() ([]byte, error) {
	if f.podsFail {
		return nil, errors.New("")
	}
	return ioutil.ReadFile("testdata/pods.json")
}
