// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package k8sobserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func TestBuildCRDListerWatchersOneInformerPerTargetNamespace(t *testing.T) {
	t.Parallel()

	client := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	cfg := &Config{
		Namespaces: []string{"team-alpha", "team-beta"},
		ObserveCRDs: []CRDConfig{
			{Group: "widgets.example.com", Version: "v1", Kind: "Widget"},
		},
	}

	watchers := buildCRDListerWatchers(client, cfg, nil)
	require.Len(t, watchers, 2, "expect one lister/watcher per configured namespace")

	var gotNS []string
	for _, w := range watchers {
		dlw, ok := w.listerWatcher.(*dynamicListerWatcher)
		require.True(t, ok, "expected *dynamicListerWatcher")
		gotNS = append(gotNS, dlw.namespace)
	}
	assert.ElementsMatch(t, []string{"team-alpha", "team-beta"}, gotNS)
}

func TestBuildCRDListerWatchersMultipleCRDsMultipliesByNamespaces(t *testing.T) {
	t.Parallel()

	client := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	cfg := &Config{
		Namespaces: []string{"ns-a", "ns-b"},
		ObserveCRDs: []CRDConfig{
			{Group: "one.example.com", Version: "v1", Kind: "One"},
			{Group: "two.example.com", Version: "v1", Kind: "Two"},
		},
	}

	watchers := buildCRDListerWatchers(client, cfg, nil)
	require.Len(t, watchers, 4)

	type key struct {
		gvk       string
		namespace string
	}
	seen := make(map[key]struct{})
	for _, w := range watchers {
		dlw := w.listerWatcher.(*dynamicListerWatcher)
		gvk := w.gvk.Group + "/" + w.gvk.Version + ", Kind=" + w.gvk.Kind
		k := key{gvk: gvk, namespace: dlw.namespace}
		_, dup := seen[k]
		require.False(t, dup, "duplicate watcher for %v in ns %q", w.gvk, dlw.namespace)
		seen[k] = struct{}{}
	}

	for _, gvk := range []string{"one.example.com/v1, Kind=One", "two.example.com/v1, Kind=Two"} {
		for _, ns := range []string{"ns-a", "ns-b"} {
			_, ok := seen[key{gvk: gvk, namespace: ns}]
			assert.True(t, ok, "missing watcher for %s in %s", gvk, ns)
		}
	}
}

func TestBuildCRDListerWatchersEmptyNamespacesUsesAllNamespacesInformer(t *testing.T) {
	t.Parallel()

	client := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	cfg := &Config{
		Namespaces: nil,
		ObserveCRDs: []CRDConfig{
			{Group: "widgets.example.com", Version: "v1", Kind: "Widget"},
		},
	}

	watchers := buildCRDListerWatchers(client, cfg, nil)
	require.Len(t, watchers, 1)
	dlw := watchers[0].listerWatcher.(*dynamicListerWatcher)
	assert.Equal(t, v1.NamespaceAll, dlw.namespace)
}
