// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xk8stest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

func CreateCollectorObjects(t *testing.T, client *K8sClient, testID, manifestsDir string, templateValues map[string]string, host string) []*unstructured.Unstructured {
	if manifestsDir == "" {
		manifestsDir = filepath.Join(".", "testdata", "e2e", "collector")
	}
	manifestFiles, err := os.ReadDir(manifestsDir)
	require.NoErrorf(t, err, "failed to read collector manifests directory %s", manifestsDir)
	if host == "" {
		host = HostEndpoint(t)
	}
	var podNamespace string
	var podLabels map[string]any
	createdObjs := make([]*unstructured.Unstructured, 0, len(manifestFiles))
	for _, manifestFile := range manifestFiles {
		tmpl := template.Must(template.New(manifestFile.Name()).ParseFiles(filepath.Join(manifestsDir, manifestFile.Name())))
		manifest := &bytes.Buffer{}
		defaultTemplateValues := map[string]string{
			"Name":         "otelcol-" + testID,
			"HostEndpoint": host,
			"TestID":       testID,
		}
		maps.Copy(defaultTemplateValues, templateValues)
		require.NoError(t, tmpl.Execute(manifest, defaultTemplateValues))
		obj, err := CreateObject(client, manifest.Bytes())
		require.NoErrorf(t, err, "failed to create collector object from manifest %s", manifestFile.Name())
		objKind := obj.GetKind()
		if objKind == "Deployment" || objKind == "DaemonSet" {
			podNamespace = obj.GetNamespace()
			selector := obj.Object["spec"].(map[string]any)["selector"]
			podLabels = selector.(map[string]any)["matchLabels"].(map[string]any)
		}
		createdObjs = append(createdObjs, obj)
	}

	WaitForCollectorToStart(t, client, podNamespace, podLabels)

	return createdObjs
}

func WaitForCollectorToStart(t *testing.T, client *K8sClient, podNamespace string, podLabels map[string]any) {
	podGVR := schema.GroupVersionResource{Version: "v1", Resource: "pods"}
	listOptions := metav1.ListOptions{LabelSelector: SelectorFromMap(podLabels).String()}
	timeout := 3 * time.Minute
	poll := 2 * time.Second
	deadline := time.Now().Add(timeout)

	t.Logf("waiting for collector pods to be ready")
	for time.Now().Before(deadline) {
		if collectorPodsReady(t, client, podNamespace, podGVR, listOptions) {
			t.Logf("collector pods are ready")
			return
		}
		time.Sleep(poll)
	}

	logCollectorPodDiagnostics(t, client, podNamespace, podGVR, listOptions)
	require.Fail(t, "collector pods were not ready", "timed out after %s", timeout)
}

func collectorPodsReady(t *testing.T, client *K8sClient, namespace string, podGVR schema.GroupVersionResource, listOptions metav1.ListOptions) bool {
	t.Helper()
	list, err := client.DynamicClient.Resource(podGVR).Namespace(namespace).List(t.Context(), listOptions)
	require.NoError(t, err, "failed to list collector pods")
	if len(list.Items) == 0 {
		t.Log("did not find collector pods")
		return false
	}

	var pods v1.PodList
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(list.UnstructuredContent(), &pods)
	require.NoError(t, err, "failed to convert unstructured to podList")

	podsNotReady := len(pods.Items)
	for i := range pods.Items {
		pod := &pods.Items[i]
		if pod.Status.Phase != v1.PodRunning {
			t.Logf("pod %v is not running, current phase: %v", pod.Name, pod.Status.Phase)
			continue
		}
		for _, cond := range pod.Status.Conditions {
			if cond.Type == v1.PodReady && cond.Status == v1.ConditionTrue {
				podsNotReady--
			}
		}
	}
	return podsNotReady == 0
}

func logCollectorPodDiagnostics(t *testing.T, client *K8sClient, namespace string, podGVR schema.GroupVersionResource, listOptions metav1.ListOptions) {
	t.Helper()
	list, err := client.DynamicClient.Resource(podGVR).Namespace(namespace).List(t.Context(), listOptions)
	if err != nil {
		return
	}
	var pods v1.PodList
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(list.UnstructuredContent(), &pods); err != nil {
		return
	}
	for i := range pods.Items {
		pod := &pods.Items[i]
		if podReady(pod) {
			continue
		}
		t.Logf("--- events for pod %s (phase: %s) ---", pod.Name, pod.Status.Phase)
		logPodEvents(t, client, namespace, pod.Name)
		logRestartingContainers(t, client, namespace, pod.Name, pod.Status.InitContainerStatuses)
		logRestartingContainers(t, client, namespace, pod.Name, pod.Status.ContainerStatuses)
	}
}

func podReady(pod *v1.Pod) bool {
	if pod.Status.Phase != v1.PodRunning {
		return false
	}
	for _, cond := range pod.Status.Conditions {
		if cond.Type == v1.PodReady && cond.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}

func logPodEvents(t *testing.T, client *K8sClient, namespace, podName string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	eventsGVR := schema.GroupVersionResource{Version: "v1", Resource: "events"}
	list, err := client.DynamicClient.Resource(eventsGVR).Namespace(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Pod", podName),
	})
	if err != nil {
		t.Logf("  failed to list events: %v", err)
		return
	}
	if len(list.Items) == 0 {
		t.Logf("  no events found")
		return
	}
	for _, item := range list.Items {
		var event v1.Event
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.Object, &event); err != nil {
			continue
		}
		t.Logf("  %s  %s  %s  %s",
			event.Type,
			event.Reason,
			event.Source.Component,
			event.Message)
	}
}

func logRestartingContainers(t *testing.T, client *K8sClient, namespace, podName string, statuses []v1.ContainerStatus) {
	t.Helper()
	for i := range statuses {
		cs := &statuses[i]
		if cs.RestartCount == 0 {
			continue
		}
		reason := ""
		if cs.LastTerminationState.Terminated != nil {
			reason = cs.LastTerminationState.Terminated.Reason
		}
		t.Logf("--- logs for container %s in pod %s (restarts: %d, last exit reason: %s) ---", cs.Name, podName, cs.RestartCount, reason)
		logContainerLogs(t, client, namespace, podName, cs.Name)
	}
}

func logContainerLogs(t *testing.T, client *K8sClient, namespace, podName, containerName string) {
	t.Helper()
	coreClient, err := corev1client.NewForConfig(client.restConfig)
	if err != nil {
		t.Logf("  failed to create client for logs: %v", err)
		return
	}

	tailLines := int64(25)
	for _, previous := range []bool{false, true} {
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		logStr := fetchContainerLogs(ctx, coreClient, namespace, podName, containerName, previous, &tailLines)
		cancel()
		if logStr != "" {
			for line := range strings.SplitSeq(logStr, "\n") {
				t.Logf("  %s", line)
			}
			return
		}
	}
	t.Logf("  no logs available")
}

func fetchContainerLogs(ctx context.Context, coreClient corev1client.CoreV1Interface, namespace, podName, containerName string, previous bool, tailLines *int64) string {
	stream, err := coreClient.Pods(namespace).GetLogs(podName, &v1.PodLogOptions{
		Container: containerName,
		Previous:  previous,
		TailLines: tailLines,
	}).Stream(ctx)
	if err != nil {
		return ""
	}
	defer stream.Close()

	logs, err := io.ReadAll(stream)
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(logs), "\n")
}
