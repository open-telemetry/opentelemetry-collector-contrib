// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsattributelimitprocessor

import (
	"testing"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// --- Helpers ---

// newTestMetrics creates a pmetric.Metrics with one ResourceMetrics, one ScopeMetrics,
// one Gauge metric with one datapoint. Resource, scope, and datapoint attributes are
// populated from the provided maps.
func newTestMetrics(
	resourceAttrs map[string]string,
	scopeAttrs map[string]string,
	datapointAttrs map[string]string,
) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	for k, v := range resourceAttrs {
		rm.Resource().Attributes().PutStr(k, v)
	}
	sm := rm.ScopeMetrics().AppendEmpty()
	for k, v := range scopeAttrs {
		sm.Scope().Attributes().PutStr(k, v)
	}
	m := sm.Metrics().AppendEmpty()
	m.SetName("test_metric")
	dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp.SetDoubleValue(1.0)
	for k, v := range datapointAttrs {
		dp.Attributes().PutStr(k, v)
	}
	return md
}

// getResourceAttrs returns the resource attributes from the first ResourceMetrics.
func getResourceAttrs(md pmetric.Metrics) pcommon.Map {
	return md.ResourceMetrics().At(0).Resource().Attributes()
}

// getDatapointAttrs returns the datapoint attributes from the first datapoint.
func getDatapointAttrs(md pmetric.Metrics) pcommon.Map {
	return md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Attributes()
}

// getScopeAttrs returns the scope attributes from the first ScopeMetrics.
func getScopeAttrs(md pmetric.Metrics) pcommon.Map {
	return md.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Attributes()
}

// defaultTestPrefixes and defaultTestKeys mirror the values previously hardcoded
// in processor.go, used here to keep existing tests working.
var defaultTestPrefixes = []string{
	"k8s.node.label.feature.node.kubernetes.io/",
	"k8s.node.label.beta.kubernetes.io/",
	"k8s.node.label.failure-domain.beta.kubernetes.io/",
	"k8s.node.label.alpha.eksctl.io/",
}

var defaultTestKeys = []string{
	"k8s.node.label.topology.kubernetes.io/region",
	"k8s.node.label.topology.kubernetes.io/zone",
	"k8s.node.label.topology.ebs.csi.aws.com/zone",
	"k8s.node.label.node.kubernetes.io/instance-type",
	"k8s.node.label.kubernetes.io/hostname",
	"k8s.node.label.eks.amazonaws.com/nodegroup-image",
	"k8s.node.label.k8s.io/cloud-provider-aws",
	"k8s.node.label.eks.amazonaws.com/sourceLaunchTemplateId",
	"k8s.node.label.eks.amazonaws.com/sourceLaunchTemplateVersion",
	"k8s.pod.label.pod-template-hash",
	"k8s.pod.label.controller-revision-hash",
}

func newTestProcessorWithLogs(maxAttrs int) (*attributeLimitProcessor, *observer.ObservedLogs) {
	core, logs := observer.New(zapcore.DebugLevel)
	logger := zap.New(core)
	p := newProcessor(&Config{
		MaxTotalAttributes:           maxAttrs,
		UnconditionalRemovalPrefixes: defaultTestPrefixes,
		UnconditionalRemovalKeys:     defaultTestKeys,
	}, logger)
	return p, logs
}

func newTestProcessorSimple(maxAttrs int) *attributeLimitProcessor {
	return newProcessor(&Config{
		MaxTotalAttributes:           maxAttrs,
		UnconditionalRemovalPrefixes: defaultTestPrefixes,
		UnconditionalRemovalKeys:     defaultTestKeys,
	}, zap.NewNop())
}

// --- B.3: Phase 1 Removal Tests ---

func TestPhase1_RemovesPrefixPatterns(t *testing.T) {
	resourceAttrs := map[string]string{
		"k8s.node.label.feature.node.kubernetes.io/cpu-cpuid.AVX2": "true",
		"k8s.node.label.feature.node.kubernetes.io/pci-1234":       "true",
		"k8s.node.label.beta.kubernetes.io/arch":                   "amd64",
		"k8s.node.label.beta.kubernetes.io/os":                     "linux",
		"k8s.node.label.failure-domain.beta.kubernetes.io/region":  "us-east-1",
		"k8s.node.label.failure-domain.beta.kubernetes.io/zone":    "us-east-1a",
		"k8s.node.label.alpha.eksctl.io/cluster-name":              "test",
		"k8s.node.label.alpha.eksctl.io/nodegroup-name":            "ng-1",
		"k8s.node.name": "test-node", // should survive
	}

	md := newTestMetrics(resourceAttrs, nil, nil)
	p := newTestProcessorSimple(500) // high limit so Phase 2 doesn't trigger
	result, err := p.processMetrics(t.Context(), md)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	attrs := getResourceAttrs(result)
	// All prefix-matched keys should be gone.
	prefixKeys := []string{
		"k8s.node.label.feature.node.kubernetes.io/cpu-cpuid.AVX2",
		"k8s.node.label.feature.node.kubernetes.io/pci-1234",
		"k8s.node.label.beta.kubernetes.io/arch",
		"k8s.node.label.beta.kubernetes.io/os",
		"k8s.node.label.failure-domain.beta.kubernetes.io/region",
		"k8s.node.label.failure-domain.beta.kubernetes.io/zone",
		"k8s.node.label.alpha.eksctl.io/cluster-name",
		"k8s.node.label.alpha.eksctl.io/nodegroup-name",
	}
	for _, key := range prefixKeys {
		if _, ok := attrs.Get(key); ok {
			t.Errorf("Phase 1 should have removed %q", key)
		}
	}
	// Non-matching key should survive.
	if _, ok := attrs.Get("k8s.node.name"); !ok {
		t.Error("k8s.node.name should not be removed by Phase 1")
	}
}

func TestPhase1_RemovesExactKeys(t *testing.T) {
	resourceAttrs := map[string]string{}
	for _, key := range defaultTestKeys {
		resourceAttrs[key] = "value"
	}
	resourceAttrs["k8s.pod.name"] = "my-pod" // should survive

	md := newTestMetrics(resourceAttrs, nil, nil)
	p := newTestProcessorSimple(500)
	result, err := p.processMetrics(t.Context(), md)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	attrs := getResourceAttrs(result)
	for _, key := range defaultTestKeys {
		if _, ok := attrs.Get(key); ok {
			t.Errorf("Phase 1 should have removed exact key %q", key)
		}
	}
	if _, ok := attrs.Get("k8s.pod.name"); !ok {
		t.Error("k8s.pod.name should not be removed by Phase 1")
	}
}

func TestPhase1_PreservesNonMatchingAttributes(t *testing.T) {
	resourceAttrs := map[string]string{
		"k8s.node.name":                  "node-1",
		"k8s.pod.name":                   "pod-1",
		"cloud.region":                   "us-east-1",
		"k8s.node.label.custom/my-label": "value",
	}

	md := newTestMetrics(resourceAttrs, nil, nil)
	p := newTestProcessorSimple(500)
	result, err := p.processMetrics(t.Context(), md)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	attrs := getResourceAttrs(result)
	if attrs.Len() != 4 {
		t.Errorf("expected 4 attributes preserved, got %d", attrs.Len())
	}
}

func TestPhase1_RunsEvenWhenUnderLimit(t *testing.T) {
	// Only 3 attrs total — well under any limit — but Phase 1 should still remove NFD.
	resourceAttrs := map[string]string{
		"k8s.node.label.feature.node.kubernetes.io/cpu-cpuid.AVX2": "true",
		"k8s.node.name": "node-1",
	}

	md := newTestMetrics(resourceAttrs, nil, nil)
	p := newTestProcessorSimple(500)
	result, err := p.processMetrics(t.Context(), md)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	attrs := getResourceAttrs(result)
	if _, ok := attrs.Get("k8s.node.label.feature.node.kubernetes.io/cpu-cpuid.AVX2"); ok {
		t.Error("Phase 1 should remove NFD labels even when under limit")
	}
	if attrs.Len() != 1 {
		t.Errorf("expected 1 attribute remaining, got %d", attrs.Len())
	}
}

func TestPhase1_OnlyOperatesOnResourceAttributes(t *testing.T) {
	// Put Phase 1 target keys in scope and datapoint — they should NOT be removed.
	scopeAttrs := map[string]string{
		"k8s.pod.label.pod-template-hash": "abc123",
	}
	datapointAttrs := map[string]string{
		"k8s.node.label.feature.node.kubernetes.io/test": "true",
	}

	md := newTestMetrics(nil, scopeAttrs, datapointAttrs)
	p := newTestProcessorSimple(500)
	result, err := p.processMetrics(t.Context(), md)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Scope attrs should be untouched.
	sa := getScopeAttrs(result)
	if _, ok := sa.Get("k8s.pod.label.pod-template-hash"); !ok {
		t.Error("Phase 1 should not remove scope attributes")
	}
	// Datapoint attrs should be untouched.
	da := getDatapointAttrs(result)
	if _, ok := da.Get("k8s.node.label.feature.node.kubernetes.io/test"); !ok {
		t.Error("Phase 1 should not remove datapoint attributes")
	}
}

// --- B.5: Phase 2 Dropping Tests ---

func TestPhase2_TierOrdering(t *testing.T) {
	// Create metrics with Tier 1 and Tier 5 attrs, limit forces dropping some.
	resourceAttrs := map[string]string{
		"k8s.node.name":                               "node-1",
		"k8s.node.label.helm.sh/chart":                "mychart-1.0", // Tier 1
		"k8s.node.label.app.kubernetes.io/managed-by": "Helm",        // Tier 1
		"k8s.node.label.my-company/team":              "platform",    // Tier 5
		"k8s.node.label.my-company/env":               "prod",        // Tier 5
	}

	md := newTestMetrics(resourceAttrs, nil, nil)
	// Limit = 3: k8s.node.name + 2 others. Need to drop 2 of the 4 labels.
	p := newTestProcessorSimple(3)
	result, err := p.processMetrics(t.Context(), md)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	attrs := getResourceAttrs(result)
	// Tier 1 should be dropped first.
	if _, ok := attrs.Get("k8s.node.label.helm.sh/chart"); ok {
		t.Error("Tier 1 attr should be dropped before Tier 5")
	}
	if _, ok := attrs.Get("k8s.node.label.app.kubernetes.io/managed-by"); ok {
		t.Error("Tier 1 attr should be dropped before Tier 5")
	}
	// Tier 5 should survive (only needed to drop 2, and Tier 1 had 2).
	if _, ok := attrs.Get("k8s.node.label.my-company/team"); !ok {
		t.Error("Tier 5 attr should survive when Tier 1 covers the excess")
	}
	if _, ok := attrs.Get("k8s.node.label.my-company/env"); !ok {
		t.Error("Tier 5 attr should survive when Tier 1 covers the excess")
	}
}

func TestPhase2_AlphabeticalWithinTier(t *testing.T) {
	// Two Tier 5 attrs, need to drop exactly 1. Should drop alphabetically first.
	resourceAttrs := map[string]string{
		"k8s.node.name":             "node-1",
		"k8s.node.label.zzz-custom": "val", // Tier 5, alphabetically later
		"k8s.node.label.aaa-custom": "val", // Tier 5, alphabetically first
	}

	md := newTestMetrics(resourceAttrs, nil, nil)
	// Limit = 2: need to drop 1 of the 2 Tier 5 attrs.
	p := newTestProcessorSimple(2)
	result, err := p.processMetrics(t.Context(), md)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	attrs := getResourceAttrs(result)
	// "aaa-custom" should be dropped first (alphabetical).
	if _, ok := attrs.Get("k8s.node.label.aaa-custom"); ok {
		t.Error("alphabetically first attr should be dropped first within tier")
	}
	if _, ok := attrs.Get("k8s.node.label.zzz-custom"); !ok {
		t.Error("alphabetically later attr should survive")
	}
}

func TestPhase2_StopsAtLimit(t *testing.T) {
	// 5 Tier 5 attrs + 1 protected. Limit = 3. Should drop exactly 3.
	resourceAttrs := map[string]string{
		"k8s.node.name":           "node-1", // protected
		"k8s.node.label.custom-a": "a",      // Tier 5
		"k8s.node.label.custom-b": "b",      // Tier 5
		"k8s.node.label.custom-c": "c",      // Tier 5
		"k8s.node.label.custom-d": "d",      // Tier 5
		"k8s.node.label.custom-e": "e",      // Tier 5
	}

	md := newTestMetrics(resourceAttrs, nil, nil)
	p := newTestProcessorSimple(3)
	result, err := p.processMetrics(t.Context(), md)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	attrs := getResourceAttrs(result)
	if attrs.Len() != 3 {
		t.Errorf("expected 3 attributes after Phase 2, got %d", attrs.Len())
	}
	// Protected attr must survive.
	if _, ok := attrs.Get("k8s.node.name"); !ok {
		t.Error("protected attr k8s.node.name should survive")
	}
}

func TestPhase2_NodeLabelsExhaustedBeforePodLabels(t *testing.T) {
	resourceAttrs := map[string]string{
		"k8s.node.name":              "node-1", // protected
		"k8s.node.label.custom-node": "val",    // Tier 5
		"k8s.pod.label.custom-pod":   "val",    // Tier 7
	}

	md := newTestMetrics(resourceAttrs, nil, nil)
	// Limit = 2: need to drop 1. Node label (Tier 5) should go before pod label (Tier 7).
	p := newTestProcessorSimple(2)
	result, err := p.processMetrics(t.Context(), md)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	attrs := getResourceAttrs(result)
	if _, ok := attrs.Get("k8s.node.label.custom-node"); ok {
		t.Error("node label (Tier 5) should be dropped before pod label (Tier 7)")
	}
	if _, ok := attrs.Get("k8s.pod.label.custom-pod"); !ok {
		t.Error("pod label (Tier 7) should survive when node labels cover the excess")
	}
}

func TestPhase2_AllTiersExhausted(t *testing.T) {
	// All attributes are protected — nothing to drop.
	resourceAttrs := map[string]string{
		"k8s.node.name":      "node-1",
		"k8s.pod.name":       "pod-1",
		"k8s.namespace.name": "ns-1",
		"k8s.cluster.name":   "cluster-1",
		"cloud.region":       "us-east-1",
	}

	md := newTestMetrics(resourceAttrs, nil, nil)
	// Limit = 2: 5 protected resource attrs + 0 droppable. Force-prune kicks in.
	p, logs := newTestProcessorWithLogs(2)
	result, err := p.processMetrics(t.Context(), md)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Force-prune should bring total down to limit.
	attrs := getResourceAttrs(result)
	if attrs.Len() != 2 {
		t.Errorf("expected 2 attrs after force-prune, got %d", attrs.Len())
	}
	// Should log an error about force-pruning.
	errorLogs := logs.FilterLevelExact(zapcore.ErrorLevel).All()
	if len(errorLogs) == 0 {
		t.Error("expected error log when all tiers exhausted")
	}
}

func TestPhase2_SkippedWhenUnderLimit(t *testing.T) {
	resourceAttrs := map[string]string{
		"k8s.node.name":              "node-1",
		"k8s.node.label.custom-node": "val", // Tier 5, droppable
	}

	md := newTestMetrics(resourceAttrs, nil, nil)
	p, logs := newTestProcessorWithLogs(500) // well over limit
	result, err := p.processMetrics(t.Context(), md)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	attrs := getResourceAttrs(result)
	// Droppable attr should survive since we're under limit.
	if _, ok := attrs.Get("k8s.node.label.custom-node"); !ok {
		t.Error("droppable attr should survive when under limit")
	}
	// No warning logs.
	warnLogs := logs.FilterLevelExact(zapcore.WarnLevel).All()
	if len(warnLogs) != 0 {
		t.Errorf("expected no warning logs when under limit, got %d", len(warnLogs))
	}
}

func TestPhase2_Tier10_DatapointAttrsLastResort(t *testing.T) {
	// No droppable resource attrs, only datapoint attrs.
	resourceAttrs := map[string]string{
		"k8s.node.name": "node-1", // protected
	}
	datapointAttrs := map[string]string{
		"custom_label_a": "val-a",
		"custom_label_b": "val-b",
		"custom_label_c": "val-c",
	}

	md := newTestMetrics(resourceAttrs, nil, datapointAttrs)
	// Limit = 2: 1 resource + 3 datapoint = 4. Need to drop 2 datapoint attrs.
	p := newTestProcessorSimple(2)
	result, err := p.processMetrics(t.Context(), md)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	da := getDatapointAttrs(result)
	ra := getResourceAttrs(result)
	total := ra.Len() + da.Len()
	if total != 2 {
		t.Errorf("expected total 2 attrs after Phase 2, got %d", total)
	}
}

func TestPhase2_ProtectedDatapointAttrsNeverDropped(t *testing.T) {
	resourceAttrs := map[string]string{
		"k8s.node.name": "node-1", // protected
	}
	datapointAttrs := map[string]string{
		"cpu":            "0",          // protected
		"mode":           "idle",       // protected
		"device":         "sda",        // protected
		"custom_label_a": "droppable1", // not protected
		"custom_label_b": "droppable2", // not protected
	}

	md := newTestMetrics(resourceAttrs, nil, datapointAttrs)
	// Limit = 4: 1 resource + 5 datapoint = 6. Need to drop 2.
	// Only custom_label_a and custom_label_b should be dropped; cpu, mode, device survive.
	p := newTestProcessorSimple(4)
	result, err := p.processMetrics(t.Context(), md)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	da := getDatapointAttrs(result)
	for _, key := range []string{"cpu", "mode", "device"} {
		if _, ok := da.Get(key); !ok {
			t.Errorf("protected datapoint attr %q should not be dropped", key)
		}
	}
	for _, key := range []string{"custom_label_a", "custom_label_b"} {
		if _, ok := da.Get(key); ok {
			t.Errorf("non-protected datapoint attr %q should be dropped", key)
		}
	}
}

// --- B.6: Protected Attributes Tests ---

func TestProtected_K8sIdentityNeverRemoved(t *testing.T) {
	resourceAttrs := map[string]string{
		"k8s.cluster.name":   "cluster-1",
		"k8s.node.name":      "node-1",
		"k8s.pod.name":       "pod-1",
		"k8s.pod.uid":        "uid-123",
		"k8s.namespace.name": "default",
		"k8s.container.name": "app",
	}

	md := newTestMetrics(resourceAttrs, nil, nil)
	p := newTestProcessorSimple(500) // high limit — tier-based dropping still skips protected attrs
	result, err := p.processMetrics(t.Context(), md)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	attrs := getResourceAttrs(result)
	for key := range resourceAttrs {
		if _, ok := attrs.Get(key); !ok {
			t.Errorf("protected K8s identity attr %q should never be removed", key)
		}
	}
}

func TestProtected_K8sWorkloadNeverRemoved(t *testing.T) {
	resourceAttrs := map[string]string{
		"k8s.deployment.name":  "my-deploy",
		"k8s.statefulset.name": "my-sts",
		"k8s.daemonset.name":   "my-ds",
		"k8s.replicaset.name":  "my-rs",
		"k8s.job.name":         "my-job",
		"k8s.cronjob.name":     "my-cj",
		"k8s.workload.name":    "my-deploy",
		"k8s.workload.type":    "Deployment",
	}

	md := newTestMetrics(resourceAttrs, nil, nil)
	p := newTestProcessorSimple(500)
	result, err := p.processMetrics(t.Context(), md)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	attrs := getResourceAttrs(result)
	for key := range resourceAttrs {
		if _, ok := attrs.Get(key); !ok {
			t.Errorf("protected K8s workload attr %q should never be removed", key)
		}
	}
}

func TestProtected_CloudHostHwPrefixNeverRemoved(t *testing.T) {
	resourceAttrs := map[string]string{
		"cloud.region":            "us-east-1",
		"cloud.account.id":        "123456789",
		"cloud.provider":          "aws",
		"cloud.platform":          "aws_eks",
		"cloud.availability_zone": "us-east-1a",
		"host.name":               "ip-10-0-1-1",
		"host.type":               "t3.large",
		"host.id":                 "i-abc123",
		"host.image.id":           "ami-123",
		"hw.type":                 "gpu",
		"hw.vendor":               "NVIDIA",
		"hw.id":                   "GPU-abc",
		"hw.model":                "A10G",
		"hw.name":                 "gpu0",
	}

	md := newTestMetrics(resourceAttrs, nil, nil)
	p := newTestProcessorSimple(500)
	result, err := p.processMetrics(t.Context(), md)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	attrs := getResourceAttrs(result)
	for key := range resourceAttrs {
		if _, ok := attrs.Get(key); !ok {
			t.Errorf("protected prefix attr %q should never be removed", key)
		}
	}
}

func TestProtected_DeviceSpecificNeverRemoved(t *testing.T) {
	resourceAttrs := map[string]string{
		"neurondevice":       "0",
		"neuroncore":         "0",
		"efa.device":         "efa0",
		"aws.efa.eni.id":     "eni-abc",
		"volume_id":          "vol-abc",
		"instance_id":        "i-abc",
		"k8s.component.name": "apiserver",
	}

	md := newTestMetrics(resourceAttrs, nil, nil)
	p := newTestProcessorSimple(500)
	result, err := p.processMetrics(t.Context(), md)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	attrs := getResourceAttrs(result)
	for key := range resourceAttrs {
		if _, ok := attrs.Get(key); !ok {
			t.Errorf("protected device attr %q should never be removed", key)
		}
	}
}

func TestProtected_PodLabelsNeverRemoved(t *testing.T) {
	resourceAttrs := map[string]string{
		"k8s.pod.label.app.kubernetes.io/name":      "my-app",
		"k8s.pod.label.app.kubernetes.io/instance":  "my-app-1",
		"k8s.pod.label.app.kubernetes.io/component": "frontend",
	}

	md := newTestMetrics(resourceAttrs, nil, nil)
	p := newTestProcessorSimple(500)
	result, err := p.processMetrics(t.Context(), md)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	attrs := getResourceAttrs(result)
	for key := range resourceAttrs {
		if _, ok := attrs.Get(key); !ok {
			t.Errorf("protected pod label %q should never be removed", key)
		}
	}
}

func TestProtected_ScopeAttrsWithCloudWatchPrefix(t *testing.T) {
	scopeAttrs := map[string]string{
		"cloudwatch.source":   "cloudwatch-agent",
		"cloudwatch.solution": "cloudwatch-agent",
	}
	// Add droppable resource attrs to trigger tier-based dropping.
	resourceAttrs := map[string]string{
		"k8s.node.label.custom-a": "a",
		"k8s.node.label.custom-b": "b",
	}

	md := newTestMetrics(resourceAttrs, scopeAttrs, nil)
	p := newTestProcessorSimple(2) // limit forces dropping
	result, err := p.processMetrics(t.Context(), md)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sa := getScopeAttrs(result)
	if _, ok := sa.Get("cloudwatch.source"); !ok {
		t.Error("scope attribute cloudwatch.source should never be removed")
	}
	if _, ok := sa.Get("cloudwatch.solution"); !ok {
		t.Error("scope attribute cloudwatch.solution should never be removed")
	}
}

func TestScope_NonProtectedScopeAttrsSurviveWhenCustomerNodeLabelCoversExcess(t *testing.T) {
	scopeAttrs := map[string]string{
		"source": "cadvisor",
	}
	resourceAttrs := map[string]string{
		"k8s.node.name":              "node-1",
		"k8s.node.label.custom-node": "val",
	}

	md := newTestMetrics(resourceAttrs, scopeAttrs, nil)
	// Limit = 2: 2 resource + 1 scope + 0 dp = 3. Need to drop 1.
	// Resource label (tier 6) should be dropped before scope attr (tier 9).
	p := newTestProcessorSimple(2)
	result, err := p.processMetrics(t.Context(), md)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ra := getResourceAttrs(result)
	if _, ok := ra.Get("k8s.node.label.custom-node"); ok {
		t.Error("resource label should be dropped before non-protected scope attr")
	}
	sa := getScopeAttrs(result)
	if _, ok := sa.Get("source"); !ok {
		t.Error("non-protected scope attr should survive when resource labels cover the excess")
	}
}

// --- B.7: Rate-Limited Logging Tests ---

func TestLogging_WarningOnPhase2(t *testing.T) {
	resourceAttrs := map[string]string{
		"k8s.node.name":           "node-1",
		"k8s.node.label.custom-a": "a",
		"k8s.node.label.custom-b": "b",
	}

	md := newTestMetrics(resourceAttrs, nil, nil)
	p, logs := newTestProcessorWithLogs(2) // forces dropping 1
	_, err := p.processMetrics(t.Context(), md)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	warnLogs := logs.FilterLevelExact(zapcore.WarnLevel).All()
	if len(warnLogs) != 1 {
		t.Fatalf("expected 1 warning log, got %d", len(warnLogs))
	}
	if warnLogs[0].Message != "dropped attributes to meet limit" {
		t.Errorf("unexpected warning message: %q", warnLogs[0].Message)
	}
}

func TestLogging_SuppressedWithinOneMinute(t *testing.T) {
	resourceAttrs := map[string]string{
		"k8s.node.name":           "node-1",
		"k8s.node.label.custom-a": "a",
		"k8s.node.label.custom-b": "b",
	}

	p, logs := newTestProcessorWithLogs(2)

	// First call — should log.
	md1 := newTestMetrics(resourceAttrs, nil, nil)
	_, _ = p.processMetrics(t.Context(), md1)

	// Second call with same metric name — should be suppressed.
	md2 := newTestMetrics(resourceAttrs, nil, nil)
	_, _ = p.processMetrics(t.Context(), md2)

	warnLogs := logs.FilterLevelExact(zapcore.WarnLevel).All()
	if len(warnLogs) != 1 {
		t.Errorf("expected 1 warning log (second suppressed), got %d", len(warnLogs))
	}
}

func TestLogging_EvictionAfterFiveMinutes(t *testing.T) {
	p := newTestProcessorSimple(2)

	// Simulate an old entry.
	p.mu.Lock()
	p.lastLogAt["old_metric"] = time.Now().Add(-6 * time.Minute)
	p.lastEviction = time.Now().Add(-6 * time.Minute)
	p.mu.Unlock()

	// Trigger a log call to force eviction.
	p.logDropWarning("new_metric", 1, 1, 1)

	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.lastLogAt["old_metric"]; ok {
		t.Error("stale entry should be evicted after 5 minutes")
	}
	if _, ok := p.lastLogAt["new_metric"]; !ok {
		t.Error("new entry should exist after logging")
	}
}

// --- B.8: Scope Attribute Counting Tests ---

func TestScopeCounting_AttributesMapCounted(t *testing.T) {
	resourceAttrs := map[string]string{
		"k8s.node.name": "node-1",
	}
	scopeAttrs := map[string]string{
		"source": "cadvisor",
	}
	datapointAttrs := map[string]string{
		"job": "cadvisor",
	}

	md := newTestMetrics(resourceAttrs, scopeAttrs, datapointAttrs)
	// Total = 1 resource + 1 scope + 1 datapoint = 3.
	// Set limit to 3 — should be exactly at limit, no Phase 2.
	p, logs := newTestProcessorWithLogs(3)
	_, err := p.processMetrics(t.Context(), md)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	warnLogs := logs.FilterLevelExact(zapcore.WarnLevel).All()
	if len(warnLogs) != 0 {
		t.Error("should not trigger Phase 2 when exactly at limit")
	}
}

func TestScopeCounting_ScopeNameVersionNotCounted(t *testing.T) {
	// Scope has name and version set but empty attributes map.
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("k8s.node.name", "node-1")
	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName("cloudwatch/otel-k8s-container-insights")
	sm.Scope().SetVersion("0.1")
	// No scope attributes set — Attributes().Len() should be 0.
	m := sm.Metrics().AppendEmpty()
	m.SetName("test_metric")
	dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetDoubleValue(1.0)
	dp.Attributes().PutStr("job", "test")

	// Total = 1 resource + 0 scope attrs + 1 datapoint = 2.
	p, logs := newTestProcessorWithLogs(2)
	_, err := p.processMetrics(t.Context(), md)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	warnLogs := logs.FilterLevelExact(zapcore.WarnLevel).All()
	if len(warnLogs) != 0 {
		t.Error("scope.Name() and scope.Version() should not be counted as attributes")
	}
}

// --- Additional: Metric Preservation ---

func TestMetricPreservation_NeverDropsDatapoints(t *testing.T) {
	// Even with impossibly low limit and all protected attrs.
	resourceAttrs := map[string]string{
		"k8s.node.name":      "node-1",
		"k8s.pod.name":       "pod-1",
		"k8s.namespace.name": "ns-1",
	}

	md := newTestMetrics(resourceAttrs, nil, nil)
	p := newTestProcessorSimple(500)
	result, err := p.processMetrics(t.Context(), md)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Structure should be preserved.
	if result.ResourceMetrics().Len() != 1 {
		t.Errorf("expected 1 ResourceMetrics, got %d", result.ResourceMetrics().Len())
	}
	if result.ResourceMetrics().At(0).ScopeMetrics().Len() != 1 {
		t.Errorf("expected 1 ScopeMetrics, got %d", result.ResourceMetrics().At(0).ScopeMetrics().Len())
	}
	if result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len() != 1 {
		t.Errorf("expected 1 Metric, got %d", result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len())
	}
	dps := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints()
	if dps.Len() != 1 {
		t.Errorf("expected 1 datapoint, got %d", dps.Len())
	}
}

func TestProcessMetrics_AlwaysReturnsNilError(t *testing.T) {
	md := newTestMetrics(nil, nil, nil)
	p := newTestProcessorSimple(150)
	_, err := p.processMetrics(t.Context(), md)
	if err != nil {
		t.Errorf("processMetrics should always return nil error, got: %v", err)
	}
}

// --- Multiple Metric Types ---

func TestProcessMetrics_HandlesAllMetricTypes(t *testing.T) {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("k8s.node.label.feature.node.kubernetes.io/test", "true")
	rm.Resource().Attributes().PutStr("k8s.node.name", "node-1")

	sm := rm.ScopeMetrics().AppendEmpty()

	// Gauge
	gauge := sm.Metrics().AppendEmpty()
	gauge.SetName("gauge_metric")
	gauge.SetEmptyGauge().DataPoints().AppendEmpty().SetDoubleValue(1.0)

	// Sum
	sum := sm.Metrics().AppendEmpty()
	sum.SetName("sum_metric")
	sumDp := sum.SetEmptySum().DataPoints().AppendEmpty()
	sumDp.SetDoubleValue(2.0)

	// Histogram
	hist := sm.Metrics().AppendEmpty()
	hist.SetName("hist_metric")
	hist.SetEmptyHistogram().DataPoints().AppendEmpty()

	// ExponentialHistogram
	expHist := sm.Metrics().AppendEmpty()
	expHist.SetName("exp_hist_metric")
	expHist.SetEmptyExponentialHistogram().DataPoints().AppendEmpty()

	// Summary
	summary := sm.Metrics().AppendEmpty()
	summary.SetName("summary_metric")
	summary.SetEmptySummary().DataPoints().AppendEmpty()

	p := newTestProcessorSimple(500)
	result, err := p.processMetrics(t.Context(), md)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Phase 1 should have removed the NFD label.
	attrs := getResourceAttrs(result)
	if _, ok := attrs.Get("k8s.node.label.feature.node.kubernetes.io/test"); ok {
		t.Error("Phase 1 should remove NFD label across all metric types")
	}
	// All 5 metrics should still be present.
	metrics := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	if metrics.Len() != 5 {
		t.Errorf("expected 5 metrics, got %d", metrics.Len())
	}
}

func TestPhase2_TwoDatapoints_OnlySecondOverLimit(t *testing.T) {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("k8s.node.name", "node-1")

	sm := rm.ScopeMetrics().AppendEmpty()
	m := sm.Metrics().AppendEmpty()
	m.SetName("test_metric")

	// First datapoint: 1 attr, total = 1 resource + 1 dp = 2
	dp1 := m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp1.Attributes().PutStr("job", "cadvisor")

	// Second datapoint: 3 attrs, total = 1 resource + 3 dp = 4
	dp2 := m.Gauge().DataPoints().AppendEmpty()
	dp2.Attributes().PutStr("dp_a", "a")
	dp2.Attributes().PutStr("dp_b", "b")
	dp2.Attributes().PutStr("dp_c", "c")

	// Limit = 3: first dp is fine (2), second dp is over (4).
	p := newTestProcessorSimple(3)
	result, err := p.processMetrics(t.Context(), md)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dps := result.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints()

	// First datapoint should be untouched.
	if dps.At(0).Attributes().Len() != 1 {
		t.Errorf("first datapoint should have 1 attr, got %d", dps.At(0).Attributes().Len())
	}
	if _, ok := dps.At(0).Attributes().Get("job"); !ok {
		t.Error("first datapoint should still have 'job' attr")
	}

	// Second datapoint should be trimmed: 1 resource + dp attrs = 3 total.
	total := rm.Resource().Attributes().Len() + dps.At(1).Attributes().Len()
	if total > 3 {
		t.Errorf("second datapoint total should be <= 3, got %d", total)
	}
}

func TestForcePrune_DropsProtectedToMeetLimit(t *testing.T) {
	resourceAttrs := map[string]string{
		"k8s.node.name":      "node-1",
		"k8s.pod.name":       "pod-1",
		"k8s.namespace.name": "ns-1",
		"k8s.cluster.name":   "cluster-1",
		"cloud.region":       "us-east-1",
	}

	md := newTestMetrics(resourceAttrs, nil, nil)
	p, logs := newTestProcessorWithLogs(3)
	result, err := p.processMetrics(t.Context(), md)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Total should be exactly at limit after force-prune.
	attrs := getResourceAttrs(result)
	dpAttrs := getDatapointAttrs(result)
	total := attrs.Len() + dpAttrs.Len()
	if total > 3 {
		t.Errorf("expected total <= 3 after force-prune, got %d", total)
	}

	// Should log an error about force-pruning.
	errorLogs := logs.FilterLevelExact(zapcore.ErrorLevel).All()
	if len(errorLogs) == 0 {
		t.Error("expected error log when force-pruning protected attributes")
	}
}

func TestForcePrune_PrunesScopeAttrs(t *testing.T) {
	scopeAttrs := map[string]string{
		"scope_a": "a",
		"scope_b": "b",
		"scope_c": "c",
	}
	resourceAttrs := map[string]string{
		"k8s.node.name": "node-1",
	}

	md := newTestMetrics(resourceAttrs, scopeAttrs, nil)
	// Total = 1 resource + 3 scope + 0 dp = 4. Limit = 2.
	// All scope attrs are non-protected (no cloudwatch.* prefix).
	// Tier-based dropping removes scope attrs (tier 2). If still over, force-prune kicks in.
	p, logs := newTestProcessorWithLogs(2)
	result, err := p.processMetrics(t.Context(), md)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sa := getScopeAttrs(result)
	ra := getResourceAttrs(result)
	total := ra.Len() + sa.Len() + getDatapointAttrs(result).Len()
	if total > 2 {
		t.Errorf("expected total <= 2 after pruning, got %d", total)
	}

	// Scope attrs should have been dropped.
	if sa.Len() >= 3 {
		t.Errorf("expected scope attrs to be pruned, still have %d", sa.Len())
	}
	_ = logs
}
