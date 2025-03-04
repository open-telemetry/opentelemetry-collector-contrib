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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type TelemetrygenObjInfo struct {
	Namespace         string
	PodLabelSelectors map[string]any
	DataType          string
	Workload          string
}

type TelemetrygenCreateOpts struct {
	TestID       string
	ManifestsDir string
	OtlpEndpoint string
	DataTypes    []string
}

func CreateTelemetryGenObjects(t *testing.T, client *K8sClient, createOpts *TelemetrygenCreateOpts) ([]*unstructured.Unstructured, []*TelemetrygenObjInfo) {
	telemetrygenObjInfos := make([]*TelemetrygenObjInfo, 0)
	manifestFiles, err := os.ReadDir(createOpts.ManifestsDir)
	require.NoErrorf(t, err, "failed to read telemetrygen manifests directory %s", createOpts.ManifestsDir)
	createdObjs := make([]*unstructured.Unstructured, 0, len(manifestFiles))
	for _, manifestFile := range manifestFiles {
		tmpl := template.Must(template.New(manifestFile.Name()).ParseFiles(filepath.Join(createOpts.ManifestsDir, manifestFile.Name())))
		for _, dataType := range createOpts.DataTypes {
			manifest := &bytes.Buffer{}
			require.NoError(t, tmpl.Execute(manifest, map[string]string{
				"Name":         "telemetrygen-" + createOpts.TestID,
				"DataType":     dataType,
				"OTLPEndpoint": createOpts.OtlpEndpoint,
				"TestID":       createOpts.TestID,
			}))
			obj, err := CreateObject(client, manifest.Bytes())
			require.NoErrorf(t, err, "failed to create telemetrygen object from manifest %s", manifestFile.Name())
			selector := obj.Object["spec"].(map[string]any)["selector"]
			telemetrygenObjInfos = append(telemetrygenObjInfos, &TelemetrygenObjInfo{
				Namespace:         obj.GetNamespace(),
				PodLabelSelectors: selector.(map[string]any)["matchLabels"].(map[string]any),
				DataType:          dataType,
				Workload:          obj.GetKind(),
			})
			createdObjs = append(createdObjs, obj)
		}
	}
	return createdObjs, telemetrygenObjInfos
}

func WaitForTelemetryGenToStart(t *testing.T, client *K8sClient, podNamespace string, podLabels map[string]any, workload, dataType string) {
	podGVR := schema.GroupVersionResource{Version: "v1", Resource: "pods"}
	listOptions := metav1.ListOptions{LabelSelector: SelectorFromMap(podLabels).String()}
	podTimeoutMinutes := 3
	var podPhase string
	require.Eventually(t, func() bool {
		list, err := client.DynamicClient.Resource(podGVR).Namespace(podNamespace).List(context.Background(), listOptions)
		require.NoError(t, err, "failed to list collector pods")
		if len(list.Items) == 0 {
			return false
		}
		podPhase = list.Items[0].Object["status"].(map[string]any)["phase"].(string)
		return podPhase == "Running"
	}, time.Duration(podTimeoutMinutes)*time.Minute, 50*time.Millisecond,
		"telemetrygen pod of Workload [%s] in datatype [%s] haven't started within %d minutes, latest pod phase is %s", workload, dataType, podTimeoutMinutes, podPhase)
}
