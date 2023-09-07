// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8stest // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8stest"

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"text/template"
	"time"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

func CreateCollectorObjects(t *testing.T, client *dynamic.DynamicClient, testID string) []*unstructured.Unstructured {
	manifestsDir := filepath.Join(".", "testdata", "e2e", "collector")
	manifestFiles, err := os.ReadDir(manifestsDir)
	require.NoErrorf(t, err, "failed to read collector manifests directory %s", manifestsDir)
	host := HostEndpoint(t)
	var podNamespace string
	var podLabels map[string]any
	createdObjs := make([]*unstructured.Unstructured, 0, len(manifestFiles))
	for _, manifestFile := range manifestFiles {
		tmpl := template.Must(template.New(manifestFile.Name()).ParseFiles(filepath.Join(manifestsDir, manifestFile.Name())))
		manifest := &bytes.Buffer{}
		require.NoError(t, tmpl.Execute(manifest, map[string]string{
			"Name":         "otelcol-" + testID,
			"HostEndpoint": host,
		}))
		obj, err := CreateObject(client, manifest.Bytes())
		require.NoErrorf(t, err, "failed to create collector object from manifest %s", manifestFile.Name())
		if obj.GetKind() == "Deployment" {
			podNamespace = obj.GetNamespace()
			selector := obj.Object["spec"].(map[string]any)["selector"]
			podLabels = selector.(map[string]any)["matchLabels"].(map[string]any)
		}
		createdObjs = append(createdObjs, obj)
	}

	WaitForCollectorToStart(t, client, podNamespace, podLabels)

	return createdObjs
}

func WaitForCollectorToStart(t *testing.T, client *dynamic.DynamicClient, podNamespace string, podLabels map[string]any) {
	podGVR := schema.GroupVersionResource{Version: "v1", Resource: "pods"}
	listOptions := metav1.ListOptions{LabelSelector: SelectorFromMap(podLabels).String()}
	podTimeoutMinutes := 3
	var podPhase string
	require.Eventually(t, func() bool {
		list, err := client.Resource(podGVR).Namespace(podNamespace).List(context.Background(), listOptions)
		require.NoError(t, err, "failed to list collector pods")
		if len(list.Items) == 0 {
			return false
		}
		podPhase = list.Items[0].Object["status"].(map[string]interface{})["phase"].(string)
		return podPhase == "Running"
	}, time.Duration(podTimeoutMinutes)*time.Minute, 50*time.Millisecond,
		"collector pod haven't started within %d minutes, latest pod phase is %s", podTimeoutMinutes, podPhase)
}
