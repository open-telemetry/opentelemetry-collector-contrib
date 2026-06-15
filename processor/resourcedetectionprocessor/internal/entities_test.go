// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	xentity "go.opentelemetry.io/collector/pdata/xpdata/entity"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/metadata"
)

func TestResolveEntities(t *testing.T) {
	tests := []struct {
		name     string
		entities []DetectedEntity
		want     map[string]string
	}{
		{
			name:     "global entity has no context",
			entities: []DetectedEntity{{Type: "host", IDKeys: []string{"host.id"}}},
			want:     map[string]string{"host": ""},
		},
		{
			name: "candidate present",
			entities: []DetectedEntity{
				{Type: "k8s.node", IDKeys: []string{"k8s.node.name"}, IDContextTypeCandidates: []string{"k8s.cluster"}},
				{Type: "k8s.cluster", IDKeys: []string{"k8s.cluster.uid"}},
			},
			want: map[string]string{"k8s.node": "k8s.cluster", "k8s.cluster": ""},
		},
		{
			name: "first present candidate wins",
			entities: []DetectedEntity{
				{Type: "process", IDKeys: []string{"process.pid"}, IDContextTypeCandidates: []string{"container", "host"}},
				{Type: "host", IDKeys: []string{"host.id"}},
			},
			want: map[string]string{"process": "host", "host": ""},
		},
		{
			name: "same type keeps first identity",
			entities: []DetectedEntity{
				{Type: "k8s.cluster", IDKeys: []string{"k8s.cluster.uid"}},
				{Type: "k8s.cluster", IDKeys: []string{"k8s.cluster.name"}, IDContextTypeCandidates: []string{"cloud.account"}},
				{Type: "cloud.account", IDKeys: []string{"cloud.account.id"}},
			},
			want: map[string]string{"k8s.cluster": "", "cloud.account": ""},
		},
		{
			name: "cycle is broken deterministically",
			entities: []DetectedEntity{
				{Type: "a", IDKeys: []string{"a.id"}, IDContextTypeCandidates: []string{"b"}},
				{Type: "b", IDKeys: []string{"b.id"}, IDContextTypeCandidates: []string{"a"}},
			},
			want: map[string]string{"a": "", "b": "a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := contextByType(resolveEntities(tt.entities, zap.NewNop()))
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResolveEntitiesOrderInvariance(t *testing.T) {
	entities := []DetectedEntity{
		{Type: "k8s.node", IDKeys: []string{"k8s.node.name"}, IDContextTypeCandidates: []string{"k8s.cluster"}},
		{Type: "k8s.cluster", IDKeys: []string{"k8s.cluster.name"}, IDContextTypeCandidates: []string{"cloud.account"}},
		{Type: "cloud.account", IDKeys: []string{"cloud.account.id"}},
		{Type: "host", IDKeys: []string{"host.id"}},
	}
	want := map[string]string{
		"k8s.node":      "k8s.cluster",
		"k8s.cluster":   "cloud.account",
		"cloud.account": "",
		"host":          "",
	}

	for _, perm := range permutations(len(entities)) {
		permuted := make([]DetectedEntity, 0, len(entities))
		for _, i := range perm {
			permuted = append(permuted, entities[i])
		}
		assert.Equal(t, want, contextByType(resolveEntities(permuted, zap.NewNop())), "permutation %v", perm)
	}
}

func TestAppendEntityRefs(t *testing.T) {
	res := pcommon.NewResource()
	res.Attributes().PutStr("k8s.node.name", "node-1")
	res.Attributes().PutStr("k8s.cluster.name", "cluster-1")

	appendEntityRefs(res, []DetectedEntity{
		{
			SchemaURL:       "https://test",
			Type:            "k8s.node",
			IDKeys:          []string{"k8s.node.name"},
			DescriptionKeys: []string{"missing", "k8s.cluster.name"},
			idContextType:   "k8s.cluster",
		},
		{Type: "host", IDKeys: []string{"host.id"}},
	}, zap.NewNop())

	refs := xentity.ResourceEntityRefs(res)
	require.Equal(t, 1, refs.Len())
	ref := refs.At(0)
	assert.Equal(t, "k8s.node", ref.Type())
	assert.Equal(t, "https://test", ref.SchemaUrl())
	assert.Equal(t, []string{"k8s.node.name"}, ref.IdKeys().AsRaw())
	assert.Equal(t, []string{"k8s.cluster.name"}, ref.DescriptionKeys().AsRaw())
	assert.Equal(t, "k8s.cluster", ref.IdContextType())
}

func TestMergeEntityRefs(t *testing.T) {
	from := pcommon.NewResource()
	from.Attributes().PutStr("k8s.node.name", "node-1")
	from.Attributes().PutStr("host.id", "host-1")
	fromRefs := xentity.ResourceEntityRefs(from)
	node := fromRefs.AppendEmpty()
	node.SetType("k8s.node")
	node.IdKeys().Append("k8s.node.name")
	node.SetIdContextType("k8s.cluster")
	host := fromRefs.AppendEmpty()
	host.SetType("host")
	host.IdKeys().Append("host.id")

	to := pcommon.NewResource()
	to.Attributes().PutStr("k8s.node.name", "node-1")
	existing := xentity.ResourceEntityRefs(to).AppendEmpty()
	existing.SetType("k8s.node")
	existing.IdKeys().Append("k8s.node.name")

	MergeEntityRefs(to, from, false)

	toRefs := xentity.ResourceEntityRefs(to)
	require.Equal(t, 1, toRefs.Len())
	assert.Equal(t, "k8s.node", toRefs.At(0).Type())
	assert.Empty(t, toRefs.At(0).IdContextType())

	MergeEntityRefs(to, from, true)
	toRefs = xentity.ResourceEntityRefs(to)
	require.Equal(t, 1, toRefs.Len())
	assert.Equal(t, "k8s.node", toRefs.At(0).Type())
	assert.Equal(t, "k8s.cluster", toRefs.At(0).IdContextType())
}

func TestDetectResourceEntities(t *testing.T) {
	newK8sDetector := func() Detector {
		res := pcommon.NewResource()
		res.Attributes().PutStr("k8s.node.name", "node-1")
		return &mockEntityDetector{res: res, entities: []DetectedEntity{{
			Type:                    "k8s.node",
			IDKeys:                  []string{"k8s.node.name"},
			IDContextTypeCandidates: []string{"k8s.cluster"},
		}}}
	}
	newCloudDetector := func() Detector {
		res := pcommon.NewResource()
		res.Attributes().PutStr("cloud.account.id", "project-1")
		res.Attributes().PutStr("k8s.cluster.name", "cluster-1")
		return &mockEntityDetector{res: res, entities: []DetectedEntity{
			{Type: "cloud.account", IDKeys: []string{"cloud.account.id"}},
			{Type: "k8s.cluster", IDKeys: []string{"k8s.cluster.name"}, IDContextTypeCandidates: []string{"cloud.account"}},
		}}
	}

	for _, order := range [][]string{{"k8s", "cloud"}, {"cloud", "k8s"}} {
		t.Run(fmt.Sprintf("order=%v", order), func(t *testing.T) {
			factories := map[DetectorType]DetectorFactory{
				"k8s":   func(processor.Settings, DetectorConfig) (Detector, error) { return newK8sDetector(), nil },
				"cloud": func(processor.Settings, DetectorConfig) (Detector, error) { return newCloudDetector(), nil },
			}
			types := make([]DetectorType, len(order))
			for i, typ := range order {
				types[i] = DetectorType(typ)
			}

			provider, err := NewProviderFactory(factories).CreateResourceProvider(
				processortest.NewNopSettings(metadata.Type), time.Second, &mockDetectorConfig{}, types...)
			require.NoError(t, err)
			require.NoError(t, provider.Refresh(context.Background(), &http.Client{Timeout: time.Second}))

			res, _, err := provider.Get(context.Background(), nil)
			require.NoError(t, err)
			assert.Equal(t, map[string]string{
				"k8s.node":      "k8s.cluster",
				"k8s.cluster":   "cloud.account",
				"cloud.account": "",
			}, contextByRef(res))
		})
	}
}

type mockEntityDetector struct {
	res      pcommon.Resource
	entities []DetectedEntity
}

func (d *mockEntityDetector) Detect(context.Context) (pcommon.Resource, string, error) {
	return d.res, "", nil
}

func (d *mockEntityDetector) EntityRefs(pcommon.Resource) []DetectedEntity {
	return d.entities
}

func contextByType(entities []DetectedEntity) map[string]string {
	got := make(map[string]string, len(entities))
	for _, ent := range entities {
		got[ent.Type] = ent.idContextType
	}
	return got
}

func contextByRef(res pcommon.Resource) map[string]string {
	refs := xentity.ResourceEntityRefs(res)
	got := make(map[string]string, refs.Len())
	for i := 0; i < refs.Len(); i++ {
		got[refs.At(i).Type()] = refs.At(i).IdContextType()
	}
	return got
}

func permutations(n int) [][]int {
	var res [][]int
	perm := make([]int, n)
	for i := range perm {
		perm[i] = i
	}
	var generate func(int)
	generate = func(k int) {
		if k == 1 {
			res = append(res, append([]int(nil), perm...))
			return
		}
		for i := 0; i < k; i++ {
			generate(k - 1)
			if k%2 == 0 {
				perm[i], perm[k-1] = perm[k-1], perm[i]
			} else {
				perm[0], perm[k-1] = perm[k-1], perm[0]
			}
		}
	}
	generate(n)
	return res
}
