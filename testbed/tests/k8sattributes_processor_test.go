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

func skipIfKwokUnavailable(t *testing.T) {
	if os.Getenv("SKIP_KWOK_TESTS") == "1" {
		t.Skip("Skipping KWOK test: SKIP_KWOK_TESTS=1")
	}
	if _, err := exec.LookPath("kwokctl"); err != nil {
		t.Skipf("Skipping KWOK test: kwokctl not found in PATH (install from https://kwok.sigs.k8s.io/)")
	}
}

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
	mockedConsumedMetrics pmetric.Metrics
	expectedResourceAttrs map[string]any // assert these resource attributes are present (nil = presence only)
	numPods               int            // pods and namespaces to create; each pod in its own deployment (1 replica)
}

func getK8sAttributesProcessorTestCases() []k8sAttributesProcessorTestCase {
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
			mockedConsumedMetrics: mockedConsumedMetricsForK8s,
			expectedResourceAttrs: expectedAttrs,
			numPods:               110,
		},
		{
			name:                  "1K_workload_cluster",
			k8sAttributesConfig:   kwokConfig,
			mockedConsumedMetrics: mockedConsumedMetricsForK8s,
			expectedResourceAttrs: expectedAttrs,
			numPods:               1000,
		},
		{
			name:                  "5K_workload_cluster",
			k8sAttributesConfig:   kwokConfig,
			mockedConsumedMetrics: mockedConsumedMetricsForK8s,
			expectedResourceAttrs: expectedAttrs,
			numPods:               5000,
		},
	}
}

// numMetricBatches is how many metric batches to send and assert on in the k8sattributes test.
const numMetricBatches = 10

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

// setupKWOKCluster creates a KWOK cluster with numPods pods as part of their own Deployment (1 replica).
// See https://kwok.sigs.k8s.io/
func setupKWOKCluster(t *testing.T, numPods int) (kubeconfigPath, podUID string, cleanup func()) {
	// Log kwokctl version for debugging CI/runner issues.
	if verOut, err := exec.Command("kwokctl", "--version").CombinedOutput(); err != nil {
		t.Logf("[kwok] kwokctl version: (failed to get: %v)", err)
	} else {
		t.Logf("[kwok] kwokctl version:\n%s", strings.TrimSpace(string(verOut)))
	}

	clusterName := "otelcol-k8s-" + strings.ReplaceAll(t.Name(), "/", "-")
	clusterName = strings.ReplaceAll(clusterName, " ", "-")
	if len(clusterName) > 50 {
		clusterName = clusterName[:50]
	}

	create := exec.Command("kwokctl", "create", "cluster", "--disable-qps-limits", "--runtime", "binary", "--name", clusterName)
	create.Dir = t.TempDir()
	out, err := create.CombinedOutput()
	require.NoError(t, err, "kwokctl create cluster: %s", out)

	var cleanupOnce sync.Once
	cleanup = func() {
		cleanupOnce.Do(func() {
			del := exec.Command("kwokctl", "delete", "cluster", "--name", clusterName)
			_ = del.Run()
			assert.Eventually(t, func() bool {
				getClusters := exec.Command("kwokctl", "get", "clusters")
				out, err = getClusters.Output()
				return err != nil || !strings.Contains(string(out), clusterName)
			}, 30*time.Second, 500*time.Millisecond, "cluster %s should be removed", clusterName)
		})
	}

	getConfig := exec.Command("kwokctl", "get", "kubeconfig", "--name", clusterName)
	getConfig.Dir = t.TempDir()
	out, err = getConfig.CombinedOutput()
	require.NoError(t, err, "kwokctl get kubeconfig: %s", out)

	tmpFile, err := os.CreateTemp(t.TempDir(), "kubeconfig-*.yaml")
	require.NoError(t, err)
	_, err = tmpFile.Write(out)
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())
	kubeconfigPath = tmpFile.Name()

	ctx := t.Context()
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	require.NoError(t, err)
	clientset, err := kubernetes.NewForConfig(config)
	require.NoError(t, err)

	// Create 100 fake nodes for extra cluster load using kwokctl scale
	const numNodes = 100
	// #nosec G204 -- clusterName and numNodes are test-controlled, not user input
	scaleNodes := exec.Command("kwokctl", "scale", "node", "--replicas", fmt.Sprintf("%d", numNodes), "--name", clusterName)
	scaleNodes.Dir = t.TempDir()
	if out, runErr := scaleNodes.CombinedOutput(); runErr != nil {
		t.Logf("kwokctl scale node (optional): %v\n%s", runErr, out)
	}

	kwokConfigPath := filepath.Join(t.TempDir(), "kwok-resources.yaml")
	kwokConfigContent := strings.TrimSpace(kwokNamespaceResourceYAML) + "\n---\n" + strings.TrimSpace(kwokDeploymentResourceYAML)
	require.NoError(t, os.WriteFile(kwokConfigPath, []byte(kwokConfigContent), 0o600))

	// #nosec G204 -- clusterName, numPods, kwokConfigPath are test-controlled, not user input
	scaleNS := exec.Command("kwokctl", "scale", "namespace",
		"--replicas", fmt.Sprintf("%d", numPods),
		"--serial-length", "6",
		"--name", clusterName,
		"--config", kwokConfigPath)
	scaleNS.Dir = t.TempDir()
	if out, runErr := scaleNS.CombinedOutput(); runErr != nil {
		t.Fatalf("kwokctl scale namespace: %v\n%s", runErr, out)
	}

	targetNS := "namespace-000000"
	// Wait for target namespace to exist before creating deployments (avoids races in CI).
	assert.Eventually(t, func() bool {
		_, getErr := clientset.CoreV1().Namespaces().Get(ctx, targetNS, metav1.GetOptions{})
		return getErr == nil
	}, 2*time.Minute, 2*time.Second, "namespace %s never created", targetNS)

	// #nosec G204 -- clusterName, numPods, kwokConfigPath are test-controlled, not user input
	scaleDeploy := exec.Command("kwokctl", "scale", "deployment",
		"--replicas", fmt.Sprintf("%d", numPods),
		"--serial-length", "6",
		"--param", ".replicas=1",
		"--name", clusterName,
		"--config", kwokConfigPath)
	scaleDeploy.Dir = t.TempDir()
	if out, runErr := scaleDeploy.CombinedOutput(); runErr != nil {
		t.Fatalf("kwokctl scale deployment: %v\n%s", runErr, out)
	}

	// Wait until we have numPods pods in namespace-000000 (all deployments go there)
	podWaitTimeout := min(3*time.Minute+time.Duration(numPods/5)*time.Second, 15*time.Minute)
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
		return podCount >= numPods
	}, podWaitTimeout, 1*time.Second, "timed out waiting for %d pods in namespace-000000 (got %d)", numPods, podCount)

	// Get first pod UID in namespace-000000 for metric association
	list, listErr := clientset.CoreV1().Pods(targetNS).List(ctx, metav1.ListOptions{})
	if listErr == nil && len(list.Items) > 0 {
		podUID = string(list.Items[0].UID)
	}

	return kubeconfigPath, podUID, cleanup
}

// TestMetricK8sAttributesProcessor tests the k8sattributes processor's
// performance and resource utilization when the component
// is used to collect k8s metadata from a test k8s cluster
// with 100 number of nodes, N number of Pods each controlled by
// its own Deployment/Replicaset, while there are also N number of Namespaces
func TestMetricK8sAttributesProcessor(t *testing.T) {
	tests := getK8sAttributesProcessorTestCases()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			skipIfKwokUnavailable(t)
			kubeconfigPath, podUID, cleanup := setupKWOKCluster(t, test.numPods)
			defer cleanup()
			runTestbedWithK8sConfig(t, &test, kubeconfigPath, podUID)
			cleanup()
		})
	}
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
	syncWait := min(15*time.Second+time.Duration(test.numPods/100)*100*time.Millisecond, 60*time.Second)
	time.Sleep(syncWait)

	tc.EnableRecording()

	require.NoError(t, sender.Start())

	tc.MockBackend.ClearReceivedItems()
	startCounter := tc.MockBackend.DataItemsReceived()

	sender, ok := tc.LoadGenerator.(*testbed.ProviderSender).Sender.(testbed.MetricDataSender)
	require.True(t, ok, "unsupported metric sender")

	for i := range numMetricBatches {
		metricsToSend := pmetric.NewMetrics()
		test.mockedConsumedMetrics.CopyTo(metricsToSend)
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
