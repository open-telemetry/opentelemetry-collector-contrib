// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xk8stest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"testing"
	"text/template"
	"time"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
		podReady := false
		if pod.Status.Phase != v1.PodRunning {
			t.Logf("pod %v is not running, current phase: %v", pod.Name, pod.Status.Phase)
			continue
		}
		for _, cond := range pod.Status.Conditions {
			if cond.Type == v1.PodReady && cond.Status == v1.ConditionTrue {
				podsNotReady--
				podReady = true
			}
		}
		// Add some debug logs for crashing pods
		if !podReady {
			for i := range pod.Status.ContainerStatuses {
				cs := &pod.Status.ContainerStatuses[i]
				restartCount := cs.RestartCount
				if restartCount > 0 && cs.LastTerminationState.Terminated != nil {
					t.Logf("restart count = %d for container %s in pod %s, last terminated reason: %s", restartCount, cs.Name, pod.Name, cs.LastTerminationState.Terminated.Reason)
					t.Logf("termination message: %s", cs.LastTerminationState.Terminated.Message)
				}
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
	if err != nil || len(list.Items) == 0 {
		t.Logf("  no events found")
		return
	}
	for _, item := range list.Items {
		fields := item.Object
		source := ""
		if s, ok := fields["source"].(map[string]any); ok {
			source = strField(s, "component")
		}
		t.Logf("  %s  %s  %s  %s",
			strField(fields, "type"),
			strField(fields, "reason"),
			source,
			strField(fields, "message"))
	}
}

func strField(obj map[string]any, key string) string {
	if v, ok := obj[key].(string); ok {
		return v
	}
	return ""
}
