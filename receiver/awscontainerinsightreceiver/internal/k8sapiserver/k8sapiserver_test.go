// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sapiserver

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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

func (m *mockK8sClient) ShutdownNodeClient() {

}

func (m *mockK8sClient) ShutdownPodClient() {

}

type MockClient struct {
	k8sclient.PodClient
	k8sclient.NodeClient
	k8sclient.EpClient

	mock.Mock
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

func (m *mockEventBroadcaster) StartRecordingToSink(_ record.EventSink) watch.Interface {
	return watch.NewFake()
}

func (m *mockEventBroadcaster) StartLogging(_ func(format string, args ...any)) watch.Interface {
	return watch.NewFake()
}

func (m *mockEventBroadcaster) NewRecorder(_ *runtime.Scheme, _ v1.EventSource) record.EventRecorder {
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

type MockClusterNameProvicer struct {
}

func (m MockClusterNameProvicer) GetClusterName() string {
	return "cluster-name"
}

func TestK8sAPIServer_New(t *testing.T) {
	k8sClientOption := func(k *K8sAPIServer) {
		k.k8sClient = nil
	}
	k8sAPIServer, err := New(MockClusterNameProvicer{}, zap.NewNop(), k8sClientOption)
	assert.Nil(t, k8sAPIServer)
	assert.NotNil(t, err)
}

func TestK8sAPIServer_GetMetrics(t *testing.T) {
	hostName, err := os.Hostname()
	assert.NoError(t, err)
	k8sClientOption := func(k *K8sAPIServer) {
		k.k8sClient = &mockK8sClient{}
	}
	leadingOption := func(k *K8sAPIServer) {
		k.leading = true
	}
	broadcasterOption := func(k *K8sAPIServer) {
		k.broadcaster = &mockEventBroadcaster{}
	}
	isLeadingCOption := func(k *K8sAPIServer) {
		k.isLeadingC = make(chan bool)
	}

	t.Setenv("HOST_NAME", hostName)
	t.Setenv("K8S_NAMESPACE", "namespace")
	k8sAPIServer, err := New(MockClusterNameProvicer{}, zap.NewNop(), k8sClientOption,
		leadingOption, broadcasterOption, isLeadingCOption)

	assert.NotNil(t, k8sAPIServer)
	assert.Nil(t, err)

	mockClient.On("NamespaceToRunningPodNum").Return(map[string]int{"default": 2})
	mockClient.On("ClusterFailedNodeCount").Return(1)
	mockClient.On("ClusterNodeCount").Return(1)
	mockClient.On("ServiceToPodNum").Return(
		map[k8sclient.Service]int{
			NewService("service1", "kube-system"): 1,
			NewService("service2", "kube-system"): 1,
		},
	)

	<-k8sAPIServer.isLeadingC
	metrics := k8sAPIServer.GetMetrics()
	assert.NoError(t, err)

	/*
		tags: map[Timestamp:1557291396709 Type:Cluster], fields: map[cluster_failed_node_count:1 cluster_node_count:1],
		tags: map[Service:service2 Timestamp:1557291396709 Type:ClusterService], fields: map[service_number_of_running_pods:1],
		tags: map[Service:service1 Timestamp:1557291396709 Type:ClusterService], fields: map[service_number_of_running_pods:1],
		tags: map[Namespace:default Timestamp:1557291396709 Type:ClusterNamespace], fields: map[namespace_number_of_running_pods:2],
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
		default:
			assert.Fail(t, "Unexpected metric type: "+metricType)
		}
	}

	require.NoError(t, k8sAPIServer.Shutdown())
}

func TestK8sAPIServer_init(t *testing.T) {
	k8sAPIServer := &K8sAPIServer{}

	err := k8sAPIServer.init()
	assert.NotNil(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "environment variable HOST_NAME is not set"))

	t.Setenv("HOST_NAME", "hostname")

	err = k8sAPIServer.init()
	assert.NotNil(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "environment variable K8S_NAMESPACE is not set"))
}
