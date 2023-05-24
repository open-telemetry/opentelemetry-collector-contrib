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

package stores

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"

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
                "memory": "50Mi"
              },
              "requests": {
                "cpu": "10m",
                "memory": "50Mi"
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
		prevMeasurements: make(map[string]*mapWithExpiry),
		logger:           zap.NewNop(),
	}
}

func generateMetric(fields map[string]interface{}, tags map[string]string) CIMetric {
	return &mockCIMetric{
		tags:   tags,
		fields: fields,
	}
}

func TestPodStore_decorateCpu(t *testing.T) {
	podStore := getPodStore()

	pod := getBaseTestPodInfo()

	// test pod metrics
	tags := map[string]string{ci.MetricType: ci.TypePod}
	fields := map[string]interface{}{ci.MetricName(ci.TypePod, ci.CPUTotal): float64(1)}

	metric := generateMetric(fields, tags)
	podStore.decorateCPU(metric, pod)

	assert.Equal(t, uint64(10), metric.GetField("pod_cpu_request").(uint64))
	assert.Equal(t, uint64(10), metric.GetField("pod_cpu_limit").(uint64))
	assert.Equal(t, float64(0.25), metric.GetField("pod_cpu_reserved_capacity").(float64))
	assert.Equal(t, float64(10), metric.GetField("pod_cpu_utilization_over_pod_limit").(float64))
	assert.Equal(t, float64(1), metric.GetField("pod_cpu_usage_total").(float64))

	// test container metrics
	tags = map[string]string{ci.MetricType: ci.TypeContainer, ci.ContainerNamekey: "ubuntu"}
	fields = map[string]interface{}{ci.MetricName(ci.TypeContainer, ci.CPUTotal): float64(1)}
	metric = generateMetric(fields, tags)
	podStore.decorateCPU(metric, pod)

	assert.Equal(t, uint64(10), metric.GetField("container_cpu_request").(uint64))
	assert.Equal(t, uint64(10), metric.GetField("container_cpu_limit").(uint64))
	assert.Equal(t, float64(1), metric.GetField("container_cpu_usage_total").(float64))
}

func TestPodStore_decorateMem(t *testing.T) {
	podStore := getPodStore()
	pod := getBaseTestPodInfo()

	tags := map[string]string{ci.MetricType: ci.TypePod}
	fields := map[string]interface{}{ci.MetricName(ci.TypePod, ci.MemWorkingset): uint64(10 * 1024 * 1024)}

	metric := generateMetric(fields, tags)
	podStore.decorateMem(metric, pod)

	assert.Equal(t, uint64(52428800), metric.GetField("pod_memory_request").(uint64))
	assert.Equal(t, uint64(52428800), metric.GetField("pod_memory_limit").(uint64))
	assert.Equal(t, float64(12.5), metric.GetField("pod_memory_reserved_capacity").(float64))
	assert.Equal(t, float64(20), metric.GetField("pod_memory_utilization_over_pod_limit").(float64))
	assert.Equal(t, uint64(10*1024*1024), metric.GetField("pod_memory_working_set").(uint64))

	tags = map[string]string{ci.MetricType: ci.TypeContainer, ci.ContainerNamekey: "ubuntu"}
	fields = map[string]interface{}{ci.MetricName(ci.TypeContainer, ci.MemWorkingset): float64(10 * 1024 * 1024)}

	metric = generateMetric(fields, tags)
	podStore.decorateMem(metric, pod)

	assert.Equal(t, uint64(52428800), metric.GetField("container_memory_request").(uint64))
	assert.Equal(t, uint64(52428800), metric.GetField("container_memory_limit").(uint64))
	assert.Equal(t, float64(10*1024*1024), metric.GetField("container_memory_working_set").(float64))
}

func TestPodStore_addContainerCount(t *testing.T) {
	pod := getBaseTestPodInfo()

	tags := map[string]string{ci.MetricType: ci.TypePod}
	fields := map[string]interface{}{ci.MetricName(ci.TypePod, ci.CPUTotal): float64(1)}

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

func TestPodStore_addStatus(t *testing.T) {
	pod := getBaseTestPodInfo()
	tags := map[string]string{ci.MetricType: ci.TypePod, ci.K8sNamespace: "default", ci.K8sPodNameKey: "cpu-limit"}
	fields := map[string]interface{}{ci.MetricName(ci.TypePod, ci.CPUTotal): float64(1)}
	podStore := getPodStore()
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
	fields := map[string]interface{}{ci.MetricName(ci.TypePod, ci.CPUTotal): float64(1)}
	kubernetesBlob := map[string]interface{}{}
	metric := generateMetric(fields, tags)
	addContainerID(pod, metric, kubernetesBlob, zap.NewNop())

	expected := map[string]interface{}{}
	expected["docker"] = map[string]string{"container_id": "637631e2634ea92c0c1aa5d24734cfe794f09c57933026592c12acafbaf6972c"}
	assert.Equal(t, expected, kubernetesBlob)
	assert.Equal(t, metric.GetTag(ci.ContainerNamekey), "ubuntu")

	tags = map[string]string{ci.ContainerNamekey: "notUbuntu", ci.ContainerIDkey: "123"}
	kubernetesBlob = map[string]interface{}{}
	metric = generateMetric(fields, tags)
	addContainerID(pod, metric, kubernetesBlob, zap.NewNop())

	expected = map[string]interface{}{}
	expected["container_id"] = "123"
	assert.Equal(t, expected, kubernetesBlob)
	assert.Equal(t, metric.GetTag(ci.ContainerNamekey), "notUbuntu")
}

func TestPodStore_addLabel(t *testing.T) {
	pod := getBaseTestPodInfo()
	kubernetesBlob := map[string]interface{}{}
	addLabels(pod, kubernetesBlob)
	expected := map[string]interface{}{}
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
	assert.Equal(t, "", getJobNamePrefix(".abcd-efg"))
	assert.Equal(t, "", getJobNamePrefix(""))
}

type mockReplicaSetInfo1 struct{}

func (m *mockReplicaSetInfo1) ReplicaSetToDeployment() map[string]string {
	return map[string]string{}
}

type mockK8sClient1 struct{}

func (m *mockK8sClient1) GetReplicaSetClient() k8sclient.ReplicaSetClient {
	return &mockReplicaSetInfo1{}
}

type mockReplicaSetInfo2 struct{}

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
	fields := map[string]interface{}{ci.MetricName(ci.TypePod, ci.CPUTotal): float64(1)}
	metric := generateMetric(fields, tags)

	// Test ReplicaSet
	rsName := "ReplicaSetTest"
	suffix := "-42kcz"
	pod.OwnerReferences[0].Kind = ci.ReplicaSet
	pod.OwnerReferences[0].Name = rsName + suffix
	kubernetesBlob := map[string]interface{}{}
	podStore.addPodOwnersAndPodName(metric, pod, kubernetesBlob)
	expectedOwner := map[string]interface{}{}
	expectedOwner["pod_owners"] = []interface{}{map[string]string{"owner_kind": ci.Deployment, "owner_name": rsName}}
	expectedOwnerName := rsName
	assert.Equal(t, expectedOwnerName, metric.GetTag(ci.PodNameKey))
	assert.Equal(t, expectedOwner, kubernetesBlob)

	// Test Job
	metric = generateMetric(fields, tags)
	jobName := "Job"
	suffix = "-0123456789"
	pod.OwnerReferences[0].Kind = ci.Job
	pod.OwnerReferences[0].Name = jobName + suffix
	kubernetesBlob = map[string]interface{}{}
	podStore.addPodOwnersAndPodName(metric, pod, kubernetesBlob)
	expectedOwner["pod_owners"] = []interface{}{map[string]string{"owner_kind": ci.CronJob, "owner_name": jobName}}
	expectedOwnerName = jobName
	assert.Equal(t, expectedOwnerName, metric.GetTag(ci.PodNameKey))
	assert.Equal(t, expectedOwner, kubernetesBlob)
}

func TestPodStore_addPodOwnersAndPodName(t *testing.T) {
	podStore := &PodStore{k8sClient: &mockK8sClient2{}}

	pod := getBaseTestPodInfo()
	tags := map[string]string{ci.MetricType: ci.TypePod, ci.ContainerNamekey: "ubuntu"}
	fields := map[string]interface{}{ci.MetricName(ci.TypePod, ci.CPUTotal): float64(1)}

	// Test DaemonSet
	metric := generateMetric(fields, tags)
	kubernetesBlob := map[string]interface{}{}
	podStore.addPodOwnersAndPodName(metric, pod, kubernetesBlob)

	expectedOwner := map[string]interface{}{}
	expectedOwner["pod_owners"] = []interface{}{map[string]string{"owner_kind": ci.DaemonSet, "owner_name": "DaemonSetTest"}}
	expectedOwnerName := "DaemonSetTest"
	assert.Equal(t, expectedOwnerName, metric.GetTag(ci.PodNameKey))
	assert.Equal(t, expectedOwner, kubernetesBlob)

	// Test ReplicaSet
	metric = generateMetric(fields, tags)
	rsName := "ReplicaSetTest"
	pod.OwnerReferences[0].Kind = ci.ReplicaSet
	pod.OwnerReferences[0].Name = rsName
	kubernetesBlob = map[string]interface{}{}
	podStore.addPodOwnersAndPodName(metric, pod, kubernetesBlob)
	expectedOwner["pod_owners"] = []interface{}{map[string]string{"owner_kind": ci.ReplicaSet, "owner_name": rsName}}
	expectedOwnerName = rsName
	assert.Equal(t, expectedOwnerName, metric.GetTag(ci.PodNameKey))
	assert.Equal(t, expectedOwner, kubernetesBlob)

	// Test StatefulSet
	metric = generateMetric(fields, tags)
	ssName := "StatefulSetTest"
	pod.OwnerReferences[0].Kind = ci.StatefulSet
	pod.OwnerReferences[0].Name = ssName
	kubernetesBlob = map[string]interface{}{}
	podStore.addPodOwnersAndPodName(metric, pod, kubernetesBlob)
	expectedOwner["pod_owners"] = []interface{}{map[string]string{"owner_kind": ci.StatefulSet, "owner_name": ssName}}
	expectedOwnerName = "cpu-limit"
	assert.Equal(t, expectedOwnerName, metric.GetTag(ci.PodNameKey))
	assert.Equal(t, expectedOwner, kubernetesBlob)

	// Test ReplicationController
	pod.Name = "this should not be in FullPodNameKey"
	rcName := "ReplicationControllerTest"
	pod.OwnerReferences[0].Kind = ci.ReplicationController
	pod.OwnerReferences[0].Name = rcName
	kubernetesBlob = map[string]interface{}{}
	podStore.addPodOwnersAndPodName(metric, pod, kubernetesBlob)
	expectedOwner["pod_owners"] = []interface{}{map[string]string{"owner_kind": ci.ReplicationController, "owner_name": rcName}}
	expectedOwnerName = rcName
	assert.Equal(t, expectedOwnerName, metric.GetTag(ci.PodNameKey))
	assert.Equal(t, "", metric.GetTag(ci.FullPodNameKey))
	assert.Equal(t, expectedOwner, kubernetesBlob)

	// Test Job
	podStore.prefFullPodName = true
	podStore.addFullPodNameMetricLabel = true
	metric = generateMetric(fields, tags)
	jobName := "JobTest"
	pod.OwnerReferences[0].Kind = ci.Job
	surfixHash := ".088123x12"
	pod.Name = jobName + surfixHash
	pod.OwnerReferences[0].Name = jobName + surfixHash
	kubernetesBlob = map[string]interface{}{}
	podStore.addPodOwnersAndPodName(metric, pod, kubernetesBlob)
	expectedOwner["pod_owners"] = []interface{}{map[string]string{"owner_kind": ci.Job, "owner_name": jobName + surfixHash}}
	expectedOwnerName = jobName + surfixHash
	assert.Equal(t, expectedOwnerName, metric.GetTag(ci.PodNameKey))
	assert.Equal(t, pod.Name, metric.GetTag(ci.FullPodNameKey))
	assert.Equal(t, expectedOwner, kubernetesBlob)

	podStore.prefFullPodName = false
	kubernetesBlob = map[string]interface{}{}
	podStore.addPodOwnersAndPodName(metric, pod, kubernetesBlob)
	expectedOwner["pod_owners"] = []interface{}{map[string]string{"owner_kind": ci.Job, "owner_name": jobName}}
	expectedOwnerName = jobName
	assert.Equal(t, expectedOwnerName, metric.GetTag(ci.PodNameKey))
	assert.Equal(t, expectedOwner, kubernetesBlob)

	// Test Deployment
	metric = generateMetric(fields, tags)
	dpName := "DeploymentTest"
	pod.OwnerReferences[0].Kind = ci.ReplicaSet
	pod.OwnerReferences[0].Name = dpName + "-sftrz2785"
	kubernetesBlob = map[string]interface{}{}
	podStore.addPodOwnersAndPodName(metric, pod, kubernetesBlob)
	expectedOwner["pod_owners"] = []interface{}{map[string]string{"owner_kind": ci.Deployment, "owner_name": dpName}}
	expectedOwnerName = dpName
	assert.Equal(t, expectedOwnerName, metric.GetTag(ci.PodNameKey))
	assert.Equal(t, expectedOwner, kubernetesBlob)

	// Test CronJob
	metric = generateMetric(fields, tags)
	cjName := "CronJobTest"
	pod.OwnerReferences[0].Kind = ci.Job
	pod.OwnerReferences[0].Name = cjName + "-1556582405"
	kubernetesBlob = map[string]interface{}{}
	podStore.addPodOwnersAndPodName(metric, pod, kubernetesBlob)
	expectedOwner["pod_owners"] = []interface{}{map[string]string{"owner_kind": ci.CronJob, "owner_name": cjName}}
	expectedOwnerName = cjName
	assert.Equal(t, expectedOwnerName, metric.GetTag(ci.PodNameKey))
	assert.Equal(t, expectedOwner, kubernetesBlob)

	// Test kube-proxy created in kops
	podStore.prefFullPodName = true
	metric = generateMetric(fields, tags)
	kpName := kubeProxy + "-xyz1"
	pod.OwnerReferences = nil
	pod.Name = kpName
	kubernetesBlob = map[string]interface{}{}
	podStore.addPodOwnersAndPodName(metric, pod, kubernetesBlob)
	assert.Equal(t, kpName, metric.GetTag(ci.PodNameKey))
	assert.True(t, len(kubernetesBlob) == 0)

	podStore.prefFullPodName = false
	metric = generateMetric(fields, tags)
	pod.OwnerReferences = nil
	pod.Name = kpName
	kubernetesBlob = map[string]interface{}{}
	podStore.addPodOwnersAndPodName(metric, pod, kubernetesBlob)
	assert.Equal(t, kubeProxy, metric.GetTag(ci.PodNameKey))
	assert.True(t, len(kubernetesBlob) == 0)
}

type mockPodClient struct {
}

func (m *mockPodClient) ListPods() ([]corev1.Pod, error) {
	pod := getBaseTestPodInfo()
	podList := []corev1.Pod{*pod}
	return podList, nil
}

func TestPodStore_RefreshTick(t *testing.T) {

	podStore := getPodStore()
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
	t.Setenv("HOST_NAME", "testNode1")
	pod := getBaseTestPodInfo()
	podList := []corev1.Pod{*pod}

	podStore := getPodStore()
	podStore.refreshInternal(time.Now(), podList)

	tags := map[string]string{ci.MetricType: ci.TypeNode}
	fields := map[string]interface{}{
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

	assert.Equal(t, uint64(1), metric.GetField("node_status_condition_ready").(uint64))
	assert.Equal(t, uint64(0), metric.GetField("node_status_condition_disk_pressure").(uint64))
	assert.Equal(t, uint64(0), metric.GetField("node_status_condition_memory_pressure").(uint64))
	assert.Equal(t, uint64(0), metric.GetField("node_status_condition_pid_pressure").(uint64))
	assert.Equal(t, uint64(0), metric.GetField("node_status_condition_network_unavailable").(uint64))

	assert.Equal(t, uint64(5), metric.GetField("node_status_capacity_pods").(uint64))
	assert.Equal(t, uint64(15), metric.GetField("node_status_allocatable_pods").(uint64))
}

func TestPodStore_Decorate(t *testing.T) {
	// not the metrics for decoration
	tags := map[string]string{}
	metric := &mockCIMetric{
		tags: tags,
	}

	podStore := getPodStore()
	podStore.podClient = &mockPodClient{}
	kubernetesBlob := map[string]interface{}{}
	ctx := context.Background()
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
