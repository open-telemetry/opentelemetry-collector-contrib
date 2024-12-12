// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sapiserver

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sclient"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sutil"
)

func NewService(name, namespace string) k8sclient.Service {
	return k8sclient.Service{ServiceName: name, Namespace: namespace}
}

var mockClient = new(MockClient)

type mockK8sClient struct{}

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

func (m *mockK8sClient) GetStatefulSetClient() k8sclient.StatefulSetClient {
	return mockClient
}

func (m *mockK8sClient) GetReplicaSetClient() k8sclient.ReplicaSetClient {
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

func (m *mockK8sClient) ShutdownStatefulSetClient() {

}

func (m *mockK8sClient) ShutdownReplicaSetClient() {

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

// k8sclient.StatefulSetClient
func (client *MockClient) StatefulSetInfos() []*k8sclient.StatefulSetInfo {
	args := client.Called()
	return args.Get(0).([]*k8sclient.StatefulSetInfo)
}

// k8sclient.ReplicaSetClient
func (client *MockClient) ReplicaSetInfos() []*k8sclient.ReplicaSetInfo {
	args := client.Called()
	return args.Get(0).([]*k8sclient.ReplicaSetInfo)
}

func (client *MockClient) ReplicaSetToDeployment() map[string]string {
	args := client.Called()
	return args.Get(0).(map[string]string)
}

// k8sclient.PodClient
func (client *MockClient) NamespaceToRunningPodNum() map[string]int {
	args := client.Called()
	return args.Get(0).(map[string]int)
}

// k8sclient.PodClient
func (client *MockClient) PodInfos() []*k8sclient.PodInfo {
	args := client.Called()
	return args.Get(0).([]*k8sclient.PodInfo)
}

// k8sclient.NodeClient
func (client *MockClient) ClusterFailedNodeCount() int {
	args := client.Called()
	return args.Get(0).(int)
}

func (client *MockClient) NodeInfos() map[string]*k8sclient.NodeInfo {
	args := client.Called()
	return args.Get(0).(map[string]*k8sclient.NodeInfo)
}

func (client *MockClient) ClusterNodeCount() int {
	args := client.Called()
	return args.Get(0).(int)
}

func (client *MockClient) NodeToLabelsMap() map[string]map[k8sclient.Label]int8 {
	args := client.Called()
	return args.Get(0).(map[string]map[k8sclient.Label]int8)
}

// k8sclient.EpClient
func (client *MockClient) ServiceToPodNum() map[k8sclient.Service]int {
	args := client.Called()
	return args.Get(0).(map[k8sclient.Service]int)
}

// k8sclient.EpClient
func (client *MockClient) PodKeyToServiceNames() map[string][]string {
	args := client.Called()
	return args.Get(0).(map[string][]string)
}

type mockEventBroadcaster struct {}

func (m *mockEventBroadcaster) StartRecordingToSink(_ record.EventSink) watch.Interface {
	return watch.NewFake()
}

func (m *mockEventBroadcaster) StartLogging(_ func(format string, args ...any)) watch.Interface {
	return watch.NewFake()
}

func (m *mockEventBroadcaster) NewRecorder(_ *runtime.Scheme, _ v1.EventSource) record.EventRecorderLogger {
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
					case pmetric.NumberDataPointValueTypeEmpty:
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

type mockClusterNameProvider struct {}

func (m mockClusterNameProvider) GetClusterName() string {
	return "cluster-name"
}

func TestK8sAPIServer_New(t *testing.T) {
	k8sAPIServer, err := NewK8sAPIServer(mockClusterNameProvider{}, zap.NewNop(), nil, false, false)
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
				Replicas: 11,
			},
			Status: &k8sclient.DeploymentStatus{
				Replicas:            11,
				ReadyReplicas:       10,
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
	mockClient.On("ReplicaSetInfos").Return([]*k8sclient.ReplicaSetInfo{
		{
			Name:      "replicaset1",
			Namespace: "kube-system",
			Spec: &k8sclient.ReplicaSetSpec{
				Replicas: 9,
			},
			Status: &k8sclient.ReplicaSetStatus{
				Replicas:          5,
				ReadyReplicas:     4,
				AvailableReplicas: 3,
			},
		},
	})
	mockClient.On("StatefulSetInfos").Return([]*k8sclient.StatefulSetInfo{
		{
			Name:      "statefulset1",
			Namespace: "kube-system",
			Spec: &k8sclient.StatefulSetSpec{
				Replicas: 10,
			},
			Status: &k8sclient.StatefulSetStatus{
				Replicas:          3,
				ReadyReplicas:     4,
				AvailableReplicas: 1,
			},
		},
	})
	mockClient.On("PodInfos").Return([]*k8sclient.PodInfo{
		{
			Name:      "kube-proxy-csm88",
			Namespace: "kube-system",
			UID:       "bc5f5839-f62e-44b9-a79e-af250d92dcb1",
			Phase:     v1.PodPending,
		},
	})
	mockClient.On("PodKeyToServiceNames").Return(map[string][]string{
		"namespace:kube-system,podName:coredns-7554568866-26jdf": {"kube-dns"},
		"namespace:kube-system,podName:coredns-7554568866-shwn6": {"kube-dns"},
	})

	mockClient.On("NodeInfos").Return(map[string]*k8sclient.NodeInfo{
		"ip-192-168-57-23.us-west-2.compute.internal": {
			Name: "ip-192-168-57-23.us-west-2.compute.internal",
			Conditions: []*k8sclient.NodeCondition{
				{
					Type:   v1.NodeReady,
					Status: v1.ConditionTrue,
				},
			},
			Capacity:     map[v1.ResourceName]resource.Quantity{},
			Allocatable:  map[v1.ResourceName]resource.Quantity{},
			InstanceType: "ml.g4dn-12xl",
			Labels: map[k8sclient.Label]int8{
				k8sclient.SageMakerNodeHealthStatus: int8(k8sutil.Schedulable),
			},
		},
	})

	mockClient.On("NodeToLabelsMap").Return(map[string]map[k8sclient.Label]int8{
		"ip-192-168-57-23.us-west-2.compute.internal": {
			k8sclient.SageMakerNodeHealthStatus: int8(k8sutil.Schedulable),
		},
	})

	leaderElection := &LeaderElection{
		k8sClient:         &mockK8sClient{},
		nodeClient:        mockClient,
		epClient:          mockClient,
		podClient:         mockClient,
		deploymentClient:  mockClient,
		daemonSetClient:   mockClient,
		statefulSetClient: mockClient,
		replicaSetClient:  mockClient,
		leading:           true,
		broadcaster:       &mockEventBroadcaster{},
		isLeadingC:        make(chan struct{}),
	}

	t.Setenv("HOST_NAME", hostName)
	t.Setenv("K8S_NAMESPACE", "namespace")
	k8sAPIServer, err := NewK8sAPIServer(mockClusterNameProvider{}, zap.NewNop(), leaderElection, true, true)

	assert.NotNil(t, k8sAPIServer)
	assert.Nil(t, err)

	metrics := k8sAPIServer.GetMetrics()
	assert.NoError(t, err)

	/*
		tags: map[Timestamp:1557291396709 Type:Cluster], fields: map[cluster_failed_node_count:1 cluster_node_count:1],
		tags: map[Service:service2 Timestamp:1557291396709 Type:ClusterService], fields: map[service_number_of_running_pods:1],
		tags: map[Service:service1 Timestamp:1557291396709 Type:ClusterService], fields: map[service_number_of_running_pods:1],
		tags: map[Namespace:default Timestamp:1557291396709 Type:ClusterNamespace], fields: map[namespace_number_of_running_pods:2],
		tags: map[PodName:deployment1 Namespace:kube-system Timestamp:1557291396709 Type:ClusterDeployment], fields: map[replicas_desired:11 replicas_ready:10 status_replicas_available:9 status_replicas_unavailable:2],
		tags: map[PodName:daemonset1 Namespace:kube-system Timestamp:1557291396709 Type:ClusterDaemonSet], fields: map[status_replicas_available:10 status_replicas_unavailable:4 replicas_desired:7 replicas_ready:6],
		tags: map[PodName:replicaset1 Namespace:kube-system Timestamp:1557291396709 Type:ClusterDaemonSet], fields: map[status_replicas_available:3 replicas_desired:9 replicas_ready:4],
		tags: map[PodName:statefulset1 Namespace:kube-system Timestamp:1557291396709 Type:ClusterDaemonSet], fields: map[status_replicas_available:1 replicas_desired:10 replicas_ready:4],
	*/
	for _, metric := range metrics {
		assert.Equal(t, "cluster-name", getStringAttrVal(metric, ci.ClusterNameKey))
		metricType := getStringAttrVal(metric, ci.MetricType)
		switch metricType {
		case ci.TypeCluster:
			assertMetricValueEqual(t, metric, "cluster_failed_node_count", int64(1))
			assertMetricValueEqual(t, metric, "cluster_node_count", int64(1))
			assertMetricValueEqual(t, metric, "cluster_number_of_running_pods", int64(2))

		case ci.TypeClusterService:
			assertMetricValueEqual(t, metric, "service_number_of_running_pods", int64(1))
			assert.Contains(t, []string{"service1", "service2"}, getStringAttrVal(metric, ci.TypeService))
			assert.Equal(t, "kube-system", getStringAttrVal(metric, ci.AttributeK8sNamespace))
		case ci.TypeClusterNamespace:
			assertMetricValueEqual(t, metric, "namespace_number_of_running_pods", int64(2))
			assert.Equal(t, "default", getStringAttrVal(metric, ci.AttributeK8sNamespace))
		case ci.TypeClusterDeployment:
			assertMetricValueEqual(t, metric, "replicas_desired", int64(11))
			assertMetricValueEqual(t, metric, "replicas_ready", int64(10))
			assertMetricValueEqual(t, metric, "status_replicas_available", int64(9))
			assertMetricValueEqual(t, metric, "status_replicas_unavailable", int64(2))
			assert.Equal(t, "kube-system", getStringAttrVal(metric, ci.AttributeK8sNamespace))
			assert.Equal(t, "deployment1", getStringAttrVal(metric, ci.AttributePodName))
			assert.Equal(t, "ClusterDeployment", getStringAttrVal(metric, ci.MetricType))
		case ci.TypeClusterDaemonSet:
			assertMetricValueEqual(t, metric, "replicas_desired", int64(7))
			assertMetricValueEqual(t, metric, "replicas_ready", int64(6))
			assertMetricValueEqual(t, metric, "status_replicas_available", int64(10))
			assertMetricValueEqual(t, metric, "status_replicas_unavailable", int64(4))
			assert.Equal(t, "kube-system", getStringAttrVal(metric, ci.AttributeK8sNamespace))
			assert.Equal(t, "daemonset1", getStringAttrVal(metric, ci.AttributePodName))
			assert.Equal(t, "ClusterDaemonSet", getStringAttrVal(metric, ci.MetricType))
		case ci.TypeClusterReplicaSet:
			assertMetricValueEqual(t, metric, "replicas_desired", int64(9))
			assertMetricValueEqual(t, metric, "replicas_ready", int64(4))
			assertMetricValueEqual(t, metric, "status_replicas_available", int64(3))
			assert.Equal(t, "kube-system", getStringAttrVal(metric, ci.AttributeK8sNamespace))
			assert.Equal(t, "replicaset1", getStringAttrVal(metric, ci.AttributePodName))
			assert.Equal(t, "ClusterReplicaSet", getStringAttrVal(metric, ci.MetricType))
		case ci.TypeClusterStatefulSet:
			assertMetricValueEqual(t, metric, "replicas_desired", int64(10))
			assertMetricValueEqual(t, metric, "replicas_ready", int64(4))
			assertMetricValueEqual(t, metric, "status_replicas_available", int64(1))
			assert.Equal(t, "kube-system", getStringAttrVal(metric, ci.AttributeK8sNamespace))
			assert.Equal(t, "statefulset1", getStringAttrVal(metric, ci.AttributePodName))
			assert.Equal(t, "ClusterStatefulSet", getStringAttrVal(metric, ci.MetricType))
		case ci.TypePod:
			assertMetricValueEqual(t, metric, "pod_status_pending", int64(1))
			assertMetricValueEqual(t, metric, "pod_status_running", int64(0))
			assertMetricValueEqual(t, metric, "pod_status_failed", int64(0))
			assertMetricValueEqual(t, metric, "pod_status_ready", int64(0))
			assertMetricValueEqual(t, metric, "pod_status_scheduled", int64(0))
			assertMetricValueEqual(t, metric, "pod_status_succeeded", int64(0))
			assertMetricValueEqual(t, metric, "pod_status_unknown", int64(0))
			assert.Equal(t, "kube-system", getStringAttrVal(metric, ci.AttributeK8sNamespace))
			assert.Equal(t, "Pending", getStringAttrVal(metric, "pod_status"))
			assert.Equal(t, "Pod", getStringAttrVal(metric, ci.MetricType))
		case ci.TypeHyperPodNode:
			assert.Equal(t, "HyperPodNode", getStringAttrVal(metric, ci.MetricType))
			assert.Equal(t, "ip-192-168-57-23.us-west-2.compute.internal", getStringAttrVal(metric, ci.NodeNameKey))
			assert.Equal(t, "ip-192-168-57-23.us-west-2.compute.internal", getStringAttrVal(metric, ci.InstanceID))
			assertMetricValueEqual(t, metric, "hyperpod_node_health_status_unschedulable_pending_reboot", int64(0))
			assertMetricValueEqual(t, metric, "hyperpod_node_health_status_schedulable", int64(1))
			assertMetricValueEqual(t, metric, "hyperpod_node_health_status_unschedulable", int64(0))
			assertMetricValueEqual(t, metric, "hyperpod_node_health_status_unschedulable_pending_replacement", int64(0))
		default:
			assert.Fail(t, "Unexpected metric type: "+metricType)
		}
	}

	require.NoError(t, k8sAPIServer.Shutdown())
}
