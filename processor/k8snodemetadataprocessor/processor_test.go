// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8snodemetadataprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap/zaptest"
	api_v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

func newTestProcessor(t *testing.T) *k8sNodeMetadataProcessor {
	t.Helper()
	p, err := newProcessor(&Config{}, zaptest.NewLogger(t))
	require.NoError(t, err)
	return p
}

func TestProcessMetrics_NoNodeName(t *testing.T) {
	p := newTestProcessor(t)
	p.nodes["node1"] = &nodeTaints{attrs: map[string]string{"k8s.node.taint.key1": "val1"}}

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("other.attr", "x")
	rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName("test")

	result, err := p.processMetrics(context.Background(), md)
	require.NoError(t, err)

	attrs := result.ResourceMetrics().At(0).Resource().Attributes()
	_, exists := attrs.Get("k8s.node.taint.key1")
	assert.False(t, exists, "should not add taints when k8s.node.name is missing")
}

func TestProcessMetrics_NodeNotInCache(t *testing.T) {
	p := newTestProcessor(t)

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("k8s.node.name", "unknown-node")
	rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName("test")

	result, err := p.processMetrics(context.Background(), md)
	require.NoError(t, err)

	attrs := result.ResourceMetrics().At(0).Resource().Attributes()
	assert.Equal(t, 1, attrs.Len(), "should only have k8s.node.name")
}

func TestProcessMetrics_AddsTaints(t *testing.T) {
	p := newTestProcessor(t)
	p.nodes["node1"] = &nodeTaints{attrs: map[string]string{
		"k8s.node.taint.dedicated":                    "gpu",
		"k8s.node.taint.node.kubernetes.io/not-ready": "",
	}}

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("k8s.node.name", "node1")
	rm.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName("test")

	result, err := p.processMetrics(context.Background(), md)
	require.NoError(t, err)

	attrs := result.ResourceMetrics().At(0).Resource().Attributes()

	val, ok := attrs.Get("k8s.node.taint.dedicated")
	require.True(t, ok)
	assert.Equal(t, "gpu", val.Str())

	val, ok = attrs.Get("k8s.node.taint.node.kubernetes.io/not-ready")
	require.True(t, ok)
	assert.Equal(t, "", val.Str())
}

func TestProcessMetrics_MultipleResourceMetrics(t *testing.T) {
	p := newTestProcessor(t)
	p.nodes["node1"] = &nodeTaints{attrs: map[string]string{"k8s.node.taint.key1": "val1"}}
	p.nodes["node2"] = &nodeTaints{attrs: map[string]string{"k8s.node.taint.key2": "val2"}}

	md := pmetric.NewMetrics()

	rm1 := md.ResourceMetrics().AppendEmpty()
	rm1.Resource().Attributes().PutStr("k8s.node.name", "node1")
	rm1.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName("m1")

	rm2 := md.ResourceMetrics().AppendEmpty()
	rm2.Resource().Attributes().PutStr("k8s.node.name", "node2")
	rm2.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetName("m2")

	result, err := p.processMetrics(context.Background(), md)
	require.NoError(t, err)

	val, ok := result.ResourceMetrics().At(0).Resource().Attributes().Get("k8s.node.taint.key1")
	require.True(t, ok)
	assert.Equal(t, "val1", val.Str())

	val, ok = result.ResourceMetrics().At(1).Resource().Attributes().Get("k8s.node.taint.key2")
	require.True(t, ok)
	assert.Equal(t, "val2", val.Str())
}

func TestHandleNodeAdd(t *testing.T) {
	p := newTestProcessor(t)

	node := &api_v1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node1"},
		Spec: api_v1.NodeSpec{
			Taints: []api_v1.Taint{
				{Key: "dedicated", Value: "gpu", Effect: api_v1.TaintEffectNoSchedule},
				{Key: "special", Value: "", Effect: api_v1.TaintEffectNoExecute},
			},
		},
	}

	p.handleNodeAdd(node)

	p.mu.RLock()
	defer p.mu.RUnlock()
	require.Contains(t, p.nodes, "node1")
	assert.Equal(t, "gpu", p.nodes["node1"].attrs["k8s.node.taint.dedicated"])
	assert.Equal(t, "", p.nodes["node1"].attrs["k8s.node.taint.special"])
}

func TestHandleNodeUpdate(t *testing.T) {
	p := newTestProcessor(t)

	oldNode := &api_v1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node1"},
		Spec: api_v1.NodeSpec{
			Taints: []api_v1.Taint{
				{Key: "old", Value: "val", Effect: api_v1.TaintEffectNoSchedule},
			},
		},
	}
	p.handleNodeAdd(oldNode)

	newNode := &api_v1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node1"},
		Spec: api_v1.NodeSpec{
			Taints: []api_v1.Taint{
				{Key: "new", Value: "val2", Effect: api_v1.TaintEffectNoSchedule},
			},
		},
	}
	p.handleNodeUpdate(oldNode, newNode)

	p.mu.RLock()
	defer p.mu.RUnlock()
	assert.NotContains(t, p.nodes["node1"].attrs, "k8s.node.taint.old")
	assert.Equal(t, "val2", p.nodes["node1"].attrs["k8s.node.taint.new"])
}

func TestHandleNodeDelete(t *testing.T) {
	p := newTestProcessor(t)
	p.nodes["node1"] = &nodeTaints{attrs: map[string]string{"k8s.node.taint.k": "v"}}

	node := &api_v1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node1"},
	}
	p.handleNodeDelete(node)

	p.mu.RLock()
	defer p.mu.RUnlock()
	assert.NotContains(t, p.nodes, "node1")
}

func TestHandleNodeDelete_Tombstone(t *testing.T) {
	p := newTestProcessor(t)
	p.nodes["node1"] = &nodeTaints{attrs: map[string]string{"k8s.node.taint.k": "v"}}

	node := &api_v1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node1"},
	}
	tombstone := cache.DeletedFinalStateUnknown{Obj: node}
	p.handleNodeDelete(tombstone)

	p.mu.RLock()
	defer p.mu.RUnlock()
	assert.NotContains(t, p.nodes, "node1")
}

func TestDuplicateTaintKeys_FirstWins(t *testing.T) {
	p := newTestProcessor(t)

	node := &api_v1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node1"},
		Spec: api_v1.NodeSpec{
			Taints: []api_v1.Taint{
				{Key: "dedicated", Value: "gpu", Effect: api_v1.TaintEffectNoSchedule},
				{Key: "dedicated", Value: "gpu", Effect: api_v1.TaintEffectNoExecute},
			},
		},
	}

	p.handleNodeAdd(node)

	p.mu.RLock()
	defer p.mu.RUnlock()
	assert.Equal(t, "gpu", p.nodes["node1"].attrs["k8s.node.taint.dedicated"])
	assert.Len(t, p.nodes["node1"].attrs, 1)
}

func TestResolveLocalNodeName_FromEnv(t *testing.T) {
	t.Setenv("K8S_NODE_NAME", "my-node")
	name, err := resolveLocalNodeName()
	require.NoError(t, err)
	assert.Equal(t, "my-node", name)
}

func TestResolveLocalNodeName_FallsBackToHostname(t *testing.T) {
	t.Setenv("K8S_NODE_NAME", "")
	name, err := resolveLocalNodeName()
	require.NoError(t, err)
	assert.NotEmpty(t, name)
}

func TestShutdown_CalledTwice_NoPanic(t *testing.T) {
	p := newTestProcessor(t)
	require.NoError(t, p.Shutdown(context.Background()))
	require.NoError(t, p.Shutdown(context.Background()))
}
