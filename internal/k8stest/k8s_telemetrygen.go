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

type TelemetrygenObjInfo struct {
	Namespace         string
	PodLabelSelectors map[string]any
	DataType          string
	Workload          string
}

func CreateTelemetryGenObjects(t *testing.T, client *dynamic.DynamicClient, testID string) ([]*unstructured.Unstructured, []*TelemetrygenObjInfo) {
	telemetrygenObjInfos := make([]*TelemetrygenObjInfo, 0)
	manifestsDir := filepath.Join(".", "testdata", "e2e", "telemetrygen")
	manifestFiles, err := os.ReadDir(manifestsDir)
	require.NoErrorf(t, err, "failed to read telemetrygen manifests directory %s", manifestsDir)
	createdObjs := make([]*unstructured.Unstructured, 0, len(manifestFiles))
	for _, manifestFile := range manifestFiles {
		tmpl := template.Must(template.New(manifestFile.Name()).ParseFiles(filepath.Join(manifestsDir, manifestFile.Name())))
		for _, dataType := range []string{"metrics", "logs", "traces"} {
			manifest := &bytes.Buffer{}
			require.NoError(t, tmpl.Execute(manifest, map[string]string{
				"Name":         "telemetrygen-" + testID,
				"DataType":     dataType,
				"OTLPEndpoint": "otelcol-" + testID + ":4317",
			}))
			obj, err := CreateObject(client, manifest.Bytes())
			require.NoErrorf(t, err, "failed to create telemetrygen object from manifest %s", manifestFile.Name())
			selector := obj.Object["spec"].(map[string]any)["selector"]
			telemetrygenObjInfos = append(telemetrygenObjInfos, &TelemetrygenObjInfo{
				Namespace:         "default",
				PodLabelSelectors: selector.(map[string]any)["matchLabels"].(map[string]any),
				DataType:          dataType,
				Workload:          obj.GetKind(),
			})
			createdObjs = append(createdObjs, obj)
		}
	}
	return createdObjs, telemetrygenObjInfos
}

func WaitForTelemetryGenToStart(t *testing.T, client *dynamic.DynamicClient, podNamespace string, podLabels map[string]any, workload, dataType string) {
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
		podPhase = list.Items[0].Object["status"].(map[string]any)["phase"].(string)
		return podPhase == "Running"
	}, time.Duration(podTimeoutMinutes)*time.Minute, 50*time.Millisecond,
		"telemetrygen pod of Workload [%s] in datatype [%s] haven't started within %d minutes, latest pod phase is %s", workload, dataType, podTimeoutMinutes, podPhase)
}
