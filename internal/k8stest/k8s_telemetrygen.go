// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8stest // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8stest"

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"text/template"
	"time"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
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

func WaitForTelemetryGenToStart(t *testing.T, client *dynamic.DynamicClient, kubeCli *kubernetes.Clientset, podNamespace string, podLabels map[string]any, workload, dataType string) {
	podGVR := schema.GroupVersionResource{Version: "v1", Resource: "pods"}
	listOptions := metav1.ListOptions{LabelSelector: SelectorFromMap(podLabels).String()}
	podTimeoutMinutes := 3
	var podPhase string
	var podStatus map[string]interface{}
	require.Eventually(t, func() bool {
		list, err := client.Resource(podGVR).Namespace(podNamespace).List(context.Background(), listOptions)
		require.NoError(t, err, "failed to list collector pods")
		if len(list.Items) == 0 {
			fmt.Printf("list no resources: %v\n")
			return false
		}
		podName := list.Items[0].Object["metadata"].(map[string]interface{})["name"].(string)
		fmt.Printf("check pod %v\n", podName)

		podPhase = list.Items[0].Object["status"].(map[string]interface{})["phase"].(string)
		fmt.Printf("check pod %v phase %s\n", podName, podPhase)

		containerStatus := list.Items[0].Object["status"].(map[string]interface{})["containerStatuses"]
		fmt.Printf("check pod %v containerStatus %v\n", podName, containerStatus)

		if podPhase == "Failed" {
			namespace := "default"
			logOptions := &v1.PodLogOptions{
				Container: "telemetrygen",
			}
			req := kubeCli.CoreV1().Pods(namespace).GetLogs(podName, logOptions)

			result, err := req.Stream(context.Background())
			if err != nil {
			}
			defer result.Close()

			buf := make([]byte, 1024)
			for {
				n, err := result.Read(buf)
				if err != nil {
					panic(err.Error())
				}
				fmt.Println(string(buf[:n]))
			}
		}

		return podPhase == "Running"
	}, time.Duration(podTimeoutMinutes)*time.Minute, 20*time.Second,
		"telemetrygen pod of Workload [%s] in datatype [%s] haven't started within %d minutes, latest pod status is %v", workload, dataType, podTimeoutMinutes, podStatus)
}
