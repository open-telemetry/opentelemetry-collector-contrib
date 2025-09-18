// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xk8stest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"

import (
	"bytes"
	"fmt"
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

// getPodLabelSelectors returns labels used to select pods created by the workload.
// - Deployment/StatefulSet/DaemonSet: spec.selector.matchLabels (fallback to template.metadata.labels)
// - Job: spec.template.metadata.labels
// - CronJob: spec.jobTemplate.spec.template.metadata.labels
func getPodLabelSelectors(obj *unstructured.Unstructured) (map[string]any, error) {
	o := obj.Object
	spec, ok := o["spec"].(map[string]any)
	if !ok || spec == nil {
		return nil, fmt.Errorf("%s/%s missing spec", obj.GetKind(), obj.GetName())
	}

	switch obj.GetKind() {
	case "Deployment", "StatefulSet", "DaemonSet":
		if sel, ok := spec["selector"].(map[string]any); ok && sel != nil {
			if ml, ok := sel["matchLabels"].(map[string]any); ok && ml != nil {
				return ml, nil
			}
		}
		// fallback — uncommon but robust
		if tmpl, ok := spec["template"].(map[string]any); ok && tmpl != nil {
			if meta, ok := tmpl["metadata"].(map[string]any); ok && meta != nil {
				if ml, ok := meta["labels"].(map[string]any); ok && ml != nil {
					return ml, nil
				}
			}
		}
		return nil, fmt.Errorf("%s/%s missing selector.matchLabels and template.metadata.labels", obj.GetKind(), obj.GetName())

	case "Job":
		if tmpl, ok := spec["template"].(map[string]any); ok && tmpl != nil {
			if meta, ok := tmpl["metadata"].(map[string]any); ok && meta != nil {
				if ml, ok := meta["labels"].(map[string]any); ok && ml != nil {
					return ml, nil
				}
			}
		}
		// last resort if API server already defaulted it
		if sel, ok := spec["selector"].(map[string]any); ok && sel != nil {
			if ml, ok := sel["matchLabels"].(map[string]any); ok && ml != nil {
				return ml, nil
			}
		}
		return nil, fmt.Errorf("Job/%s missing template.metadata.labels (and selector.matchLabels)", obj.GetName())

	case "CronJob":
		jt, ok := spec["jobTemplate"].(map[string]any)
		if !ok || jt == nil {
			return nil, fmt.Errorf("CronJob/%s missing spec.jobTemplate", obj.GetName())
		}
		jts, ok := jt["spec"].(map[string]any)
		if !ok || jts == nil {
			return nil, fmt.Errorf("CronJob/%s missing spec.jobTemplate.spec", obj.GetName())
		}
		tmpl, ok := jts["template"].(map[string]any)
		if !ok || tmpl == nil {
			return nil, fmt.Errorf("CronJob/%s missing spec.jobTemplate.spec.template", obj.GetName())
		}
		meta, ok := tmpl["metadata"].(map[string]any)
		if !ok || meta == nil {
			return nil, fmt.Errorf("CronJob/%s missing spec.jobTemplate.spec.template.metadata", obj.GetName())
		}
		ml, ok := meta["labels"].(map[string]any)
		if !ok || ml == nil {
			return nil, fmt.Errorf("CronJob/%s missing spec.jobTemplate.spec.template.metadata.labels", obj.GetName())
		}
		return ml, nil

	default:
		return nil, fmt.Errorf("unsupported kind %q", obj.GetKind())
	}
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

			podLabels, err := getPodLabelSelectors(obj)
			require.NoErrorf(t, err, "failed to extract pod label selectors for %s %s", obj.GetKind(), obj.GetName())

			telemetrygenObjInfos = append(telemetrygenObjInfos, &TelemetrygenObjInfo{
				Namespace:         obj.GetNamespace(),
				PodLabelSelectors: podLabels,
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
		list, err := client.DynamicClient.Resource(podGVR).Namespace(podNamespace).List(t.Context(), listOptions)
		require.NoError(t, err, "failed to list collector pods")
		if len(list.Items) == 0 {
			return false
		}
		podPhase = list.Items[0].Object["status"].(map[string]any)["phase"].(string)
		return podPhase == "Running"
	}, time.Duration(podTimeoutMinutes)*time.Minute, 50*time.Millisecond,
		"telemetrygen pod of Workload [%s] in datatype [%s] haven't started within %d minutes, latest pod phase is %s", workload, dataType, podTimeoutMinutes, podPhase)
}
