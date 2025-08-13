// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package stores

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	ci "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s/k8sclient"
)

func getBaseTestPodInfo() *corev1.Pod {
	podJSON := `
{
  "kind": "PodList",
  "apiVersion": "v1",
  "metadata": {

  },
  "items": [
    {
      "metadata": {
        "name": "cpu-limit",
        "namespace": "default",
        "ownerReferences": [
            {
                "apiVersion": "apps/v1",
                "blockOwnerDeletion": true,
                "controller": true,
                "kind": "DaemonSet",
                "name": "DaemonSetTest",
                "uid": "36779a62-4aca-11e9-977b-0672b6c6fc94"
            }
        ],
        "selfLink": "/api/v1/namespaces/default/pods/cpu-limit",
        "uid": "764d01e1-2a2f-11e9-95ea-0a695d7ce286",
        "resourceVersion": "5671573",
        "creationTimestamp": "2019-02-06T16:51:34Z",
        "labels": {
          "app": "hello_test"
        },
        "annotations": {
          "kubernetes.io/config.seen": "2019-02-19T00:06:56.109155665Z",
          "kubernetes.io/config.source": "api"
        }
      },
      "spec": {
        "volumes": [
          {
            "name": "default-token-tlgw7",
            "secret": {
              "secretName": "default-token-tlgw7",
              "defaultMode": 420
            }
          }
        ],
        "containers": [
          {
            "name": "ubuntu",
            "image": "ubuntu",
            "command": [
              "/bin/bash"
            ],
            "args": [
              "-c",
              "sleep 300000000"
            ],
            "resources": {
              "limits": {
                "cpu": "10m",
                "memory": "50Mi",
				"nvidia.com/gpu": "1"
              },
              "requests": {
                "cpu": "10m",
                "memory": "50Mi",
				"nvidia.com/gpu": "1"
              }
            },
            "volumeMounts": [
              {
                "name": "default-token-tlgw7",
                "readOnly": true,
                "mountPath": "/var/run/secrets/kubernetes.io/serviceaccount"
              }
            ],
            "terminationMessagePath": "/dev/termination-log",
            "terminationMessagePolicy": "File",
            "imagePullPolicy": "Always"
          }
        ],
        "restartPolicy": "Always",
        "terminationGracePeriodSeconds": 30,
        "dnsPolicy": "ClusterFirst",
        "serviceAccountName": "default",
        "serviceAccount": "default",
        "nodeName": "ip-192-168-67-127.us-west-2.compute.internal",
        "securityContext": {

        },
        "schedulerName": "default-scheduler",
        "tolerations": [
          {
            "key": "node.kubernetes.io/not-ready",
            "operator": "Exists",
            "effect": "NoExecute",
            "tolerationSeconds": 300
          },
          {
            "key": "node.kubernetes.io/unreachable",
            "operator": "Exists",
            "effect": "NoExecute",
            "tolerationSeconds": 300
          }
        ],
        "priority": 0
      },
      "status": {
        "phase": "Running",
        "conditions": [
          {
            "type": "Initialized",
            "status": "True",
            "lastProbeTime": null,
            "lastTransitionTime": "2019-02-06T16:51:34Z"
          },
          {
            "type": "Ready",
            "status": "True",
            "lastProbeTime": null,
            "lastTransitionTime": "2019-02-06T16:51:43Z"
          },
          {
            "type": "ContainersReady",
            "status": "True",
            "lastProbeTime": null,
            "lastTransitionTime": null
          },
          {
            "type": "PodScheduled",
            "status": "True",
            "lastProbeTime": null,
            "lastTransitionTime": "2019-02-06T16:51:34Z"
          }
        ],
        "hostIP": "192.168.67.127",
        "podIP": "192.168.76.93",
        "startTime": "2019-02-06T16:51:34Z",
        "containerStatuses": [
          {
            "name": "ubuntu",
            "state": {
              "running": {
                "startedAt": "2019-02-06T16:51:42Z"
              }
            },
            "lastState": {

            },
            "ready": true,
            "restartCount": 0,
            "image": "ubuntu:latest",
            "imageID": "docker-pullable://ubuntu@sha256:7a47ccc3bbe8a451b500d2b53104868b46d60ee8f5b35a24b41a86077c650210",
            "containerID": "docker://637631e2634ea92c0c1aa5d24734cfe794f09c57933026592c12acafbaf6972c"
          }
        ],
        "qosClass": "Guaranteed"
      }
    }
  ]
}`
	pods := corev1.PodList{}
	err := json.Unmarshal([]byte(podJSON), &pods)
	if err != nil {
		panic(fmt.Sprintf("unmarshal pod err %v", err))
	}

	return &pods.Items[0]
}

func getPodStore() *PodStore {
	nodeInfo := newNodeInfo("testNode1", &mockNodeInfoProvider{}, zap.NewNop())
	nodeInfo.setCPUCapacity(4000)
	nodeInfo.setMemCapacity(400 * 1024 * 1024)
	return &PodStore{
		cache:            newMapWithExpiry(time.Minute),
		nodeInfo:         nodeInfo,
		prevMeasurements: sync.Map{},
		logger:           zap.NewNop(),
	}
}

func generateMetric(fields map[string]any, tags map[string]string) CIMetric {
	tagsCopy := maps.Clone(tags)
	fieldsCopy := maps.Clone(fields)

	return &mockCIMetric{
		tags:   tagsCopy,
		fields: fieldsCopy,
	}
}

func TestPodStore_decorateCpu(t *testing.T) {
	podStore := getPodStore()
	defer require.NoError(t, podStore.Shutdown())

	pod := getBaseTestPodInfo()

	// test pod metrics
	tags := map[string]string{ci.MetricType: ci.TypePod}
	fields := map[string]any{ci.MetricName(ci.TypePod, ci.CPUTotal): float64(1)}

	metric := generateMetric(fields, tags)
	podStore.decorateCPU(metric, pod)

	assert.Equal(t, uint64(10), metric.GetField("pod_cpu_request").(uint64))
	assert.Equal(t, uint64(10), metric.GetField("pod_cpu_limit").(uint64))
	assert.Equal(t, float64(0.25), metric.GetField("pod_cpu_reserved_capacity").(float64))
	assert.Equal(t, float64(10), metric.GetField("pod_cpu_utilization_over_pod_limit").(float64))
	assert.Equal(t, float64(1), metric.GetField("pod_cpu_usage_total").(float64))

	// test container metrics
	tags = map[string]string{ci.MetricType: ci.TypeContainer, ci.ContainerNamekey: "ubuntu"}
	fields = map[string]any{ci.MetricName(ci.TypeContainer, ci.CPUTotal): float64(1)}
	metric = generateMetric(fields, tags)
	podStore.decorateCPU(metric, pod)

	assert.Equal(t, uint64(10), metric.GetField("container_cpu_request").(uint64))
	assert.Equal(t, uint64(10), metric.GetField("container_cpu_limit").(uint64))
	assert.Equal(t, float64(1), metric.GetField("container_cpu_usage_total").(float64))
	assert.False(t, metric.HasField("container_cpu_utilization_over_container_limit"))

	podStore.includeEnhancedMetrics = true
	podStore.decorateCPU(metric, pod)

	assert.Equal(t, float64(10), metric.GetField("container_cpu_utilization_over_container_limit").(float64))
}

func TestPodStore_decorateMem(t *testing.T) {
	podStore := getPodStore()
	defer require.NoError(t, podStore.Shutdown())
	pod := getBaseTestPodInfo()

	tags := map[string]string{ci.MetricType: ci.TypePod}
	fields := map[string]any{ci.MetricName(ci.TypePod, ci.MemWorkingset): uint64(10 * 1024 * 1024)}

	metric := generateMetric(fields, tags)
	podStore.decorateMem(metric, pod)

	assert.Equal(t, uint64(52428800), metric.GetField("pod_memory_request").(uint64))
	assert.Equal(t, uint64(52428800), metric.GetField("pod_memory_limit").(uint64))
	assert.Equal(t, float64(12.5), metric.GetField("pod_memory_reserved_capacity").(float64))
	assert.Equal(t, float64(20), metric.GetField("pod_memory_utilization_over_pod_limit").(float64))
	assert.Equal(t, uint64(10*1024*1024), metric.GetField("pod_memory_working_set").(uint64))

	tags = map[string]string{ci.MetricType: ci.TypeContainer, ci.ContainerNamekey: "ubuntu"}
	fields = map[string]any{ci.MetricName(ci.TypeContainer, ci.MemWorkingset): uint64(10 * 1024 * 1024)}

	metric = generateMetric(fields, tags)
	podStore.decorateMem(metric, pod)

	assert.Equal(t, uint64(52428800), metric.GetField("container_memory_request").(uint64))
	assert.Equal(t, uint64(52428800), metric.GetField("container_memory_limit").(uint64))
	assert.Equal(t, uint64(10*1024*1024), metric.GetField("container_memory_working_set").(uint64))
	assert.False(t, metric.HasField("container_memory_utilization_over_container_limit"))

	podStore.includeEnhancedMetrics = true
	podStore.decorateMem(metric, pod)

	assert.Equal(t, float64(20), metric.GetField("container_memory_utilization_over_container_limit").(float64))
}

func TestPodStore_decorateGpu(t *testing.T) {
	podStore := getPodStore()
	defer require.NoError(t, podStore.Shutdown())

	pod := getBaseTestPodInfo()

	// test pod metrics
	tags := map[string]string{ci.MetricType: ci.TypePod}
	fields := map[string]any{}

	metric := generateMetric(fields, tags)
	podStore.includeEnhancedMetrics = true
	podStore.enableAcceleratedComputeMetrics = true
	podStore.decorateGPU(metric, pod)

	assert.Equal(t, uint64(1), metric.GetField("pod_gpu_request").(uint64))
	assert.Equal(t, uint64(1), metric.GetField("pod_gpu_limit").(uint64))
	assert.Equal(t, uint64(1), metric.GetField("pod_gpu_usage_total").(uint64))
	assert.Equal(t, float64(5), metric.GetField("pod_gpu_reserved_capacity").(float64))
}

func getPodStoreWithNeuronCapacity() *PodStore {
	nodeInfo := newNodeInfo("testNode1", &mockNodeInfoProvider{}, zap.NewNop())
	nodeInfo.setCPUCapacity(4000)
	nodeInfo.setMemCapacity(400 * 1024 * 1024)
	return &PodStore{
		cache:            newMapWithExpiry(time.Minute),
		nodeInfo:         nodeInfo,
		prevMeasurements: sync.Map{},
		logger:           zap.NewNop(),
	}
}

func TestPodStore_decorateNeuron(t *testing.T) {
	tests := []struct {
		name          string
		resourceKey   string
		requestValue  string
		limitValue    string
		expectedReq   uint64
		expectedLimit uint64
		expectedUsage uint64
	}{
		{
			name:          "neuron devices",
			resourceKey:   "aws.amazon.com/neuron",
			requestValue:  "1",
			limitValue:    "2",
			expectedReq:   2,
			expectedLimit: 4,
			expectedUsage: 4,
		},
		{
			name:          "neuron cores direct",
			resourceKey:   "aws.amazon.com/neuroncore",
			requestValue:  "1",
			limitValue:    "2",
			expectedReq:   1,
			expectedLimit: 2,
			expectedUsage: 2,
		},
		{
			name:          "neuron device resource",
			resourceKey:   "aws.amazon.com/neurondevice",
			requestValue:  "1",
			limitValue:    "1",
			expectedReq:   2,
			expectedLimit: 2,
			expectedUsage: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			podStore := getPodStoreWithNeuronCapacity()
			defer require.NoError(t, podStore.Shutdown())

			pod := getBaseTestPodInfo()
			pod.Spec.Containers[0].Resources.Requests[corev1.ResourceName(tt.resourceKey)] = resource.MustParse(tt.requestValue)
			pod.Spec.Containers[0].Resources.Limits[corev1.ResourceName(tt.resourceKey)] = resource.MustParse(tt.limitValue)

			tags := map[string]string{ci.MetricType: ci.TypePod}
			metric := generateMetric(map[string]any{}, tags)

			podStore.includeEnhancedMetrics = true
			podStore.enableAcceleratedComputeMetrics = true
			podStore.decorateNeuron(metric, pod)

			assert.Equal(t, tt.expectedReq, metric.GetField("pod_neuroncore_request").(uint64))
			assert.Equal(t, tt.expectedLimit, metric.GetField("pod_neuroncore_limit").(uint64))
			assert.Equal(t, tt.expectedUsage, metric.GetField("pod_neuroncore_usage_total").(uint64))
		})
	}
}

func TestPodStore_decorateNode_withMultipleNeuronPods(t *testing.T) {
	t.Setenv(ci.HostName, "testNode1")
	podStore := getPodStoreWithNeuronCapacity()
	defer require.NoError(t, podStore.Shutdown())

	pod1 := getBaseTestPodInfo()
	pod1.Name = "neuron-pod-1"
	pod1.Spec.Containers[0].Resources.Requests["aws.amazon.com/neuron"] = resource.MustParse("1")
	pod1.Spec.Containers[0].Resources.Limits["aws.amazon.com/neuron"] = resource.MustParse("2")

	pod2 := getBaseTestPodInfo()
	pod2.Name = "neuron-pod-2"
	pod2.Spec.Containers[0].Resources.Requests["aws.amazon.com/neuron"] = resource.MustParse("2")
	pod2.Spec.Containers[0].Resources.Limits["aws.amazon.com/neuron"] = resource.MustParse("4")

	podList := []corev1.Pod{*pod1, *pod2}
	podStore.refreshInternal(time.Now(), podList)

	tags := map[string]string{ci.MetricType: ci.TypeNode}
	fields := map[string]any{
		ci.MetricName(ci.TypeNode, ci.CPUTotal):      float64(1000),
		ci.MetricName(ci.TypeNode, ci.MemWorkingset): float64(2048),
	}

	metric := generateMetric(fields, tags)
	podStore.includeEnhancedMetrics = true
	podStore.enableAcceleratedComputeMetrics = true

	podStore.decorateNode(metric)

	assert.Equal(t, uint64(6), metric.GetField("node_neuroncore_request").(uint64))
	assert.Equal(t, uint64(32), metric.GetField("node_neuroncore_limit").(uint64))
	assert.Equal(t, uint64(12), metric.GetField("node_neuroncore_usage_total").(uint64))
	assert.Equal(t, float64(18.75), metric.GetField("node_neuroncore_reserved_capacity").(float64))
	assert.Equal(t, float64(81.25), metric.GetField("node_neuroncore_unreserved_capacity").(float64))
	assert.Equal(t, uint64(26), metric.GetField("node_neuroncore_available_capacity").(uint64))
}

func TestPodStore_previousCleanupLocking(_ *testing.T) {
	podStore := getPodStore()
	podStore.podClient = &mockPodClient{}
	pod := getBaseTestPodInfo()
	ctx := context.TODO()

	tags := map[string]string{ci.MetricType: ci.TypePod, ci.K8sNamespace: "default", ci.K8sPodNameKey: "cpu-limit"}
	fields := map[string]any{ci.MetricName(ci.TypePod, ci.CPUTotal): float64(1)}
	metric := generateMetric(fields, tags)

	quit := make(chan bool)
	go func() {
		for {
			select {
			case <-quit:
				return
			default:
				// manipulate last refreshed so that we are always forcing a refresh
				podStore.lastRefreshed = time.Now().Add(-1 * time.Hour)
				podStore.RefreshTick(ctx)
			}
		}
	}()

	for i := 0; i < 1000; i++ {
		// status metrics push things to the previous list
		podStore.addStatus(metric, pod)
	}

	quit <- true
	// this test would crash without proper locking
}

func TestPodStore_addContainerCount(t *testing.T) {
	pod := getBaseTestPodInfo()

	tags := map[string]string{ci.MetricType: ci.TypePod}
	fields := map[string]any{ci.MetricName(ci.TypePod, ci.CPUTotal): float64(1)}

	metric := generateMetric(fields, tags)

	addContainerCount(metric, pod)
	assert.Equal(t, int(1), metric.GetField(ci.MetricName(ci.TypePod, ci.RunningContainerCount)).(int))
	assert.Equal(t, int(1), metric.GetField(ci.MetricName(ci.TypePod, ci.ContainerCount)).(int))

	metric = generateMetric(fields, tags)

	pod.Status.ContainerStatuses[0].State.Running = nil
	addContainerCount(metric, pod)
	assert.Equal(t, int(0), metric.GetField(ci.MetricName(ci.TypePod, ci.RunningContainerCount)).(int))
	assert.Equal(t, int(1), metric.GetField(ci.MetricName(ci.TypePod, ci.ContainerCount)).(int))
}

const (
	PodFailedMetricName    = "pod_status_failed"
	PodPendingMetricName   = "pod_status_pending"
	PodRunningMetricName   = "pod_status_running"
	PodSucceededMetricName = "pod_status_succeeded"
	PodUnknownMetricName   = "pod_status_unknown"
	PodReadyMetricName     = "pod_status_ready"
	PodScheduledMetricName = "pod_status_scheduled"
)

func TestPodStore_enhanced_metrics_disabled(t *testing.T) {
	decoratedResultMetric := runAddStatusToGetDecoratedCIMetric("./test_resources/pod_in_phase_failed.json", false)

	assert.False(t, decoratedResultMetric.HasField(PodFailedMetricName))
	assert.False(t, decoratedResultMetric.HasField(PodPendingMetricName))
	assert.False(t, decoratedResultMetric.HasField(PodRunningMetricName))
	assert.False(t, decoratedResultMetric.HasField(PodSucceededMetricName))
}

func TestPodStore_addStatus_adds_pod_failed_metric(t *testing.T) {
	decoratedResultMetric := runAddStatusToGetDecoratedCIMetric("./test_resources/pod_in_phase_failed.json", true)

	assert.Equal(t, 1, decoratedResultMetric.GetField(PodFailedMetricName))
	assert.Equal(t, 0, decoratedResultMetric.GetField(PodPendingMetricName))
	assert.Equal(t, 0, decoratedResultMetric.GetField(PodRunningMetricName))
	assert.Equal(t, 0, decoratedResultMetric.GetField(PodSucceededMetricName))
}

func TestPodStore_addStatus_adds_pod_pending_metric(t *testing.T) {
	decoratedResultMetric := runAddStatusToGetDecoratedCIMetric("./test_resources/pod_in_phase_pending.json", true)

	assert.Equal(t, 0, decoratedResultMetric.GetField(PodFailedMetricName))
	assert.Equal(t, 1, decoratedResultMetric.GetField(PodPendingMetricName))
	assert.Equal(t, 0, decoratedResultMetric.GetField(PodRunningMetricName))
	assert.Equal(t, 0, decoratedResultMetric.GetField(PodSucceededMetricName))
}

func TestPodStore_addStatus_adds_pod_running_metric(t *testing.T) {
	decoratedResultMetric := runAddStatusToGetDecoratedCIMetric("./test_resources/pod_in_phase_running.json", true)

	assert.Equal(t, 0, decoratedResultMetric.GetField(PodFailedMetricName))
	assert.Equal(t, 0, decoratedResultMetric.GetField(PodPendingMetricName))
	assert.Equal(t, 1, decoratedResultMetric.GetField(PodRunningMetricName))
	assert.Equal(t, 0, decoratedResultMetric.GetField(PodSucceededMetricName))
}

func TestPodStore_addStatus_adds_pod_succeeded_metric(t *testing.T) {
	decoratedResultMetric := runAddStatusToGetDecoratedCIMetric("./test_resources/pod_in_phase_succeeded.json", true)

	assert.Equal(t, 0, decoratedResultMetric.GetField(PodFailedMetricName))
	assert.Equal(t, 0, decoratedResultMetric.GetField(PodPendingMetricName))
	assert.Equal(t, 0, decoratedResultMetric.GetField(PodRunningMetricName))
	assert.Equal(t, 1, decoratedResultMetric.GetField(PodSucceededMetricName))
}

func TestPodStore_addStatus_enhanced_metrics_disabled(t *testing.T) {
	decoratedResultMetric := runAddStatusToGetDecoratedCIMetric("./test_resources/all_pod_conditions_valid.json", false)

	assert.False(t, decoratedResultMetric.HasField(PodReadyMetricName))
	assert.False(t, decoratedResultMetric.HasField(PodScheduledMetricName))
	assert.False(t, decoratedResultMetric.HasField(PodUnknownMetricName))
}

func TestPodStore_addStatus_adds_all_pod_conditions_as_metrics_when_true_false_unknown(t *testing.T) {
	decoratedResultMetric := runAddStatusToGetDecoratedCIMetric("./test_resources/all_pod_conditions_valid.json", true)

	assert.Equal(t, 0, decoratedResultMetric.GetField(PodReadyMetricName))
	assert.Equal(t, 0, decoratedResultMetric.GetField(PodScheduledMetricName))
	assert.Equal(t, 1, decoratedResultMetric.GetField(PodUnknownMetricName))
}

func TestPodStore_addStatus_adds_all_pod_conditions_as_metrics_when_Ready_Scheduled_Condition_Unknown(t *testing.T) {
	decoratedResultMetric := runAddStatusToGetDecoratedCIMetric("./test_resources/pod_Ready_Scheduled_Condition_Unknown.json", true)

	assert.Equal(t, 0, decoratedResultMetric.GetField(PodReadyMetricName))
	assert.Equal(t, 0, decoratedResultMetric.GetField(PodScheduledMetricName))
	assert.Equal(t, 1, decoratedResultMetric.GetField(PodUnknownMetricName))
}

func TestPodStore_addStatus_adds_all_pod_conditions_as_metrics_when_unexpected(t *testing.T) {
	decoratedResultMetric := runAddStatusToGetDecoratedCIMetric("./test_resources/one_pod_condition_invalid.json", true)

	assert.Equal(t, 1, decoratedResultMetric.GetField(PodReadyMetricName))
	assert.Equal(t, 1, decoratedResultMetric.GetField(PodScheduledMetricName))
	assert.Equal(t, 0, decoratedResultMetric.GetField(PodUnknownMetricName))
}

func TestPodStore_addStatus_enhanced_metrics(t *testing.T) {
	pod := getBaseTestPodInfo()
	// add another container
	containerCopy := pod.Status.ContainerStatuses[0]
	containerCopy.Name = "ubuntu2"
	pod.Status.ContainerStatuses = append(pod.Status.ContainerStatuses, containerCopy)

	tags := map[string]string{ci.MetricType: ci.TypePod, ci.K8sNamespace: "default", ci.K8sPodNameKey: "cpu-limit"}
	fields := map[string]any{ci.MetricName(ci.TypePod, ci.CPUTotal): float64(1)}
	podStore := getPodStore()
	podStore.includeEnhancedMetrics = true
	defer require.NoError(t, podStore.Shutdown())
	metric := generateMetric(fields, tags)

	podStore.addStatus(metric, pod)
	assert.Equal(t, "Running", metric.GetTag(ci.PodStatus))
	val := metric.GetField(ci.MetricName(ci.TypePod, ci.ContainerRestartCount))
	assert.Nil(t, val)

	// set up container defaults
	tags = map[string]string{ci.MetricType: ci.TypeContainer, ci.K8sNamespace: "default", ci.K8sPodNameKey: "cpu-limit", ci.ContainerNamekey: "ubuntu"}
	metric = generateMetric(fields, tags)
	podStore.addStatus(metric, pod)
	assert.Equal(t, "Running", metric.GetTag(ci.ContainerStatus))
	val = metric.GetField(ci.ContainerRestartCount)
	assert.Nil(t, val)
	// set up the other container
	tags = map[string]string{ci.MetricType: ci.TypeContainer, ci.K8sNamespace: "default", ci.K8sPodNameKey: "cpu-limit", ci.ContainerNamekey: "ubuntu2"}
	metric = generateMetric(fields, tags)
	podStore.addStatus(metric, pod)
	assert.Equal(t, "Running", metric.GetTag(ci.ContainerStatus))
	val = metric.GetField(ci.ContainerRestartCount)
	assert.Nil(t, val)

	tags = map[string]string{ci.MetricType: ci.TypePod, ci.K8sNamespace: "default", ci.K8sPodNameKey: "cpu-limit"}
	metric = generateMetric(fields, tags)

	podStore.addStatus(metric, pod)
	assert.Equal(t, "Running", metric.GetTag(ci.PodStatus))
	val = metric.GetField(ci.ContainerRestartCount)
	assert.Nil(t, val)
	val = metric.GetField(ci.MetricName(ci.TypePod, ci.StatusContainerRunning))
	assert.NotNil(t, val)
	assert.Equal(t, 2, val)

	pod.Status.ContainerStatuses[0].State.Running = nil
	pod.Status.ContainerStatuses[0].State.Terminated = &corev1.ContainerStateTerminated{}
	pod.Status.ContainerStatuses[0].LastTerminationState.Terminated = &corev1.ContainerStateTerminated{Reason: "OOMKilled"}
	pod.Status.ContainerStatuses[0].RestartCount = 1
	pod.Status.ContainerStatuses[1].State.Running = nil
	pod.Status.ContainerStatuses[1].State.Terminated = &corev1.ContainerStateTerminated{}
	pod.Status.ContainerStatuses[1].LastTerminationState.Terminated = &corev1.ContainerStateTerminated{Reason: "OOMKilled"}
	pod.Status.ContainerStatuses[1].RestartCount = 1
	pod.Status.Phase = "Succeeded"

	tags = map[string]string{ci.MetricType: ci.TypePod, ci.K8sNamespace: "default", ci.K8sPodNameKey: "cpu-limit"}
	metric = generateMetric(fields, tags)

	podStore.addStatus(metric, pod)
	assert.Equal(t, "Succeeded", metric.GetTag(ci.PodStatus))
	assert.Equal(t, 2, metric.GetField(ci.MetricName(ci.TypePod, ci.ContainerRestartCount)))

	// update the container metrics
	// set up container defaults
	tags = map[string]string{ci.MetricType: ci.TypeContainer, ci.K8sNamespace: "default", ci.K8sPodNameKey: "cpu-limit", ci.ContainerNamekey: "ubuntu"}
	metric = generateMetric(fields, tags)
	podStore.addStatus(metric, pod)
	assert.Equal(t, 1, metric.GetField(ci.ContainerRestartCount))

	// test the other container
	tags = map[string]string{ci.MetricType: ci.TypeContainer, ci.K8sNamespace: "default", ci.K8sPodNameKey: "cpu-limit", ci.ContainerNamekey: "ubuntu2"}
	metric = generateMetric(fields, tags)
	podStore.addStatus(metric, pod)
	assert.Equal(t, 1, metric.GetField(ci.ContainerRestartCount))

	tags = map[string]string{ci.MetricType: ci.TypePod, ci.K8sNamespace: "default", ci.K8sPodNameKey: "cpu-limit"}
	metric = generateMetric(fields, tags)

	podStore.addStatus(metric, pod)
	assert.Equal(t, 2, metric.GetField(ci.MetricName(ci.TypePod, ci.StatusContainerTerminated)))
	assert.Equal(t, 2, metric.GetField(ci.MetricName(ci.TypePod, ci.StatusContainerTerminatedReasonOOMKilled)))

	pod.Status.ContainerStatuses[0].LastTerminationState.Terminated = nil
	pod.Status.ContainerStatuses[0].State.Waiting = &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}
	pod.Status.ContainerStatuses[1].LastTerminationState.Terminated = nil
	pod.Status.ContainerStatuses[1].State.Waiting = &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}

	tags = map[string]string{ci.MetricType: ci.TypePod, ci.K8sNamespace: "default", ci.K8sPodNameKey: "cpu-limit"}
	metric = generateMetric(fields, tags)

	podStore.addStatus(metric, pod)
	assert.Equal(t, 2, metric.GetField(ci.MetricName(ci.TypePod, ci.StatusContainerWaiting)))
	assert.Equal(t, 2, metric.GetField(ci.MetricName(ci.TypePod, ci.StatusContainerWaitingReasonCrashLoopBackOff)))
	// sparse metrics
	assert.Nil(t, metric.GetField(ci.MetricName(ci.TypePod, ci.StatusContainerWaitingReasonImagePullError)))
	assert.Nil(t, metric.GetField(ci.MetricName(ci.TypePod, ci.StatusContainerTerminatedReasonOOMKilled)))
	assert.Nil(t, metric.GetField(ci.MetricName(ci.TypePod, ci.StatusContainerWaitingReasonStartError)))
	assert.Nil(t, metric.GetField(ci.MetricName(ci.TypePod, ci.StatusContainerWaitingReasonCreateContainerError)))
	assert.Nil(t, metric.GetField(ci.MetricName(ci.TypePod, ci.StatusContainerWaitingReasonCreateContainerConfigError)))

	pod.Status.ContainerStatuses[0].State.Waiting = &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff"}
	pod.Status.ContainerStatuses[1].State.Waiting = &corev1.ContainerStateWaiting{Reason: "StartError"}

	tags = map[string]string{ci.MetricType: ci.TypePod, ci.K8sNamespace: "default", ci.K8sPodNameKey: "cpu-limit"}
	metric = generateMetric(fields, tags)

	podStore.addStatus(metric, pod)
	assert.Equal(t, "Succeeded", metric.GetTag(ci.PodStatus))
	assert.Equal(t, 2, metric.GetField(ci.MetricName(ci.TypePod, ci.StatusContainerWaiting)))
	assert.Equal(t, 1, metric.GetField(ci.MetricName(ci.TypePod, ci.StatusContainerWaitingReasonImagePullError)))
	assert.Equal(t, 1, metric.GetField(ci.MetricName(ci.TypePod, ci.StatusContainerWaitingReasonStartError)))

	pod.Status.ContainerStatuses[0].State.Waiting = &corev1.ContainerStateWaiting{Reason: "ErrImagePull"}
	metric = generateMetric(fields, tags)
	podStore.addStatus(metric, pod)
	assert.Equal(t, 1, metric.GetField(ci.MetricName(ci.TypePod, ci.StatusContainerWaitingReasonImagePullError)))

	pod.Status.ContainerStatuses[0].State.Waiting = &corev1.ContainerStateWaiting{Reason: "InvalidImageName"}
	metric = generateMetric(fields, tags)
	podStore.addStatus(metric, pod)
	assert.Equal(t, 1, metric.GetField(ci.MetricName(ci.TypePod, ci.StatusContainerWaitingReasonImagePullError)))

	pod.Status.ContainerStatuses[0].State.Waiting = &corev1.ContainerStateWaiting{Reason: "CreateContainerError"}
	metric = generateMetric(fields, tags)
	podStore.addStatus(metric, pod)
	assert.Equal(t, 1, metric.GetField(ci.MetricName(ci.TypePod, ci.StatusContainerWaitingReasonCreateContainerError)))

	pod.Status.ContainerStatuses[0].State.Waiting = &corev1.ContainerStateWaiting{Reason: "CreateContainerConfigError"}
	metric = generateMetric(fields, tags)
	podStore.addStatus(metric, pod)
	assert.Equal(t, 1, metric.GetField(ci.MetricName(ci.TypePod, ci.StatusContainerWaitingReasonCreateContainerConfigError)))

	pod.Status.ContainerStatuses[0].State.Waiting = &corev1.ContainerStateWaiting{Reason: "StartError"}
	pod.Status.ContainerStatuses[1].State.Waiting = nil
	metric = generateMetric(fields, tags)
	podStore.addStatus(metric, pod)
	assert.Equal(t, 1, metric.GetField(ci.MetricName(ci.TypePod, ci.StatusContainerWaitingReasonStartError)))

	// test delta of restartCount
	pod.Status.ContainerStatuses[0].RestartCount = 3
	tags = map[string]string{ci.MetricType: ci.TypePod, ci.K8sNamespace: "default", ci.K8sPodNameKey: "cpu-limit"}
	metric = generateMetric(fields, tags)

	podStore.addStatus(metric, pod)
	assert.Equal(t, 2, metric.GetField(ci.MetricName(ci.TypePod, ci.ContainerRestartCount)))

	tags = map[string]string{ci.MetricType: ci.TypeContainer, ci.K8sNamespace: "default", ci.K8sPodNameKey: "cpu-limit", ci.ContainerNamekey: "ubuntu"}
	metric = generateMetric(fields, tags)

	podStore.addStatus(metric, pod)
	assert.Equal(t, 2, metric.GetField(ci.ContainerRestartCount))
}

func TestPodStore_addStatus_without_enhanced_metrics(t *testing.T) {
	pod := getBaseTestPodInfo()
	tags := map[string]string{ci.MetricType: ci.TypePod, ci.K8sNamespace: "default", ci.K8sPodNameKey: "cpu-limit"}
	fields := map[string]any{ci.MetricName(ci.TypePod, ci.CPUTotal): float64(1)}
	podStore := getPodStore()
	podStore.includeEnhancedMetrics = false
	metric := generateMetric(fields, tags)

	podStore.addStatus(metric, pod)
	assert.Equal(t, "Running", metric.GetTag(ci.PodStatus))
	val := metric.GetField(ci.MetricName(ci.TypePod, ci.ContainerRestartCount))
	assert.Nil(t, val)

	tags = map[string]string{ci.MetricType: ci.TypeContainer, ci.K8sNamespace: "default", ci.K8sPodNameKey: "cpu-limit", ci.ContainerNamekey: "ubuntu"}
	metric = generateMetric(fields, tags)

	podStore.addStatus(metric, pod)
	assert.Equal(t, "Running", metric.GetTag(ci.ContainerStatus))
	val = metric.GetField(ci.ContainerRestartCount)
	assert.Nil(t, val)
	assert.False(t, metric.HasField(ci.MetricName(ci.TypeContainer, ci.StatusContainerRunning)))

	pod.Status.ContainerStatuses[0].State.Running = nil
	pod.Status.ContainerStatuses[0].State.Terminated = &corev1.ContainerStateTerminated{}
	pod.Status.ContainerStatuses[0].LastTerminationState.Terminated = &corev1.ContainerStateTerminated{Reason: "OOMKilled"}
	pod.Status.ContainerStatuses[0].RestartCount = 1
	pod.Status.Phase = "Succeeded"

	tags = map[string]string{ci.MetricType: ci.TypePod, ci.K8sNamespace: "default", ci.K8sPodNameKey: "cpu-limit"}
	metric = generateMetric(fields, tags)

	podStore.addStatus(metric, pod)
	assert.Equal(t, "Succeeded", metric.GetTag(ci.PodStatus))
	assert.Equal(t, int(1), metric.GetField(ci.MetricName(ci.TypePod, ci.ContainerRestartCount)).(int))

	tags = map[string]string{ci.MetricType: ci.TypeContainer, ci.K8sNamespace: "default", ci.K8sPodNameKey: "cpu-limit", ci.ContainerNamekey: "ubuntu"}
	metric = generateMetric(fields, tags)

	podStore.addStatus(metric, pod)
	assert.Equal(t, "Terminated", metric.GetTag(ci.ContainerStatus))
	assert.Equal(t, "OOMKilled", metric.GetTag(ci.ContainerLastTerminationReason))
	assert.Equal(t, int(1), metric.GetField(ci.ContainerRestartCount).(int))
	assert.False(t, metric.HasField(ci.MetricName(ci.TypeContainer, ci.StatusContainerTerminated)))

	pod.Status.ContainerStatuses[0].State.Terminated = nil
	pod.Status.ContainerStatuses[0].State.Waiting = &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}

	tags = map[string]string{ci.MetricType: ci.TypeContainer, ci.K8sNamespace: "default", ci.K8sPodNameKey: "cpu-limit", ci.ContainerNamekey: "ubuntu"}
	metric = generateMetric(fields, tags)

	podStore.addStatus(metric, pod)
	assert.Equal(t, "Waiting", metric.GetTag(ci.ContainerStatus))
	assert.False(t, metric.HasField(ci.MetricName(ci.TypeContainer, ci.StatusContainerWaiting)))
	assert.False(t, metric.HasField(ci.MetricName(ci.TypeContainer, ci.StatusContainerWaitingReasonCrashLoopBackOff)))

	pod.Status.ContainerStatuses[0].State.Waiting = &corev1.ContainerStateWaiting{Reason: "SomeOtherReason"}

	tags = map[string]string{ci.MetricType: ci.TypeContainer, ci.K8sNamespace: "default", ci.K8sPodNameKey: "cpu-limit", ci.ContainerNamekey: "ubuntu"}
	metric = generateMetric(fields, tags)

	podStore.addStatus(metric, pod)
	assert.Equal(t, "Waiting", metric.GetTag(ci.ContainerStatus))
	assert.False(t, metric.HasField(ci.MetricName(ci.TypeContainer, ci.StatusContainerWaiting)))
	assert.False(t, metric.HasField(ci.MetricName(ci.TypeContainer, ci.StatusContainerWaitingReasonCrashLoopBackOff)))

	// test delta of restartCount
	pod.Status.ContainerStatuses[0].RestartCount = 3
	tags = map[string]string{ci.MetricType: ci.TypePod, ci.K8sNamespace: "default", ci.K8sPodNameKey: "cpu-limit"}
	metric = generateMetric(fields, tags)

	podStore.addStatus(metric, pod)
	assert.Equal(t, int(2), metric.GetField(ci.MetricName(ci.TypePod, ci.ContainerRestartCount)).(int))

	tags = map[string]string{ci.MetricType: ci.TypeContainer, ci.K8sNamespace: "default", ci.K8sPodNameKey: "cpu-limit", ci.ContainerNamekey: "ubuntu"}
	metric = generateMetric(fields, tags)

	podStore.addStatus(metric, pod)
	assert.Equal(t, int(2), metric.GetField(ci.ContainerRestartCount).(int))
}

func TestPodStore_addContainerID(t *testing.T) {
	pod := getBaseTestPodInfo()
	tags := map[string]string{ci.ContainerNamekey: "ubuntu", ci.ContainerIDkey: "123"}
	fields := map[string]any{ci.MetricName(ci.TypePod, ci.CPUTotal): float64(1)}
	kubernetesBlob := map[string]any{}
	metric := generateMetric(fields, tags)
	addContainerID(pod, metric, kubernetesBlob, zap.NewNop())

	expected := map[string]any{}
	expected["docker"] = map[string]string{"container_id": "637631e2634ea92c0c1aa5d24734cfe794f09c57933026592c12acafbaf6972c"}
	assert.Equal(t, expected, kubernetesBlob)
	assert.Equal(t, "ubuntu", metric.GetTag(ci.ContainerNamekey))

	tags = map[string]string{ci.ContainerNamekey: "notUbuntu", ci.ContainerIDkey: "123"}
	kubernetesBlob = map[string]any{}
	metric = generateMetric(fields, tags)
	addContainerID(pod, metric, kubernetesBlob, zap.NewNop())

	expected = map[string]any{}
	expected["container_id"] = "123"
	assert.Equal(t, expected, kubernetesBlob)
	assert.Equal(t, "notUbuntu", metric.GetTag(ci.ContainerNamekey))
}

func TestPodStore_addLabel(t *testing.T) {
	pod := getBaseTestPodInfo()
	kubernetesBlob := map[string]any{}
	addLabels(pod, kubernetesBlob)
	expected := map[string]any{}
	expected["labels"] = map[string]string{"app": "hello_test"}
	assert.Equal(t, expected, kubernetesBlob)
}

func TestGetJobNamePrefix(t *testing.T) {
	assert.Equal(t, "abcd", getJobNamePrefix("abcd-efg"))
	assert.Equal(t, "abcd", getJobNamePrefix("abcd.efg"))
	assert.Equal(t, "abcd", getJobNamePrefix("abcd-e.fg"))
	assert.Equal(t, "abc", getJobNamePrefix("abc.d-efg"))
	assert.Equal(t, "abcd", getJobNamePrefix("abcd-.efg"))
	assert.Equal(t, "abcd", getJobNamePrefix("abcd.-efg"))
	assert.Equal(t, "abcdefg", getJobNamePrefix("abcdefg"))
	assert.Equal(t, "abcdefg", getJobNamePrefix("abcdefg-"))
	assert.Empty(t, getJobNamePrefix(".abcd-efg"))
	assert.Empty(t, getJobNamePrefix(""))
}

type mockReplicaSetInfo1 struct{}

func (m *mockReplicaSetInfo1) ReplicaSetInfos() []*k8sclient.ReplicaSetInfo {
	return []*k8sclient.ReplicaSetInfo{}
}

func (m *mockReplicaSetInfo1) ReplicaSetToDeployment() map[string]string {
	return map[string]string{}
}

type mockK8sClient1 struct{}

func (m *mockK8sClient1) GetReplicaSetClient() k8sclient.ReplicaSetClient {
	return &mockReplicaSetInfo1{}
}

type mockReplicaSetInfo2 struct{}

func (m *mockReplicaSetInfo2) ReplicaSetInfos() []*k8sclient.ReplicaSetInfo {
	return []*k8sclient.ReplicaSetInfo{}
}

func (m *mockReplicaSetInfo2) ReplicaSetToDeployment() map[string]string {
	return map[string]string{"DeploymentTest-sftrz2785": "DeploymentTest"}
}

type mockK8sClient2 struct{}

func (m *mockK8sClient2) GetReplicaSetClient() k8sclient.ReplicaSetClient {
	return &mockReplicaSetInfo2{}
}

func TestPodStore_addPodOwnersAndPodNameFallback(t *testing.T) {
	podStore := &PodStore{k8sClient: &mockK8sClient1{}}
	pod := getBaseTestPodInfo()
	tags := map[string]string{ci.MetricType: ci.TypePod, ci.ContainerNamekey: "ubuntu"}
	fields := map[string]any{ci.MetricName(ci.TypePod, ci.CPUTotal): float64(1)}
	metric := generateMetric(fields, tags)

	// Test ReplicaSet
	rsName := "ReplicaSetTest"
	suffix := "-42kcz"
	pod.OwnerReferences[0].Kind = ci.ReplicaSet
	pod.OwnerReferences[0].Name = rsName + suffix
	kubernetesBlob := map[string]any{}
	podStore.addPodOwnersAndPodName(metric, pod, kubernetesBlob)
	expectedOwner := map[string]any{}
	expectedOwner["pod_owners"] = []any{map[string]string{"owner_kind": ci.Deployment, "owner_name": rsName}}
	expectedOwnerName := rsName
	assert.Equal(t, expectedOwnerName, metric.GetTag(ci.PodNameKey))
	assert.Equal(t, expectedOwner, kubernetesBlob)

	// Test Job
	metric = generateMetric(fields, tags)
	jobName := "Job"
	suffix = "-0123456789"
	pod.OwnerReferences[0].Kind = ci.Job
	pod.OwnerReferences[0].Name = jobName + suffix
	kubernetesBlob = map[string]any{}
	podStore.addPodOwnersAndPodName(metric, pod, kubernetesBlob)
	expectedOwner["pod_owners"] = []any{map[string]string{"owner_kind": ci.CronJob, "owner_name": jobName}}
	expectedOwnerName = jobName
	assert.Equal(t, expectedOwnerName, metric.GetTag(ci.PodNameKey))
	assert.Equal(t, expectedOwner, kubernetesBlob)
}

func TestPodStore_addPodOwnersAndPodName(t *testing.T) {
	podStore := &PodStore{k8sClient: &mockK8sClient2{}}

	pod := getBaseTestPodInfo()
	tags := map[string]string{ci.MetricType: ci.TypePod, ci.ContainerNamekey: "ubuntu"}
	fields := map[string]any{ci.MetricName(ci.TypePod, ci.CPUTotal): float64(1)}

	// Test DaemonSet
	metric := generateMetric(fields, tags)
	kubernetesBlob := map[string]any{}
	podStore.addPodOwnersAndPodName(metric, pod, kubernetesBlob)

	expectedOwner := map[string]any{}
	expectedOwner["pod_owners"] = []any{map[string]string{"owner_kind": ci.DaemonSet, "owner_name": "DaemonSetTest"}}
	expectedOwnerName := "DaemonSetTest"
	assert.Equal(t, expectedOwnerName, metric.GetTag(ci.PodNameKey))
	assert.Equal(t, expectedOwner, kubernetesBlob)

	// Test ReplicaSet
	metric = generateMetric(fields, tags)
	rsName := "ReplicaSetTest"
	pod.OwnerReferences[0].Kind = ci.ReplicaSet
	pod.OwnerReferences[0].Name = rsName
	kubernetesBlob = map[string]any{}
	podStore.addPodOwnersAndPodName(metric, pod, kubernetesBlob)
	expectedOwner["pod_owners"] = []any{map[string]string{"owner_kind": ci.ReplicaSet, "owner_name": rsName}}
	expectedOwnerName = rsName
	assert.Equal(t, expectedOwnerName, metric.GetTag(ci.PodNameKey))
	assert.Equal(t, expectedOwner, kubernetesBlob)

	// Test StatefulSet
	metric = generateMetric(fields, tags)
	ssName := "StatefulSetTest"
	pod.OwnerReferences[0].Kind = ci.StatefulSet
	pod.OwnerReferences[0].Name = ssName
	kubernetesBlob = map[string]any{}
	podStore.addPodOwnersAndPodName(metric, pod, kubernetesBlob)
	expectedOwner["pod_owners"] = []any{map[string]string{"owner_kind": ci.StatefulSet, "owner_name": ssName}}
	expectedOwnerName = "cpu-limit"
	assert.Equal(t, expectedOwnerName, metric.GetTag(ci.PodNameKey))
	assert.Equal(t, expectedOwner, kubernetesBlob)

	// Test ReplicationController
	pod.Name = "this should not be in FullPodName"
	rcName := "ReplicationControllerTest"
	pod.OwnerReferences[0].Kind = ci.ReplicationController
	pod.OwnerReferences[0].Name = rcName
	kubernetesBlob = map[string]any{}
	podStore.addPodOwnersAndPodName(metric, pod, kubernetesBlob)
	expectedOwner["pod_owners"] = []any{map[string]string{"owner_kind": ci.ReplicationController, "owner_name": rcName}}
	expectedOwnerName = rcName
	assert.Equal(t, expectedOwnerName, metric.GetTag(ci.PodNameKey))
	assert.Empty(t, metric.GetTag(ci.FullPodNameKey))
	assert.Equal(t, expectedOwner, kubernetesBlob)

	// Test Job
	podStore.prefFullPodName = true
	podStore.addFullPodNameMetricLabel = true
	metric = generateMetric(fields, tags)
	jobName := "JobTest"
	pod.OwnerReferences[0].Kind = ci.Job
	suffixHash := ".088123x12"
	pod.Name = jobName + suffixHash
	pod.OwnerReferences[0].Name = jobName + suffixHash
	kubernetesBlob = map[string]any{}
	podStore.addPodOwnersAndPodName(metric, pod, kubernetesBlob)
	expectedOwner["pod_owners"] = []any{map[string]string{"owner_kind": ci.Job, "owner_name": jobName + suffixHash}}
	expectedOwnerName = jobName + suffixHash
	assert.Equal(t, expectedOwnerName, metric.GetTag(ci.PodNameKey))
	assert.Equal(t, pod.Name, metric.GetTag(ci.FullPodNameKey))
	assert.Equal(t, expectedOwner, kubernetesBlob)

	podStore.prefFullPodName = false
	kubernetesBlob = map[string]any{}
	podStore.addPodOwnersAndPodName(metric, pod, kubernetesBlob)
	expectedOwner["pod_owners"] = []any{map[string]string{"owner_kind": ci.Job, "owner_name": jobName}}
	expectedOwnerName = jobName
	assert.Equal(t, expectedOwnerName, metric.GetTag(ci.PodNameKey))
	assert.Equal(t, expectedOwner, kubernetesBlob)

	// Test Deployment
	metric = generateMetric(fields, tags)
	dpName := "DeploymentTest"
	pod.OwnerReferences[0].Kind = ci.ReplicaSet
	pod.OwnerReferences[0].Name = dpName + "-sftrz2785"
	kubernetesBlob = map[string]any{}
	podStore.addPodOwnersAndPodName(metric, pod, kubernetesBlob)
	expectedOwner["pod_owners"] = []any{map[string]string{"owner_kind": ci.Deployment, "owner_name": dpName}}
	expectedOwnerName = dpName
	assert.Equal(t, expectedOwnerName, metric.GetTag(ci.PodNameKey))
	assert.Equal(t, expectedOwner, kubernetesBlob)

	// Test CronJob
	metric = generateMetric(fields, tags)
	cjName := "CronJobTest"
	pod.OwnerReferences[0].Kind = ci.Job
	pod.OwnerReferences[0].Name = cjName + "-1556582405"
	kubernetesBlob = map[string]any{}
	podStore.addPodOwnersAndPodName(metric, pod, kubernetesBlob)
	expectedOwner["pod_owners"] = []any{map[string]string{"owner_kind": ci.CronJob, "owner_name": cjName}}
	expectedOwnerName = cjName
	assert.Equal(t, expectedOwnerName, metric.GetTag(ci.PodNameKey))
	assert.Equal(t, expectedOwner, kubernetesBlob)

	// Test kube-proxy created in kops
	podStore.prefFullPodName = true
	metric = generateMetric(fields, tags)
	kpName := kubeProxy + "-xyz1"
	pod.OwnerReferences = nil
	pod.Name = kpName
	kubernetesBlob = map[string]any{}
	podStore.addPodOwnersAndPodName(metric, pod, kubernetesBlob)
	assert.Equal(t, kpName, metric.GetTag(ci.PodNameKey))
	assert.Empty(t, kubernetesBlob)

	podStore.prefFullPodName = false
	metric = generateMetric(fields, tags)
	pod.OwnerReferences = nil
	pod.Name = kpName
	kubernetesBlob = map[string]any{}
	podStore.addPodOwnersAndPodName(metric, pod, kubernetesBlob)
	assert.Equal(t, kubeProxy, metric.GetTag(ci.PodNameKey))
	assert.Empty(t, kubernetesBlob)
}

type mockPodClient struct{}

func (m *mockPodClient) ListPods() ([]corev1.Pod, error) {
	pod := getBaseTestPodInfo()
	podList := []corev1.Pod{*pod}
	return podList, nil
}

func TestPodStore_RefreshTick(t *testing.T) {
	podStore := getPodStore()
	defer require.NoError(t, podStore.Shutdown())
	podStore.podClient = &mockPodClient{}
	podStore.lastRefreshed = time.Now().Add(-time.Minute)
	podStore.RefreshTick(context.Background())

	assert.Equal(t, uint64(10), podStore.nodeInfo.nodeStats.cpuReq)
	assert.Equal(t, uint64(50*1024*1024), podStore.nodeInfo.nodeStats.memReq)
	assert.Equal(t, 1, podStore.nodeInfo.nodeStats.podCnt)
	assert.Equal(t, 1, podStore.nodeInfo.nodeStats.containerCnt)
	assert.Equal(t, 1, podStore.cache.Size())
}

func TestPodStore_decorateNode(t *testing.T) {
	t.Setenv(ci.HostName, "testNode1")
	pod := getBaseTestPodInfo()
	podList := []corev1.Pod{*pod}
	podStore := getPodStore()
	defer require.NoError(t, podStore.Shutdown())
	podStore.refreshInternal(time.Now(), podList)

	tags := map[string]string{ci.MetricType: ci.TypeNode}
	fields := map[string]any{
		ci.MetricName(ci.TypeNode, ci.CPUTotal):      float64(100),
		ci.MetricName(ci.TypeNode, ci.CPULimit):      uint64(4000),
		ci.MetricName(ci.TypeNode, ci.MemWorkingset): float64(100 * 1024 * 1024),
		ci.MetricName(ci.TypeNode, ci.MemLimit):      uint64(400 * 1024 * 1024),
	}

	metric := generateMetric(fields, tags)
	podStore.decorateNode(metric)

	assert.Equal(t, uint64(10), metric.GetField("node_cpu_request").(uint64))
	assert.Equal(t, uint64(4000), metric.GetField("node_cpu_limit").(uint64))
	assert.Equal(t, float64(0.25), metric.GetField("node_cpu_reserved_capacity").(float64))
	assert.Equal(t, float64(100), metric.GetField("node_cpu_usage_total").(float64))

	assert.Equal(t, uint64(50*1024*1024), metric.GetField("node_memory_request").(uint64))
	assert.Equal(t, uint64(400*1024*1024), metric.GetField("node_memory_limit").(uint64))
	assert.Equal(t, float64(12.5), metric.GetField("node_memory_reserved_capacity").(float64))
	assert.Equal(t, float64(100*1024*1024), metric.GetField("node_memory_working_set").(float64))

	assert.Equal(t, int(1), metric.GetField("node_number_of_running_containers").(int))
	assert.Equal(t, int(1), metric.GetField("node_number_of_running_pods").(int))

	assert.False(t, metric.HasField("node_status_condition_ready"))
	assert.False(t, metric.HasField("node_status_condition_disk_pressure"))
	assert.False(t, metric.HasField("node_status_condition_memory_pressure"))
	assert.False(t, metric.HasField("node_status_condition_pid_pressure"))
	assert.False(t, metric.HasField("node_status_condition_network_unavailable"))
	assert.False(t, metric.HasField("node_status_condition_unknown"))

	assert.False(t, metric.HasField("node_status_capacity_pods"))
	assert.False(t, metric.HasField("node_status_allocatable_pods"))

	podStore.includeEnhancedMetrics = true
	podStore.enableAcceleratedComputeMetrics = true
	podStore.decorateNode(metric)

	assert.Equal(t, uint64(1), metric.GetField("node_gpu_request").(uint64))
	assert.Equal(t, uint64(20), metric.GetField("node_gpu_limit").(uint64))
	assert.Equal(t, uint64(1), metric.GetField("node_gpu_usage_total").(uint64))
	assert.Equal(t, float64(5), metric.GetField("node_gpu_reserved_capacity").(float64))

	assert.Equal(t, uint64(0), metric.GetField("node_neuroncore_request").(uint64))
	assert.Equal(t, uint64(32), metric.GetField("node_neuroncore_limit").(uint64))
	assert.Equal(t, uint64(0), metric.GetField("node_neuroncore_usage_total").(uint64))
	assert.Equal(t, float64(0), metric.GetField("node_neuroncore_reserved_capacity").(float64))
	assert.Equal(t, float64(100), metric.GetField("node_neuroncore_unreserved_capacity").(float64))
	assert.Equal(t, uint64(32), metric.GetField("node_neuroncore_available_capacity").(uint64))

	assert.Equal(t, uint64(1), metric.GetField("node_status_condition_ready").(uint64))
	assert.Equal(t, uint64(0), metric.GetField("node_status_condition_disk_pressure").(uint64))
	assert.Equal(t, uint64(0), metric.GetField("node_status_condition_memory_pressure").(uint64))
	assert.Equal(t, uint64(0), metric.GetField("node_status_condition_pid_pressure").(uint64))
	assert.Equal(t, uint64(0), metric.GetField("node_status_condition_network_unavailable").(uint64))
	assert.Equal(t, uint64(1), metric.GetField("node_status_condition_unknown").(uint64))

	assert.Equal(t, uint64(5), metric.GetField("node_status_capacity_pods").(uint64))
	assert.Equal(t, uint64(15), metric.GetField("node_status_allocatable_pods").(uint64))
}

func TestPodStore_decorateNode_multiplePodStates(t *testing.T) {
	podStore := getPodStore()
	defer require.NoError(t, podStore.Shutdown())

	tags := map[string]string{ci.MetricType: ci.TypeNode}
	fields := map[string]any{
		ci.MetricName(ci.TypeNode, ci.CPUTotal):      float64(100),
		ci.MetricName(ci.TypeNode, ci.CPULimit):      uint64(4000),
		ci.MetricName(ci.TypeNode, ci.MemWorkingset): float64(100 * 1024 * 1024),
		ci.MetricName(ci.TypeNode, ci.MemLimit):      uint64(400 * 1024 * 1024),
	}
	metric := generateMetric(fields, tags)

	// terminated pods should not contribute to requests
	failedPod := generatePodInfo("./test_resources/pod_in_phase_failed.json")
	succeededPod := generatePodInfo("./test_resources/pod_in_phase_succeeded.json")
	podList := []corev1.Pod{*failedPod, *succeededPod}
	podStore.refreshInternal(time.Now(), podList)
	podStore.decorateNode(metric)

	assert.Equal(t, uint64(0), metric.GetField("node_cpu_request").(uint64))
	assert.Equal(t, uint64(4000), metric.GetField("node_cpu_limit").(uint64))
	assert.Equal(t, float64(0), metric.GetField("node_cpu_reserved_capacity").(float64))
	assert.Equal(t, float64(100), metric.GetField("node_cpu_usage_total").(float64))

	assert.Equal(t, uint64(0), metric.GetField("node_memory_request").(uint64))
	assert.Equal(t, uint64(400*1024*1024), metric.GetField("node_memory_limit").(uint64))
	assert.Equal(t, float64(0), metric.GetField("node_memory_reserved_capacity").(float64))
	assert.Equal(t, float64(100*1024*1024), metric.GetField("node_memory_working_set").(float64))

	// non-terminated pods should contribute to requests
	pendingPod := generatePodInfo("./test_resources/pod_in_phase_pending.json")
	podList = append(podList, *pendingPod)
	podStore.refreshInternal(time.Now(), podList)
	podStore.decorateNode(metric)
	assert.Equal(t, uint64(10), metric.GetField("node_cpu_request").(uint64))
	assert.Equal(t, float64(0.25), metric.GetField("node_cpu_reserved_capacity").(float64))

	assert.Equal(t, uint64(50*1024*1024), metric.GetField("node_memory_request").(uint64))
	assert.Equal(t, float64(12.5), metric.GetField("node_memory_reserved_capacity").(float64))

	runningPod := generatePodInfo("./test_resources/pod_in_phase_running.json")
	podList = append(podList, *runningPod)
	podStore.refreshInternal(time.Now(), podList)
	podStore.decorateNode(metric)

	assert.Equal(t, uint64(20), metric.GetField("node_cpu_request").(uint64))
	assert.Equal(t, float64(0.5), metric.GetField("node_cpu_reserved_capacity").(float64))

	assert.Equal(t, uint64(100*1024*1024), metric.GetField("node_memory_request").(uint64))
	assert.Equal(t, float64(25), metric.GetField("node_memory_reserved_capacity").(float64))
}

func TestPodStore_decorateNode_withNeuron(t *testing.T) {
	t.Setenv(ci.HostName, "testNode1")
	pod := getBaseTestPodInfo()
	pod.Spec.Containers[0].Resources.Requests["aws.amazon.com/neuron"] = resource.MustParse("1")
	pod.Spec.Containers[0].Resources.Limits["aws.amazon.com/neuron"] = resource.MustParse("2")

	podList := []corev1.Pod{*pod}
	podStore := getPodStoreWithNeuronCapacity()
	defer require.NoError(t, podStore.Shutdown())
	podStore.refreshInternal(time.Now(), podList)

	tags := map[string]string{ci.MetricType: ci.TypeNode}
	fields := map[string]any{
		ci.MetricName(ci.TypeNode, ci.CPUTotal):      float64(100),
		ci.MetricName(ci.TypeNode, ci.CPULimit):      uint64(4000),
		ci.MetricName(ci.TypeNode, ci.MemWorkingset): float64(100 * 1024 * 1024),
		ci.MetricName(ci.TypeNode, ci.MemLimit):      uint64(400 * 1024 * 1024),
	}

	metric := generateMetric(fields, tags)
	podStore.includeEnhancedMetrics = true
	podStore.enableAcceleratedComputeMetrics = true
	podStore.decorateNode(metric)

	assert.Equal(t, uint64(2), metric.GetField("node_neuroncore_request").(uint64))
	assert.Equal(t, uint64(32), metric.GetField("node_neuroncore_limit").(uint64))
	assert.Equal(t, uint64(4), metric.GetField("node_neuroncore_usage_total").(uint64))
	assert.Equal(t, float64(6.25), metric.GetField("node_neuroncore_reserved_capacity").(float64))
	assert.Equal(t, float64(93.75), metric.GetField("node_neuroncore_unreserved_capacity").(float64))
	assert.Equal(t, uint64(30), metric.GetField("node_neuroncore_available_capacity").(uint64))
}

func TestPodStore_decorateNode_withNeuroncore(t *testing.T) {
	t.Setenv(ci.HostName, "testNode1")
	pod := getBaseTestPodInfo()
	pod.Spec.Containers[0].Resources.Requests["aws.amazon.com/neuroncore"] = resource.MustParse("1")
	pod.Spec.Containers[0].Resources.Limits["aws.amazon.com/neuroncore"] = resource.MustParse("2")

	podList := []corev1.Pod{*pod}
	podStore := getPodStoreWithNeuronCapacity()
	defer require.NoError(t, podStore.Shutdown())
	podStore.refreshInternal(time.Now(), podList)

	tags := map[string]string{ci.MetricType: ci.TypeNode}
	fields := map[string]any{
		ci.MetricName(ci.TypeNode, ci.CPUTotal):      float64(100),
		ci.MetricName(ci.TypeNode, ci.CPULimit):      uint64(4000),
		ci.MetricName(ci.TypeNode, ci.MemWorkingset): float64(100 * 1024 * 1024),
		ci.MetricName(ci.TypeNode, ci.MemLimit):      uint64(400 * 1024 * 1024),
	}

	metric := generateMetric(fields, tags)
	podStore.includeEnhancedMetrics = true
	podStore.enableAcceleratedComputeMetrics = true
	podStore.decorateNode(metric)

	assert.Equal(t, uint64(1), metric.GetField("node_neuroncore_request").(uint64))
	assert.Equal(t, uint64(32), metric.GetField("node_neuroncore_limit").(uint64))
	assert.Equal(t, uint64(2), metric.GetField("node_neuroncore_usage_total").(uint64))
	assert.Equal(t, float64(3.125), metric.GetField("node_neuroncore_reserved_capacity").(float64))
	assert.Equal(t, float64(96.875), metric.GetField("node_neuroncore_unreserved_capacity").(float64))
	assert.Equal(t, uint64(31), metric.GetField("node_neuroncore_available_capacity").(uint64))
}

func TestPodStore_decorateNeuron_oddCoreAllocation(t *testing.T) {
	podStore := getPodStoreWithNeuronCapacity()
	defer require.NoError(t, podStore.Shutdown())

	pod := getBaseTestPodInfo()
	pod.Spec.Containers[0].Resources.Limits["aws.amazon.com/neuroncore"] = resource.MustParse("5")

	podList := []corev1.Pod{*pod}
	podStore.refreshInternal(time.Now(), podList)

	tags := map[string]string{ci.MetricType: ci.TypePod}
	fields := map[string]any{}

	metric := generateMetric(fields, tags)
	podStore.includeEnhancedMetrics = true
	podStore.enableAcceleratedComputeMetrics = true
	podStore.decorateNeuron(metric, pod)

	assert.Equal(t, uint64(0), metric.GetField("pod_neuroncore_request").(uint64))
	assert.Equal(t, uint64(5), metric.GetField("pod_neuroncore_limit").(uint64))
	assert.Equal(t, uint64(5), metric.GetField("pod_neuroncore_usage_total").(uint64))
	assert.Equal(t, float64(15.625), metric.GetField("pod_neuroncore_reserved_capacity").(float64))
}

func TestNodeInfo_getNeuronCoresPerDevice(t *testing.T) {
	nodeInfo := newNodeInfo("testNode1", &mockNodeInfoProvider{}, zap.NewNop())
	coresPerDevice, hasRatio := nodeInfo.getNeuronCoresPerDevice()

	assert.True(t, hasRatio)
	assert.Equal(t, 2, coresPerDevice)
}

func TestPodStore_Decorate(t *testing.T) {
	// not the metrics for decoration
	tags := map[string]string{}
	metric := &mockCIMetric{
		tags: tags,
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	podStore := getPodStore()
	defer require.NoError(t, podStore.Shutdown())
	podStore.podClient = &mockPodClient{}
	kubernetesBlob := map[string]any{}
	ok := podStore.Decorate(ctx, metric, kubernetesBlob)
	assert.True(t, ok)

	// metric with no namespace
	tags = map[string]string{
		ci.ContainerNamekey: "testContainer",
		ci.PodIDKey:         "123",
		ci.K8sPodNameKey:    "testPod",
		// ci.K8sNamespace:     "testNamespace",
		ci.TypeService: "testService",
		ci.NodeNameKey: "testNode",
	}
	metric = &mockCIMetric{
		tags: tags,
	}
	ok = podStore.Decorate(ctx, metric, kubernetesBlob)
	assert.False(t, ok)

	// metric with pod info not in cache
	tags = map[string]string{
		ci.ContainerNamekey: "testContainer",
		ci.K8sPodNameKey:    "testPod",
		ci.PodIDKey:         "123",
		ci.K8sNamespace:     "testNamespace",
		ci.TypeService:      "testService",
		ci.NodeNameKey:      "testNode",
	}
	metric = &mockCIMetric{
		tags: tags,
	}
	ok = podStore.Decorate(ctx, metric, kubernetesBlob)
	assert.False(t, ok)

	// decorate the same metric with a placeholder item in cache
	ok = podStore.Decorate(ctx, metric, kubernetesBlob)
	assert.False(t, ok)
}

func runAddStatusToGetDecoratedCIMetric(podInfoSourceFileName string, includeEnhancedMetrics bool) CIMetric {
	podInfo := generatePodInfo(podInfoSourceFileName)
	podStore := getPodStore()
	podStore.includeEnhancedMetrics = includeEnhancedMetrics
	rawCIMetric := generateRawCIMetric()
	podStore.addStatus(rawCIMetric, podInfo)
	return rawCIMetric
}

func generatePodInfo(sourceFileName string) *corev1.Pod {
	podInfoJSON, err := os.ReadFile(sourceFileName)
	if err != nil {
		panic(fmt.Sprintf("reading file failed %v", err))
	}
	pods := corev1.PodList{}
	err = json.Unmarshal(podInfoJSON, &pods)
	if err != nil {
		panic(fmt.Sprintf("unmarshal pod err %v", err))
	}
	return &pods.Items[0]
}

func generateRawCIMetric() CIMetric {
	tags := map[string]string{ci.MetricType: ci.TypePod, ci.K8sNamespace: "default", ci.K8sPodNameKey: "cpu-limit"}
	fields := map[string]any{ci.MetricName(ci.TypePod, ci.CPUTotal): float64(1)}
	return generateMetric(fields, tags)
}
