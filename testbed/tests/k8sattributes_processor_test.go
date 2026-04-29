// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tests

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/testbed/testbed"
)

// ── Test coverage reference ───────────────────────────────────────────────────
//
// # TestMetricK8sAttributesProcessor (basic)
//
// Cluster: 100 nodes, N namespaces
// Each Deployment owns exactly 1 ReplicaSet which owns exactly 1 Pod.
//
//	Scale  | Nodes | Namespaces | Deployments | ReplicaSets | Pods
//	-------|-------|------------|-------------|-------------|------
//	   110 |   100 |        110 |         110 |         110 |   110
//	 1 000 |   100 |      1 000 |       1 000 |       1 000 | 1 000
//	 5 000 |   100 |      5 000 |       5 000 |       5 000 | 5 000
//
// # TestMetricK8sAttributesProcessorExtended (extended)
//
// Cluster: 100 nodes, N namespaces in total with 4 namespaces being targeted for the workloads(1 per workload type).
// DaemonSet pods are pinned to a single node via nodeSelector (1 pod per DaemonSet).
//
//	Namespace        | Workload type | Ownership chain
//	-----------------|---------------|--------------------------------------
//	namespace-000000 | Deployment    | Deployment → ReplicaSet → Pod
//	namespace-000001 | StatefulSet   | StatefulSet → Pod
//	namespace-000002 | DaemonSet     | DaemonSet (nodeSelector) → Pod
//	namespace-000003 | CronJob       | CronJob → Job → Pod
//
//	Scale  | Nodes | NS | Deployments | RSes  | StatefulSets | DaemonSets | CronJobs | Jobs  | Total Pods
//	-------|-------|----|-------------|-------|--------------|------------|----------|-------|------------
//	   110 |   100 |  110 |         110 |   110 |          110 |        110 |      110 |   110 |        440
//	 1 000 |   100 |  110 |       1 000 | 1 000 |        1 000 |      1 000 |    1 000 | 1 000 |      4 000
//	 5 000 |   100 |  110 |       5 000 | 5 000 |        5 000 |      5 000 |    5 000 | 5 000 |     20 000

// Create 100 fake nodes for extra cluster load using kwokctl scale
const numNodes = 100

// numMetricBatches is how many metric batches to send and assert on in the k8sattributes test.
const numMetricBatches = 10

// kwokNamespaceResourceYAML is a KwokctlResource for "kwokctl scale namespace"
const kwokNamespaceResourceYAML = `
apiVersion: config.kwok.x-k8s.io/v1alpha1
kind: KwokctlResource
metadata:
  name: namespace
parameters: {}
template: |-
  apiVersion: v1
  kind: Namespace
  metadata:
    name: {{ Name }}
`

// kwokDeploymentResourceYAML is a KwokctlResource for "kwokctl scale deployment"
const kwokDeploymentResourceYAML = `
apiVersion: config.kwok.x-k8s.io/v1alpha1
kind: KwokctlResource
metadata:
  name: deployment
parameters:
  replicas: 1
  initContainers: []
  containers:
  - name: container-0
    image: registry.k8s.io/pause:3.9
  hostNetwork: false
  nodeName: ""
template: |-
  apiVersion: apps/v1
  kind: Deployment
  metadata:
    name: {{ Name }}
    namespace: {{ or Namespace "namespace-000000" }}
  spec:
    replicas: {{ .replicas }}
    selector:
      matchLabels:
        app: {{ Name }}
    template:
      metadata:
        labels:
          app: {{ Name }}
      spec:
        containers:
        {{ range $index, $container := .containers }}
        - name: {{ $container.name }}
          image: {{ $container.image }}
        {{ end }}
        initContainers:
        {{ range $index, $container := .initContainers }}
        - name: {{ $container.name }}
          image: {{ $container.image }}
        {{ end }}
        hostNetwork: {{ .hostNetwork }}
        nodeName: {{ .nodeName }}
`

// kwokStatefulSetResourceYAML is a KwokctlResource for "kwokctl scale statefulset".
// Pods land in namespace-000001 and are created by the kube-controller-manager StatefulSet
// controller, giving each pod a direct OwnerReference to its StatefulSet.
const kwokStatefulSetResourceYAML = `
apiVersion: config.kwok.x-k8s.io/v1alpha1
kind: KwokctlResource
metadata:
  name: statefulset
parameters:
  replicas: 1
  containers:
  - name: container-0
    image: registry.k8s.io/pause:3.9
template: |-
  apiVersion: apps/v1
  kind: StatefulSet
  metadata:
    name: {{ Name }}
    namespace: namespace-000001
  spec:
    replicas: {{ .replicas }}
    selector:
      matchLabels:
        app: {{ Name }}
    serviceName: "{{ Name }}-svc"
    template:
      metadata:
        labels:
          app: {{ Name }}
      spec:
        containers:
        {{ range $index, $container := .containers }}
        - name: {{ $container.name }}
          image: {{ $container.image }}
        {{ end }}
`

// kwokDaemonSetResourceYAML is a KwokctlResource for "kwokctl scale daemonset".
// A nodeSelector pins each DaemonSet pod to kwok-node-000000, so that even with 100
// nodes the DaemonSet controller creates exactly one pod per DaemonSet (1:1 ratio),
// giving that pod a direct OwnerReference to its DaemonSet.
// Pods land in namespace-000002.
const kwokDaemonSetResourceYAML = `
apiVersion: config.kwok.x-k8s.io/v1alpha1
kind: KwokctlResource
metadata:
  name: daemonset
parameters:
  containers:
  - name: container-0
    image: registry.k8s.io/pause:3.9
template: |-
  apiVersion: apps/v1
  kind: DaemonSet
  metadata:
    name: {{ Name }}
    namespace: namespace-000002
  spec:
    selector:
      matchLabels:
        app: {{ Name }}
    template:
      metadata:
        labels:
          app: {{ Name }}
      spec:
        nodeSelector:
          kubernetes.io/hostname: node-000000
        containers:
        {{ range $index, $container := .containers }}
        - name: {{ $container.name }}
          image: {{ $container.image }}
        {{ end }}
`

// kwokCronJobResourceYAML is a KwokctlResource for "kwokctl scale cronjob".
// Each CronJob fires every minute; the kube-controller-manager CronJob controller
// creates a Job with an OwnerReference to the CronJob, and the Job controller then
// creates a Pod with an OwnerReference to the Job.  This establishes the full
// Pod → Job → CronJob ownership chain that the k8sattributes processor traverses.
// Pods land in namespace-000003.
const kwokCronJobResourceYAML = `
apiVersion: config.kwok.x-k8s.io/v1alpha1
kind: KwokctlResource
metadata:
  name: cronjob
parameters:
  containers:
  - name: container-0
    image: registry.k8s.io/pause:3.9
template: |-
  apiVersion: batch/v1
  kind: CronJob
  metadata:
    name: {{ Name }}
    namespace: namespace-000003
  spec:
    schedule: "*/1 * * * *"
    concurrencyPolicy: Forbid
    jobTemplate:
      spec:
        template:
          spec:
            restartPolicy: Never
            containers:
            {{ range $index, $container := .containers }}
            - name: {{ $container.name }}
              image: {{ $container.image }}
            {{ end }}
`

// sample metrics consumed by the test pipeline
// k8s.pod.uid is added at test time from the KWOK-created pod so the processor can associate by pod UID.
var mockedConsumedMetricsForK8s = func() pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "test-service")
	m := rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	m.SetName("metric-name")
	m.SetDescription("metric-description")
	m.SetUnit("metric-unit")
	m.SetEmptyGauge().DataPoints().AppendEmpty().SetIntValue(0)
	return md
}()

type k8sAttributesProcessorTestCase struct {
	name                  string
	k8sAttributesConfig   string
	expectedResourceAttrs map[string]any // assert these resource attributes are present (nil = presence only)
	numWorkloads          int            // workload instances to create (basic: one pod per deployment; extended: instances per ownership type)
}

func getK8sAttributesProcessorBasicTestCases() []k8sAttributesProcessorTestCase {
	// __CONTEXT__ is replaced at runtime with the current context from the kubeconfig.
	kwokConfig := `
  k8s_attributes:
    auth_type: "kubeConfig"
    context: "__CONTEXT__"
    extract:
      metadata:
        - k8s.pod.name
        - k8s.pod.uid
        - k8s.namespace.name
        - k8s.node.name
        - k8s.deployment.name
    pod_association:
      - sources:
          - from: resource_attribute
            name: k8s.pod.uid
`
	expectedAttrs := map[string]any{
		"k8s.pod.name":        nil,
		"k8s.namespace.name":  nil,
		"k8s.pod.uid":         nil,
		"k8s.node.name":       nil,
		"k8s.deployment.name": nil,
	}
	return []k8sAttributesProcessorTestCase{
		{
			name:                  "110_workload_cluster",
			k8sAttributesConfig:   kwokConfig,
			expectedResourceAttrs: expectedAttrs,
			numWorkloads:          110,
		},
		{
			name:                  "1K_workload_cluster",
			k8sAttributesConfig:   kwokConfig,
			expectedResourceAttrs: expectedAttrs,
			numWorkloads:          1000,
		},
		{
			name:                  "5K_workload_cluster",
			k8sAttributesConfig:   kwokConfig,
			expectedResourceAttrs: expectedAttrs,
			numWorkloads:          5000,
		},
	}
}

func getK8sAttributesProcessorExtendedTestCases() []k8sAttributesProcessorTestCase {
	// __CONTEXT__ is replaced at runtime with the current context from the kubeconfig.
	kwokConfig := `
  k8s_attributes:
    auth_type: "kubeConfig"
    context: "__CONTEXT__"
    extract:
      metadata:
        - k8s.pod.name
        - k8s.pod.start_time
        - k8s.pod.uid
        - k8s.namespace.name
        - k8s.deployment.name
        - k8s.deployment.uid
        - k8s.replicaset.name
        - k8s.replicaset.uid
        - k8s.statefulset.name
        - k8s.statefulset.uid
        - k8s.daemonset.name
        - k8s.daemonset.uid
        - k8s.cronjob.name
        - k8s.cronjob.uid
        - k8s.job.name
        - k8s.job.uid
        - k8s.node.name
        - k8s.cluster.uid
        - container.image.name
        - container.image.tag
    pod_association:
      - sources:
          - from: resource_attribute
            name: k8s.pod.uid
`
	// Attributes that must appear on every enriched batch regardless of ownership chain.
	// Chain-specific attrs (deployment, statefulset, daemonset, cronjob/job) are validated
	// separately in runTestbedWithK8sConfigExtended via the podTypes chainAttrs.
	// container.id and container.image.repo_digests are omitted: KWOK runs no real container
	// runtime, so those fields are never populated in the pod status.
	expectedAttrs := map[string]any{
		"k8s.pod.name":         nil,
		"k8s.pod.start_time":   nil,
		"k8s.pod.uid":          nil,
		"k8s.namespace.name":   nil,
		"k8s.node.name":        nil,
		"k8s.cluster.uid":      nil,
		"container.image.name": nil,
		"container.image.tag":  nil,
	}
	return []k8sAttributesProcessorTestCase{
		{
			name:                  "110_workload_cluster_extended",
			k8sAttributesConfig:   kwokConfig,
			expectedResourceAttrs: expectedAttrs,
			numWorkloads:          110,
		},
		{
			name:                  "1K_workload_cluster_extended",
			k8sAttributesConfig:   kwokConfig,
			expectedResourceAttrs: expectedAttrs,
			numWorkloads:          1000,
		},
		{
			name:                  "5K_workload_cluster_extended",
			k8sAttributesConfig:   kwokConfig,
			expectedResourceAttrs: expectedAttrs,
			numWorkloads:          5000,
		},
	}
}

func skipIfKwokUnavailable(t *testing.T) {
	if os.Getenv("SKIP_KWOK_TESTS") == "1" {
		t.Skip("Skipping KWOK test: SKIP_KWOK_TESTS=1")
	}
	if _, err := exec.LookPath("kwokctl"); err != nil {
		t.Skipf("Skipping KWOK test: kwokctl not found in PATH (install from https://kwok.sigs.k8s.io/)")
	}
}

// logKWOKClusterState logs namespaces, deployments, pods, and control-plane component logs in the cluster for debugging when pod count is 0.
func logKWOKClusterState(ctx context.Context, t *testing.T, clientset *kubernetes.Clientset, targetNS, clusterName string) {
	t.Helper()
	// List all namespaces
	nsList, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Logf("[kwok debug] list namespaces: %v", err)
		return
	}
	var nsNames []string
	for i := range nsList.Items {
		nsNames = append(nsNames, nsList.Items[i].Name)
	}
	t.Logf("[kwok debug] namespaces (%d): %v", len(nsList.Items), nsNames)
	// List deployments in target namespace
	deployList, err := clientset.AppsV1().Deployments(targetNS).List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Logf("[kwok debug] list deployments in %q: %v", targetNS, err)
	} else {
		var deployNames []string
		for i := range deployList.Items {
			deployNames = append(deployNames, deployList.Items[i].Name)
		}
		t.Logf("[kwok debug] deployments in %q (%d): %v", targetNS, len(deployList.Items), deployNames)
	}
	// List ReplicaSets in target namespace and log first ReplicaSet status (for debugging deployment controller).
	rsList, err := clientset.AppsV1().ReplicaSets(targetNS).List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Logf("[kwok debug] list replicasets in %q: %v", targetNS, err)
	} else {
		var rsNames []string
		for i := range rsList.Items {
			rsNames = append(rsNames, rsList.Items[i].Name)
		}
		t.Logf("[kwok debug] replicasets in %q (%d): %v", targetNS, len(rsList.Items), rsNames)
		if len(rsList.Items) > 0 {
			first := &rsList.Items[0]
			st := first.Status
			t.Logf("[kwok debug] first replicaset %q status: replicas=%d, ready=%d, available=%d, fullyLabeled=%d, observedGeneration=%d",
				first.Name, st.Replicas, st.ReadyReplicas, st.AvailableReplicas, st.FullyLabeledReplicas, st.ObservedGeneration)
		}
	}
	// List pods in target namespace
	podList, err := clientset.CoreV1().Pods(targetNS).List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Logf("[kwok debug] list pods in %q: %v", targetNS, err)
	} else {
		t.Logf("[kwok debug] pods in %q: %d", targetNS, len(podList.Items))
	}
	// List deployments and pods across all namespaces (counts per namespace)
	for i := range nsList.Items {
		name := nsList.Items[i].Name
		d, _ := clientset.AppsV1().Deployments(name).List(ctx, metav1.ListOptions{})
		p, _ := clientset.CoreV1().Pods(name).List(ctx, metav1.ListOptions{})
		if len(d.Items) > 0 || len(p.Items) > 0 {
			t.Logf("[kwok debug] namespace %q: %d deployments, %d pods", name, len(d.Items), len(p.Items))
		}
	}
	// Log all cluster events (per namespace) when no pods were created, to help debug reconciliation.
	const maxEventsPerNS = 50
	for i := range nsList.Items {
		nsName := nsList.Items[i].Name
		evList, err := clientset.CoreV1().Events(nsName).List(ctx, metav1.ListOptions{Limit: maxEventsPerNS})
		if err != nil {
			t.Logf("[kwok debug] list events in %q: %v", nsName, err)
			continue
		}
		if len(evList.Items) == 0 {
			continue
		}
		t.Logf("[kwok debug] events in %q (%d):", nsName, len(evList.Items))
		for j := range evList.Items {
			ev := &evList.Items[j]
			t.Logf("[kwok debug]   %s %s %s/%s: %s", ev.LastTimestamp.Format("15:04:05"), ev.Reason, ev.InvolvedObject.Kind, ev.InvolvedObject.Name, ev.Message)
		}
	}
	// Control plane logs (binaries: use kwokctl logs).
	for _, component := range []string{"kwok-controller", "kube-apiserver", "kube-controller-manager", "kube-scheduler"} {
		// #nosec G204 -- clusterName is test-controlled
		cmd := exec.Command("kwokctl", "logs", component, "--name", clusterName)
		out, err := cmd.CombinedOutput()
		text := strings.TrimSpace(string(out))
		switch {
		case err != nil:
			t.Logf("[kwok debug] kwokctl logs %s: %v\n%s", component, err, text)
		case text != "":
			t.Logf("[kwok debug] kwokctl logs %s:\n%s", component, text)
		default:
			t.Logf("[kwok debug] kwokctl logs %s: (no output)", component)
		}
	}
}

// kwokCluster holds the outputs of createKWOKCluster that both setup helpers need.
type kwokCluster struct {
	Name           string
	KubeconfigPath string
	Clientset      *kubernetes.Clientset
	Cleanup        func()
}

// createKWOKCluster starts a kwokctl binary-runtime cluster, scales it to numNodes nodes
// and numNamespaces namespaces, writes the kubeconfig to a temp file, constructs a
// Kubernetes client, and returns the assembled kwokCluster.
// The sanitized t.Name() to form the cluster name (capped at 50
// characters).  The returned Cleanup must be called when the cluster is no longer needed.
func createKWOKCluster(t *testing.T, numNamespaces int) kwokCluster {
	t.Helper()

	if verOut, err := exec.Command("kwokctl", "--version").CombinedOutput(); err != nil {
		t.Logf("[kwok] kwokctl version: (failed to get: %v)", err)
	} else {
		t.Logf("[kwok] kwokctl version:\n%s", strings.TrimSpace(string(verOut)))
	}

	clusterName := strings.ReplaceAll(t.Name(), "/", "-")
	clusterName = strings.ReplaceAll(clusterName, " ", "-")
	if len(clusterName) > 50 {
		clusterName = clusterName[:50]
	}

	// #nosec G204 -- clusterName is test-controlled
	create := exec.Command("kwokctl", "create", "cluster", "--disable-qps-limits", "--runtime", "binary", "--name", clusterName)
	create.Dir = t.TempDir()
	out, err := create.CombinedOutput()
	require.NoError(t, err, "kwokctl create cluster: %s", out)

	var cleanupOnce sync.Once
	cleanup := func() {
		cleanupOnce.Do(func() {
			// #nosec G204
			del := exec.Command("kwokctl", "delete", "cluster", "--name", clusterName)
			_ = del.Run()
			assert.Eventually(t, func() bool {
				getClusters := exec.Command("kwokctl", "get", "clusters")
				clusterOut, clusterErr := getClusters.Output()
				return clusterErr != nil || !strings.Contains(string(clusterOut), clusterName)
			}, 30*time.Second, 500*time.Millisecond, "cluster %s should be removed", clusterName)
		})
	}

	// #nosec G204
	getConfig := exec.Command("kwokctl", "get", "kubeconfig", "--name", clusterName)
	getConfig.Dir = t.TempDir()
	out, err = getConfig.CombinedOutput()
	require.NoError(t, err, "kwokctl get kubeconfig: %s", out)

	tmpFile, err := os.CreateTemp(t.TempDir(), "kubeconfig-*.yaml")
	require.NoError(t, err)
	_, err = tmpFile.Write(out)
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())

	kubeconfigPath := tmpFile.Name()
	k8sCfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	require.NoError(t, err)
	clientset, err := kubernetes.NewForConfig(k8sCfg)
	require.NoError(t, err)

	// Scale to numNodes fake nodes.
	// #nosec G204 -- clusterName and numNodes are test-controlled
	scaleNodes := exec.Command("kwokctl", "scale", "node", "--replicas", fmt.Sprintf("%d", numNodes), "--name", clusterName)
	scaleNodes.Dir = t.TempDir()
	if nodeOut, nodeErr := scaleNodes.CombinedOutput(); nodeErr != nil {
		t.Logf("[kwok] kwokctl scale node (optional): %v\n%s", nodeErr, nodeOut)
	}

	// Scale to numNamespaces namespaces.
	kwokNSConfigPath := filepath.Join(t.TempDir(), "kwok-ns-resource.yaml")
	require.NoError(t, os.WriteFile(kwokNSConfigPath, []byte(strings.TrimSpace(kwokNamespaceResourceYAML)), 0o600))
	// #nosec G204
	scaleNS := exec.Command("kwokctl", "scale", "namespace",
		"--replicas", fmt.Sprintf("%d", numNamespaces),
		"--serial-length", "6",
		"--name", clusterName,
		"--config", kwokNSConfigPath)
	scaleNS.Dir = t.TempDir()
	if nsOut, nsErr := scaleNS.CombinedOutput(); nsErr != nil {
		t.Fatalf("kwokctl scale namespace: %v\n%s", nsErr, nsOut)
	}

	// Wait for all 4 namespaces to exist before returning so callers can immediately
	// issue workload scale commands without racing the namespace controller.
	ctx := t.Context()
	for i := range 4 {
		ns := fmt.Sprintf("namespace-%06d", i)
		assert.Eventually(t, func() bool {
			_, nsErr := clientset.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
			return nsErr == nil
		}, 2*time.Minute, 2*time.Second, "namespace %s never created", ns)
	}

	return kwokCluster{
		Name:           clusterName,
		KubeconfigPath: kubeconfigPath,
		Clientset:      clientset,
		Cleanup:        cleanup,
	}
}

// setupKWOKCluster creates a KWOK cluster with numWorkloads pods as part of their own Deployment (1 replica).
// See https://kwok.sigs.k8s.io/
func setupKWOKCluster(t *testing.T, numWorkloads int) (kubeconfigPath, podUID string, cleanup func()) {
	// createKWOKCluster scales to numNodes nodes and numWorkloads namespaces (one per deployment).
	kc := createKWOKCluster(t, numWorkloads)
	cleanup = kc.Cleanup
	kubeconfigPath = kc.KubeconfigPath
	clusterName := kc.Name
	clientset := kc.Clientset
	ctx := t.Context()

	// All deployments are placed in namespace-000000 by the deployment template.
	targetNS := "namespace-000000"

	kwokConfigPath := filepath.Join(t.TempDir(), "kwok-resources.yaml")
	require.NoError(t, os.WriteFile(kwokConfigPath, []byte(strings.TrimSpace(kwokDeploymentResourceYAML)), 0o600))

	// #nosec G204 -- clusterName, numWorkloads, kwokConfigPath are test-controlled, not user input
	scaleDeploy := exec.Command("kwokctl", "scale", "deployment",
		"--replicas", fmt.Sprintf("%d", numWorkloads),
		"--serial-length", "6",
		"--param", ".replicas=1",
		"--name", clusterName,
		"--config", kwokConfigPath)
	scaleDeploy.Dir = t.TempDir()
	if out, runErr := scaleDeploy.CombinedOutput(); runErr != nil {
		t.Fatalf("kwokctl scale deployment: %v\n%s", runErr, out)
	}

	// Wait until we have numWorkloads pods in namespace-000000 (all deployments go there)
	podWaitTimeout := min(3*time.Minute+time.Duration(numWorkloads/5)*time.Second, 15*time.Minute)
	var podCount int
	var debugLogged bool
	var attempt int
	// Minimum attempts before logging cluster state, to give the deployment controller time to create replicasets/pods.
	const minAttemptsBeforeDebugLog = 10
	assert.Eventually(t, func() bool {
		attempt++
		list, listErr := clientset.CoreV1().Pods(targetNS).List(ctx, metav1.ListOptions{})
		if listErr != nil {
			if !debugLogged && attempt >= minAttemptsBeforeDebugLog {
				debugLogged = true
				t.Logf("[kwok debug] list pods in %q failed after %d attempts: %v", targetNS, attempt, listErr)
				logKWOKClusterState(ctx, t, clientset, targetNS, clusterName)
			}
			return false
		}
		podCount = len(list.Items)
		if podCount == 0 && !debugLogged && attempt >= minAttemptsBeforeDebugLog {
			debugLogged = true
			t.Logf("[kwok debug] got 0 pods in %q after %d attempts, logging cluster state", targetNS, attempt)
			logKWOKClusterState(ctx, t, clientset, targetNS, clusterName)
		}
		return podCount >= numWorkloads
	}, podWaitTimeout, 1*time.Second, "timed out waiting for %d pods in namespace-000000 (got %d)", numWorkloads, podCount)

	// Get first pod UID in namespace-000000 for metric association
	list, listErr := clientset.CoreV1().Pods(targetNS).List(ctx, metav1.ListOptions{})
	if listErr == nil && len(list.Items) > 0 {
		podUID = string(list.Items[0].UID)
	}

	return kubeconfigPath, podUID, cleanup
}

// getKubeconfigCurrentContext returns the current context name from the kubeconfig at path, or "" on error.
func getKubeconfigCurrentContext(kubeconfigPath string) string {
	cmd := exec.Command("kubectl", "config", "current-context")
	cmd.Env = append(os.Environ(), "KUBECONFIG="+kubeconfigPath)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// runTestbedWithK8sConfig runs the testbed flow (child process collector, send metrics, assert).
// If kubeconfigPath is non-empty, KUBECONFIG is set for the collector process (e.g. for KWOK cluster).
// If podUID is non-empty, it is set as k8s.pod.uid on the sent metrics so the processor can associate them with the pod.
func runTestbedWithK8sConfig(t *testing.T, test *k8sAttributesProcessorTestCase, kubeconfigPath, podUID string) {
	sender := testbed.NewOTLPMetricDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t))
	receiver := testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t))

	resultDir, err := filepath.Abs(filepath.Join("results", t.Name()))
	require.NoError(t, err)

	opts := []testbed.ChildProcessOption{testbed.WithEnvVar("GOMAXPROCS", "2")}
	if kubeconfigPath != "" {
		opts = append(opts, testbed.WithEnvVar("KUBECONFIG", kubeconfigPath))
	}
	agentProc := testbed.NewChildProcessCollector(opts...)
	k8sConfigBody := test.k8sAttributesConfig
	if kubeconfigPath != "" {
		currentContext := getKubeconfigCurrentContext(kubeconfigPath)
		k8sConfigBody = strings.Replace(k8sConfigBody, "__CONTEXT__", currentContext, 1)
	}
	processors := []ProcessorNameAndConfigBody{
		{Name: "k8s_attributes", Body: k8sConfigBody},
	}
	configStr := createConfigYaml(t, sender, receiver, resultDir, processors, nil)
	configCleanup, err := agentProc.PrepareConfig(t, configStr)
	require.NoError(t, err)
	defer configCleanup()

	options := testbed.LoadOptions{DataItemsPerSecond: 10000, ItemsPerBatch: 10}
	dataProvider := testbed.NewPerfTestDataProvider(options)
	tc := testbed.NewTestCase(
		t,
		dataProvider,
		sender,
		receiver,
		agentProc,
		&testbed.PerfTestValidator{},
		performanceResultsSummary,
		testbed.WithResourceLimits(testbed.ResourceSpec{
			ExpectedMaxCPU: 200,
			ExpectedMaxRAM: 2000,
		}),
	)
	defer tc.Stop()

	tc.StartBackend()
	tc.StartAgent()
	defer tc.StopAgent()

	// Allow the k8sattributes processor's informer to sync before sending metrics.
	syncWait := min(15*time.Second+time.Duration(test.numWorkloads/100)*100*time.Millisecond, 60*time.Second)
	time.Sleep(syncWait)

	tc.EnableRecording()

	require.NoError(t, sender.Start())

	tc.MockBackend.ClearReceivedItems()
	startCounter := tc.MockBackend.DataItemsReceived()

	sender, ok := tc.LoadGenerator.(*testbed.ProviderSender).Sender.(testbed.MetricDataSender)
	require.True(t, ok, "unsupported metric sender")

	for i := range numMetricBatches {
		metricsToSend := pmetric.NewMetrics()
		mockedConsumedMetricsForK8s.CopyTo(metricsToSend)
		if podUID != "" {
			metricsToSend.ResourceMetrics().At(0).Resource().Attributes().PutStr("k8s.pod.uid", podUID)
		}
		require.NoError(t, sender.ConsumeMetrics(t.Context(), metricsToSend))
		tc.LoadGenerator.IncDataItemsSent()
		if i < numMetricBatches-1 {
			time.Sleep(250 * time.Millisecond)
		}
	}

	tc.WaitFor(func() bool { return tc.MockBackend.DataItemsReceived() == startCounter+uint64(numMetricBatches) },
		"datapoints received")

	received := tc.MockBackend.ReceivedMetrics
	require.Len(t, received, numMetricBatches, "expected %d metric batches", numMetricBatches)
	for i := range numMetricBatches {
		m := received[i]
		rm := m.ResourceMetrics()
		require.Equal(t, 1, rm.Len(), "batch %d", i)
		gotAttrs := rm.At(0).Resource().Attributes().AsRaw()
		if test.expectedResourceAttrs != nil {
			for k, v := range test.expectedResourceAttrs {
				require.Contains(t, gotAttrs, k, "batch %d: missing resource attribute %q", i, k)
				if v != nil {
					require.Equal(t, v, gotAttrs[k], "batch %d: resource attribute %q", i, k)
				}
			}
		}
	}
}

// extendedClusterPodUIDs holds one representative pod UID from each of the four
// ownership types created by setupKWOKClusterExtended.
type extendedClusterPodUIDs struct {
	Deployment  string
	StatefulSet string
	DaemonSet   string
	CronJob     string
}

// setupKWOKClusterExtended creates a KWOK cluster populated with N workloads of each of
// four types, driven entirely by kwokctl scale commands — the same mechanism used by
// setupKWOKCluster for Deployments:
//
//   - namespace-000000: N Deployments → ReplicaSets → Pods  (deployment controller)
//   - namespace-000001: N StatefulSets → Pods               (statefulset controller)
//   - namespace-000002: N DaemonSets → Pods                 (daemonset controller; nodeSelector → 1:1)
//   - namespace-000003: N CronJobs → Jobs → Pods            (cronjob+job controllers; */1 * * * *)
//
// createKWOKCluster handles node scaling (numNodes nodes) and namespace scaling
// The four workload scale commands run in parallel, as do their pod-readiness waits.
// CronJob pods receive extra timeout headroom because the schedule must first fire (≤ 60 s).
func setupKWOKClusterExtended(t *testing.T, n int) (kubeconfigPath string, podUIDs extendedClusterPodUIDs, cleanup func()) {
	t.Helper()
	// createKWOKCluster scales to numNodes nodes and n namespaces
	kc := createKWOKCluster(t, n)
	cleanup = kc.Cleanup
	kubeconfigPath = kc.KubeconfigPath
	clusterName := kc.Name
	clientset := kc.Clientset
	ctx := t.Context()

	// Single config file containing the four workload KwokctlResource templates.
	kwokConfigPath := filepath.Join(t.TempDir(), "kwok-ext-resources.yaml")
	kwokConfigContent := strings.Join([]string{
		strings.TrimSpace(kwokDeploymentResourceYAML),
		strings.TrimSpace(kwokStatefulSetResourceYAML),
		strings.TrimSpace(kwokDaemonSetResourceYAML),
		strings.TrimSpace(kwokCronJobResourceYAML),
	}, "\n---\n")
	require.NoError(t, os.WriteFile(kwokConfigPath, []byte(kwokConfigContent), 0o600))

	// Scale all four workload types in parallel.
	type scaleSpec struct {
		kind        string
		extraParams []string
	}
	workloads := []scaleSpec{
		{kind: "deployment", extraParams: []string{"--param", ".replicas=1"}},
		{kind: "statefulset", extraParams: []string{"--param", ".replicas=1"}},
		{kind: "daemonset"},
		{kind: "cronjob"},
	}

	scaleErrs := make([]error, len(workloads))
	var scaleWg sync.WaitGroup
	for wi, ws := range workloads {
		scaleWg.Go(func() {
			args := []string{
				"scale", ws.kind,
				"--replicas", fmt.Sprintf("%d", n),
				"--serial-length", "6",
				"--name", clusterName,
				"--config", kwokConfigPath,
			}
			args = append(args, ws.extraParams...)
			// #nosec G204
			cmd := exec.Command("kwokctl", args...)
			cmd.Dir = t.TempDir()
			if cmdOut, cmdErr := cmd.CombinedOutput(); cmdErr != nil {
				scaleErrs[wi] = fmt.Errorf("kwokctl scale %s: %w\n%s", ws.kind, cmdErr, cmdOut)
			}
		})
	}
	scaleWg.Wait()
	for wi, e := range scaleErrs {
		require.NoError(t, e, "scale command for workload index %d failed", wi)
	}

	// Wait for N pods in each namespace in parallel.
	// CronJob pods get extra headroom: the schedule must fire (≤ 60 s) before pods appear.
	baseWait := min(3*time.Minute+time.Duration(n/5)*time.Second, 15*time.Minute)
	nsWaits := []struct {
		ns      string
		timeout time.Duration
	}{
		{"namespace-000000", baseWait},
		{"namespace-000001", baseWait},
		{"namespace-000002", baseWait},
		{"namespace-000003", baseWait + 2*time.Minute},
	}

	podCounts := make([]int, len(nsWaits))
	var waitWg sync.WaitGroup
	for wi, target := range nsWaits {
		waitWg.Go(func() {
			assert.Eventually(t, func() bool {
				list, listErr := clientset.CoreV1().Pods(target.ns).List(ctx, metav1.ListOptions{})
				if listErr != nil {
					return false
				}
				podCounts[wi] = len(list.Items)
				return podCounts[wi] >= n
			}, target.timeout, 1*time.Second,
				"timed out waiting for %d pods in %s (got %d)", n, target.ns, podCounts[wi])
		})
	}
	waitWg.Wait()

	// Collect one representative pod UID per namespace for use in metric association.
	nsToPodUID := make([]string, len(nsWaits))
	for wi, target := range nsWaits {
		list, listErr := clientset.CoreV1().Pods(target.ns).List(ctx, metav1.ListOptions{Limit: 1})
		if listErr == nil && len(list.Items) > 0 {
			nsToPodUID[wi] = string(list.Items[0].UID)
		}
	}

	podUIDs = extendedClusterPodUIDs{
		Deployment:  nsToPodUID[0],
		StatefulSet: nsToPodUID[1],
		DaemonSet:   nsToPodUID[2],
		CronJob:     nsToPodUID[3],
	}
	require.NotEmpty(t, podUIDs.Deployment, "deploy pod UID must not be empty")
	require.NotEmpty(t, podUIDs.StatefulSet, "statefulset pod UID must not be empty")
	require.NotEmpty(t, podUIDs.DaemonSet, "daemonset pod UID must not be empty")
	require.NotEmpty(t, podUIDs.CronJob, "cronjob pod UID must not be empty")

	t.Logf("[kwok-ext] %d of each type (4×%d=%d pods total); UIDs: deploy=%s sts=%s ds=%s cj=%s",
		n, n, 4*n, podUIDs.Deployment, podUIDs.StatefulSet, podUIDs.DaemonSet, podUIDs.CronJob)

	return kubeconfigPath, podUIDs, cleanup
}

// runTestbedWithK8sConfigExtended runs the collector testbed with the extended metadata
// configuration.  It sends numMetricBatches metric batches per ownership type
// (4 types × numMetricBatches total), then validates that the processor enriches each
// batch with the correct ownership-chain attributes.
func runTestbedWithK8sConfigExtended(t *testing.T, test *k8sAttributesProcessorTestCase, kubeconfigPath string, podUIDs extendedClusterPodUIDs) {
	t.Helper()

	sender := testbed.NewOTLPMetricDataSender(testbed.DefaultHost, testutil.GetAvailablePort(t))
	receiver := testbed.NewOTLPDataReceiver(testutil.GetAvailablePort(t))

	resultDir, err := filepath.Abs(filepath.Join("results", t.Name()))
	require.NoError(t, err)

	agentProc := testbed.NewChildProcessCollector(
		testbed.WithEnvVar("GOMAXPROCS", "2"),
		testbed.WithEnvVar("KUBECONFIG", kubeconfigPath),
	)
	k8sBody := strings.Replace(test.k8sAttributesConfig, "__CONTEXT__", getKubeconfigCurrentContext(kubeconfigPath), 1)
	configStr := createConfigYaml(t, sender, receiver, resultDir,
		[]ProcessorNameAndConfigBody{{Name: "k8s_attributes", Body: k8sBody}}, nil)
	cfgCleanup, err := agentProc.PrepareConfig(t, configStr)
	require.NoError(t, err)
	defer cfgCleanup()

	dataProvider := testbed.NewPerfTestDataProvider(testbed.LoadOptions{DataItemsPerSecond: 10000, ItemsPerBatch: 10})
	tc := testbed.NewTestCase(
		t,
		dataProvider,
		sender,
		receiver,
		agentProc,
		&testbed.PerfTestValidator{},
		performanceResultsSummary,
		testbed.WithResourceLimits(testbed.ResourceSpec{
			ExpectedMaxCPU: 200,
			ExpectedMaxRAM: 3000, // extended metadata set requires more RAM at 20k-pod scale
		}),
	)
	defer tc.Stop()

	tc.StartBackend()
	tc.StartAgent()
	defer tc.StopAgent()

	// Extended sync wait: 4×N pods means the informer cache needs proportionally more time.
	syncWait := min(15*time.Second+time.Duration(4*test.numWorkloads/100)*100*time.Millisecond, 180*time.Second)
	time.Sleep(syncWait)

	tc.EnableRecording()
	require.NoError(t, sender.Start())
	tc.MockBackend.ClearReceivedItems()
	startCounter := tc.MockBackend.DataItemsReceived()

	metricSender, ok := tc.LoadGenerator.(*testbed.ProviderSender).Sender.(testbed.MetricDataSender)
	require.True(t, ok, "unsupported metric sender")

	// podTypes defines the ownership chain to exercise for each workload type,
	// the pod UID to embed in the metric for association, and the resource attributes
	// that must appear on enriched metrics for that chain.
	type podType struct {
		uid         string
		description string
		chainAttrs  []string
	}
	podTypes := []podType{
		{
			uid:         podUIDs.Deployment,
			description: "deployment",
			chainAttrs:  []string{"k8s.deployment.name", "k8s.deployment.uid", "k8s.replicaset.name", "k8s.replicaset.uid"},
		},
		{
			uid:         podUIDs.StatefulSet,
			description: "statefulset",
			chainAttrs:  []string{"k8s.statefulset.name", "k8s.statefulset.uid"},
		},
		{
			uid:         podUIDs.DaemonSet,
			description: "daemonset",
			chainAttrs:  []string{"k8s.daemonset.name", "k8s.daemonset.uid"},
		},
		{
			uid:         podUIDs.CronJob,
			description: "cronjob",
			// Both the Job and the CronJob attributes must be present for the two-level chain.
			chainAttrs: []string{"k8s.cronjob.name", "k8s.cronjob.uid", "k8s.job.name", "k8s.job.uid"},
		},
	}

	totalBatches := len(podTypes) * numMetricBatches
	for _, pt := range podTypes {
		for i := range numMetricBatches {
			m := pmetric.NewMetrics()
			mockedConsumedMetricsForK8s.CopyTo(m)
			m.ResourceMetrics().At(0).Resource().Attributes().PutStr("k8s.pod.uid", pt.uid)
			require.NoError(t, metricSender.ConsumeMetrics(t.Context(), m))
			tc.LoadGenerator.IncDataItemsSent()
			if i < numMetricBatches-1 {
				time.Sleep(100 * time.Millisecond)
			}
		}
		time.Sleep(250 * time.Millisecond) // give the pipeline time to flush between types
	}

	tc.WaitFor(
		func() bool { return tc.MockBackend.DataItemsReceived() == startCounter+uint64(totalBatches) },
		"all extended metric batches received",
	)

	received := tc.MockBackend.ReceivedMetrics
	require.Len(t, received, totalBatches, "expected %d metric batches", totalBatches)

	// Group received batch indices by k8s.pod.uid for per-chain validation.
	uidToBatches := make(map[string][]int, len(podTypes))
	for i, m := range received {
		rm := m.ResourceMetrics()
		require.Equal(t, 1, rm.Len(), "batch %d: expected 1 ResourceMetrics", i)
		gotAttrs := rm.At(0).Resource().Attributes().AsRaw()
		if test.expectedResourceAttrs != nil {
			for k, v := range test.expectedResourceAttrs {
				require.Contains(t, gotAttrs, k, "batch %d: missing resource attribute %q", i, k)
				if v != nil {
					require.Equal(t, v, gotAttrs[k], "batch %d: resource attribute %q", i, k)
				}
			}
		}
		if uid, ok := gotAttrs["k8s.pod.uid"].(string); ok {
			uidToBatches[uid] = append(uidToBatches[uid], i)
		}
	}

	for _, pt := range podTypes {
		batches, found := uidToBatches[pt.uid]
		require.True(t, found && len(batches) > 0,
			"%s: no batches received for pod UID %s", pt.description, pt.uid)

		// Spot-check the first received batch for this chain.
		attrs := received[batches[0]].ResourceMetrics().At(0).Resource().Attributes().AsRaw()
		for _, k := range pt.chainAttrs {
			require.Contains(t, attrs, k,
				"%s batch %d: missing chain attribute %q", pt.description, batches[0], k)
		}
	}
}

// TestMetricK8sAttributesProcessor tests the k8sattributes processor's
// performance and resource utilization when the component
// is used to collect k8s metadata from a test k8s cluster
// with 100 number of nodes, N number of Pods each controlled by
// its own Deployment/Replicaset, while there are also N number of Namespaces
func TestMetricK8sAttributesProcessor(t *testing.T) {
	for _, test := range getK8sAttributesProcessorBasicTestCases() {
		t.Run(test.name, func(t *testing.T) {
			skipIfKwokUnavailable(t)
			kubeconfigPath, podUID, cleanup := setupKWOKCluster(t, test.numWorkloads)
			defer cleanup()
			runTestbedWithK8sConfig(t, &test, kubeconfigPath, podUID)
			cleanup()
		})
	}
}

// TestMetricK8sAttributesProcessorExtended validates the k8sattributesprocessor at high
// scale with an extended metadata configuration, exercising four ownership chains:
//
//   - Deployment → ReplicaSet → Pod
//   - StatefulSet → Pod
//   - DaemonSet   → Pod  (single-node cluster; 1:1 ratio)
//   - CronJob     → Job → Pod
//
// Three scales are tested: 110, 1 000, and 5 000 instances of each type
// (up to 20 000 pods for the 5 K scenario).
func TestMetricK8sAttributesProcessorExtended(t *testing.T) {
	for _, test := range getK8sAttributesProcessorExtendedTestCases() {
		t.Run(test.name, func(t *testing.T) {
			skipIfKwokUnavailable(t)
			kubeconfigPath, podUIDs, cleanup := setupKWOKClusterExtended(t, test.numWorkloads)
			defer cleanup()
			runTestbedWithK8sConfigExtended(t, &test, kubeconfigPath, podUIDs)
			cleanup()
		})
	}
}
