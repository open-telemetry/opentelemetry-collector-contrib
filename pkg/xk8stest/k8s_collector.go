// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xk8stest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"

import (
	"bytes"
	"context"
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

func CreateCollectorObjects(t *testing.T, client *K8sClient, testID string, manifestsDir string, templateValues map[string]string, host string) []*unstructured.Unstructured {
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
		for key, value := range templateValues {
			defaultTemplateValues[key] = value
		}
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
	podTimeoutMinutes := 3
	t.Logf("waiting for collector pods to be ready")
	require.Eventuallyf(t, func() bool {
		list, err := client.DynamicClient.Resource(podGVR).Namespace(podNamespace).List(context.Background(), listOptions)
		require.NoError(t, err, "failed to list collector pods")
		podsNotReady := len(list.Items)
		if podsNotReady == 0 {
			t.Log("did not find collector pods")
			return false
		}

		var pods v1.PodList
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(list.UnstructuredContent(), &pods)
		require.NoError(t, err, "failed to convert unstructured to podList")

		for _, pod := range pods.Items {
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
				for _, cs := range pod.Status.ContainerStatuses {
					restartCount := cs.RestartCount
					if restartCount > 0 && cs.LastTerminationState.Terminated != nil {
						t.Logf("restart count = %d for container %s in pod %s, last terminated reason: %s", restartCount, cs.Name, pod.Name, cs.LastTerminationState.Terminated.Reason)
						t.Logf("termination message: %s", cs.LastTerminationState.Terminated.Message)
					}
				}
			}
		}
		if podsNotReady == 0 {
			t.Logf("collector pods are ready")
			return true
		}
		return false
	}, time.Duration(podTimeoutMinutes)*time.Minute, 2*time.Second,
		"collector pods were not ready within %d minutes", podTimeoutMinutes)
}
