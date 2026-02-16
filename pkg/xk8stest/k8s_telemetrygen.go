// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xk8stest // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest"

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
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
	const (
		baseTimeout     = 3 * time.Minute
		extendedTimeout = 8 * time.Minute
		pollInterval    = 200 * time.Millisecond
	)

	listOptions := metav1.ListOptions{LabelSelector: SelectorFromMap(podLabels).String()}
	var lastPodStatus string
	var slowImagePull bool

	cond := func() bool {
		running, slowPull, status := telemetrygenPodsStatus(t, client, podNamespace, listOptions)
		lastPodStatus = status
		if slowPull {
			slowImagePull = true
		}
		return running
	}

	if assert.Eventuallyf(t, cond, baseTimeout, pollInterval,
		"telemetrygen pods for Workload [%s] datatype [%s] haven't started in namespace %s, last observed status:\n%s",
		workload, dataType, podNamespace, lastPodStatus) {
		return
	}

	if slowImagePull && assert.Eventuallyf(t, cond, extendedTimeout-baseTimeout, pollInterval,
		"telemetrygen pods (slow image pull) for Workload [%s] datatype [%s] haven't started in namespace %s, last observed status:\n%s",
		workload, dataType, podNamespace, lastPodStatus) {
		return
	}

	require.Failf(t, "telemetrygen startup timeout",
		"telemetrygen pods for Workload [%s] datatype [%s] haven't started in namespace %s, last observed status:\n%s",
		workload, dataType, podNamespace, lastPodStatus)
}

func telemetrygenPodsStatus(t *testing.T, client *K8sClient, podNamespace string, listOptions metav1.ListOptions) (bool, bool, string) {
	podGVR := schema.GroupVersionResource{Version: "v1", Resource: "pods"}
	list, err := client.DynamicClient.Resource(podGVR).Namespace(podNamespace).List(t.Context(), listOptions)
	require.NoError(t, err, "failed to list collector pods")
	if len(list.Items) == 0 {
		return false, false, "no pods found yet"
	}

	var pods v1.PodList
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(list.UnstructuredContent(), &pods)
	require.NoError(t, err, "failed to convert telemetrygen pods")

	running := false
	slowPull := false
	var b strings.Builder
	for i := range pods.Items {
		pod := &pods.Items[i]
		if pod.Status.Phase == v1.PodRunning {
			running = true
		}
		fmt.Fprintf(&b, "pod=%s phase=%s", pod.Name, pod.Status.Phase)
		if msg := podWaitMessage(pod); msg != "" {
			fmt.Fprintf(&b, " (%s)", msg)
			if strings.Contains(msg, "ImagePull") {
				slowPull = true
			}
		}
		b.WriteString("\n")
	}
	return running, slowPull, strings.TrimSpace(b.String())
}

func podWaitMessage(pod *v1.Pod) string {
	var messages []string
	for i := range pod.Status.ContainerStatuses {
		cs := &pod.Status.ContainerStatuses[i]
		switch {
		case cs.State.Waiting != nil:
			msg := cs.State.Waiting.Reason
			if cs.State.Waiting.Message != "" {
				msg = fmt.Sprintf("%s - %s", msg, cs.State.Waiting.Message)
			}
			messages = append(messages, fmt.Sprintf("%s waiting: %s", cs.Name, msg))
		case cs.State.Terminated != nil && cs.State.Terminated.ExitCode != 0:
			msg := cs.State.Terminated.Reason
			if cs.State.Terminated.Message != "" {
				msg = fmt.Sprintf("%s - %s", msg, cs.State.Terminated.Message)
			}
			messages = append(messages, fmt.Sprintf("%s terminated: %s", cs.Name, msg))
		}
	}
	if len(messages) == 0 {
		for _, cond := range pod.Status.Conditions {
			if cond.Status != v1.ConditionTrue {
				messages = append(messages, fmt.Sprintf("condition %s=%s (%s)", cond.Type, cond.Status, cond.Reason))
			}
		}
	}
	return strings.Join(messages, "; ")
}
