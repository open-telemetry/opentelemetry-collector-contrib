// Copyright  OpenTelemetry Authors
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

package k8sapiserver

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sclient"
)

func NewService(name, namespace string) k8sclient.Service {
	return k8sclient.Service{ServiceName: name, Namespace: namespace}
}

var mockClient = new(MockClient)

type mockK8sClient struct {
}

func (m *mockK8sClient) GetClientSet() kubernetes.Interface {
	return fake.NewSimpleClientset()
}

func (m *mockK8sClient) GetEpClient() k8sclient.EpClient {
	return mockClient
}

func (m *mockK8sClient) GetNodeClient() k8sclient.NodeClient {
	return mockClient
}

func (m *mockK8sClient) GetPodClient() k8sclient.PodClient {
	return mockClient
}

func (m *mockK8sClient) GetDeploymentClient() k8sclient.DeploymentClient {
	return mockClient
}

func (m *mockK8sClient) GetDaemonSetClient() k8sclient.DaemonSetClient {
	return mockClient
}

func (m *mockK8sClient) ShutdownNodeClient() {

}

func (m *mockK8sClient) ShutdownPodClient() {

}

func (m *mockK8sClient) ShutdownDeploymentClient() {

}

func (m *mockK8sClient) ShutdownDaemonSetClient() {

}

type MockClient struct {
	k8sclient.PodClient
	k8sclient.NodeClient
	k8sclient.EpClient

	mock.Mock
}

// k8sclient.DeploymentClient
func (client *MockClient) DeploymentInfos() []*k8sclient.DeploymentInfo {
	args := client.Called()
	return args.Get(0).([]*k8sclient.DeploymentInfo)
}

// k8sclient.DaemonSetClient
func (client *MockClient) DaemonSetInfos() []*k8sclient.DaemonSetInfo {
	args := client.Called()
	return args.Get(0).([]*k8sclient.DaemonSetInfo)
}

// k8sclient.PodClient
func (client *MockClient) NamespaceToRunningPodNum() map[string]int {
	args := client.Called()
	return args.Get(0).(map[string]int)
}

// k8sclient.NodeClient
func (client *MockClient) ClusterFailedNodeCount() int {
	args := client.Called()
	return args.Get(0).(int)
}

func (client *MockClient) ClusterNodeCount() int {
	args := client.Called()
	return args.Get(0).(int)
}

// k8sclient.EpClient
func (client *MockClient) ServiceToPodNum() map[k8sclient.Service]int {
	args := client.Called()
	return args.Get(0).(map[k8sclient.Service]int)
}

type mockEventBroadcaster struct {
}

func (m *mockEventBroadcaster) StartRecordingToSink(sink record.EventSink) watch.Interface {
	return watch.NewFake()
}

func (m *mockEventBroadcaster) StartLogging(logf func(format string, args ...interface{})) watch.Interface {
	return watch.NewFake()
}

func (m *mockEventBroadcaster) NewRecorder(scheme *runtime.Scheme, source v1.EventSource) record.EventRecorder {
	return record.NewFakeRecorder(100)
}

func getStringAttrVal(m pmetric.Metrics, key string) string {
	rm := m.ResourceMetrics().At(0)
	attributes := rm.Resource().Attributes()
	if attributeValue, ok := attributes.Get(key); ok {
		return attributeValue.Str()
	}
	return ""
}

func assertMetricValueEqual(t *testing.T, m pmetric.Metrics, metricName string, expected int64) {
	rm := m.ResourceMetrics().At(0)
	ilms := rm.ScopeMetrics()

	for j := 0; j < ilms.Len(); j++ {
		metricSlice := ilms.At(j).Metrics()
		for i := 0; i < metricSlice.Len(); i++ {
			metric := metricSlice.At(i)
			if metric.Name() == metricName {
				if metric.Type() == pmetric.MetricTypeGauge {
					switch metric.Gauge().DataPoints().At(0).ValueType() {
					case pmetric.NumberDataPointValueTypeDouble:
						assert.Equal(t, expected, metric.Gauge().DataPoints().At(0).DoubleValue())
					case pmetric.NumberDataPointValueTypeInt:
						assert.Equal(t, expected, metric.Gauge().DataPoints().At(0).IntValue())
					}

					return
				}

				msg := fmt.Sprintf("Metric with name: %v has wrong type.", metricName)
				assert.Fail(t, msg)
			}
		}
	}

	msg := fmt.Sprintf("No metric with name: %v", metricName)
	assert.Fail(t, msg)
}

type mockClusterNameProvider struct {
}

func (m mockClusterNameProvider) GetClusterName() string {
	return "cluster-name"
}

func TestK8sAPIServer_New(t *testing.T) {
	k8sAPIServer, err := NewK8sAPIServer(mockClusterNameProvider{}, zap.NewNop(), nil)
	assert.Nil(t, k8sAPIServer)
	assert.NotNil(t, err)
}

func TestK8sAPIServer_GetMetrics(t *testing.T) {
	hostName, err := os.Hostname()
	assert.NoError(t, err)

	mockClient.On("NamespaceToRunningPodNum").Return(map[string]int{"default": 2})
	mockClient.On("ClusterFailedNodeCount").Return(1)
	mockClient.On("ClusterNodeCount").Return(1)
	mockClient.On("ServiceToPodNum").Return(
		map[k8sclient.Service]int{
			NewService("service1", "kube-system"): 1,
			NewService("service2", "kube-system"): 1,
		},
	)
	mockClient.On("DeploymentInfos").Return([]*k8sclient.DeploymentInfo{
		{
			Name:      "deployment1",
			Namespace: "kube-system",
			Spec: &k8sclient.DeploymentSpec{
				Replicas: 10,
			},
			Status: &k8sclient.DeploymentStatus{
				Replicas:            11,
				AvailableReplicas:   9,
				UnavailableReplicas: 2,
			},
		},
	})
	mockClient.On("DaemonSetInfos").Return([]*k8sclient.DaemonSetInfo{
		{
			Name:      "daemonset1",
			Namespace: "kube-system",
			Status: &k8sclient.DaemonSetStatus{
				NumberAvailable:        10,
				NumberUnavailable:      4,
				DesiredNumberScheduled: 7,
				CurrentNumberScheduled: 6,
			},
		},
	})

	leaderElection := &LeaderElection{
		k8sClient:        &mockK8sClient{},
		nodeClient:       mockClient,
		epClient:         mockClient,
		podClient:        mockClient,
		deploymentClient: mockClient,
		daemonSetClient:  mockClient,
		leading:          true,
		broadcaster:      &mockEventBroadcaster{},
		isLeadingC:       make(chan bool),
	}

	t.Setenv("HOST_NAME", hostName)
	//t.Setenv("K8S_NAMESPACE", "namespace")
	k8sAPIServer, err := NewK8sAPIServer(mockClusterNameProvider{}, zap.NewNop(), leaderElection)

	assert.NotNil(t, k8sAPIServer)
	assert.Nil(t, err)

	metrics := k8sAPIServer.GetMetrics()
	assert.NoError(t, err)

	/*
		tags: map[Timestamp:1557291396709 Type:Cluster], fields: map[cluster_failed_node_count:1 cluster_node_count:1],
		tags: map[Service:service2 Timestamp:1557291396709 Type:ClusterService], fields: map[service_number_of_running_pods:1],
		tags: map[Service:service1 Timestamp:1557291396709 Type:ClusterService], fields: map[service_number_of_running_pods:1],
		tags: map[Namespace:default Timestamp:1557291396709 Type:ClusterNamespace], fields: map[namespace_number_of_running_pods:2],
		tags: map[PodName:deployment1 Namespace:kube-system Timestamp:1557291396709 Type:ClusterDeployment], fields: map[deployment_spec_replicas:10 deployment_status_replicas:11 deployment_status_replicas_available:9 deployment_status_replicas_unavailable:2],
		tags: map[PodName:daemonset1 Namespace:kube-system Timestamp:1557291396709 Type:ClusterDaemonSet], fields: map[daemonset_status_number_available:10 daemonset_status_number_unavailable:4 daemonset_status_desired_number_scheduled:7 daemonset_status_current_number_scheduled:6],
	*/
	for _, metric := range metrics {
		assert.Equal(t, "cluster-name", getStringAttrVal(metric, ci.ClusterNameKey))
		metricType := getStringAttrVal(metric, ci.MetricType)
		switch metricType {
		case ci.TypeCluster:
			assertMetricValueEqual(t, metric, "cluster_failed_node_count", int64(1))
			assertMetricValueEqual(t, metric, "cluster_node_count", int64(1))
		case ci.TypeClusterService:
			assertMetricValueEqual(t, metric, "service_number_of_running_pods", int64(1))
			assert.Contains(t, []string{"service1", "service2"}, getStringAttrVal(metric, ci.TypeService))
			assert.Equal(t, "kube-system", getStringAttrVal(metric, ci.K8sNamespace))
		case ci.TypeClusterNamespace:
			assertMetricValueEqual(t, metric, "namespace_number_of_running_pods", int64(2))
			assert.Equal(t, "default", getStringAttrVal(metric, ci.K8sNamespace))
		case ci.TypeClusterDeployment:
			assertMetricValueEqual(t, metric, "deployment_spec_replicas", int64(10))
			assertMetricValueEqual(t, metric, "deployment_status_replicas", int64(11))
			assertMetricValueEqual(t, metric, "deployment_status_replicas_available", int64(9))
			assertMetricValueEqual(t, metric, "deployment_status_replicas_unavailable", int64(2))
			assert.Equal(t, "kube-system", getStringAttrVal(metric, ci.K8sNamespace))
			assert.Equal(t, "deployment1", getStringAttrVal(metric, ci.PodNameKey))
			assert.Equal(t, "ClusterDeployment", getStringAttrVal(metric, ci.MetricType))
		case ci.TypeClusterDaemonSet:
			assertMetricValueEqual(t, metric, "daemonset_status_number_available", int64(10))
			assertMetricValueEqual(t, metric, "daemonset_status_number_unavailable", int64(4))
			assertMetricValueEqual(t, metric, "daemonset_status_desired_number_scheduled", int64(7))
			assertMetricValueEqual(t, metric, "daemonset_status_current_number_scheduled", int64(6))
			assert.Equal(t, "kube-system", getStringAttrVal(metric, ci.K8sNamespace))
			assert.Equal(t, "daemonset1", getStringAttrVal(metric, ci.PodNameKey))
			assert.Equal(t, "ClusterDaemonSet", getStringAttrVal(metric, ci.MetricType))
		default:
			assert.Fail(t, "Unexpected metric type: "+metricType)
		}
	}

	k8sAPIServer.Shutdown()
}
