package resourceuuidprocessor

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func metadataSnapshot() map[string]any {
	return map[string]any{
		"workloadDetails": map[string]any{
			"schemaVersion": 1, "clusterName": "cluster",
			"node": map[string]any{
				"name": "node",
				"attributes": map[string]string{
					"k8s.cluster.name": "cluster", "k8s.cluster.resourceUUID": "cluster-uuid",
					"k8s.node.name": "node", "k8s.node.resourceUUID": "node-uuid",
					"resourceUUID": "node-uuid", "resourceName": "node",
				},
			},
		},
		"nodes": []map[string]any{{
			"moid": "cluster_uid-1", "uuid": "pod-uuid", "kind": "Pod", "name": "pod", "namespace": "default",
			"workloadDetails": map[string]any{
				"attributes": map[string]string{
					"k8s.cluster.name": "cluster", "k8s.cluster.resourceUUID": "cluster-uuid",
					"k8s.node.name": "node", "k8s.node.resourceUUID": "node-uuid",
					"k8s.namespace.name": "default", "k8s.pod.uid": "uid-1",
					"k8s.pod.name": "pod", "k8s.pod.ip": "10.0.0.1",
					"resourceUUID": "pod-uuid", "k8s.pod.resourceUUID": "pod-uuid",
					"resourceName": "pod",
				},
				"owners": []map[string]string{
					{"kind": "ReplicaSet", "name": "rs", "uid": "rs-uid", "resourceUUID": "rs-uuid"},
					{"kind": "Deployment", "name": "deployment", "uid": "deployment-uid", "resourceUUID": "deployment-uuid"},
				},
			},
		}},
	}
}

func snapshotPodDetails(s map[string]any) map[string]any {
	return s["nodes"].([]map[string]any)[0]["workloadDetails"].(map[string]any)
}

func TestMetadataEnrichmentAndNodeIsolation(t *testing.T) {
	snapshot := metadataSnapshot()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode(snapshot))
	}))
	defer srv.Close()
	p := newTestProcessor(t, srv.URL, func(c *Config) { c.CacheSize = 1 })
	p.refresh(context.Background())
	require.Equal(t, []string{"uid-1"}, p.cache.Keys())
	out, err := p.processLogs(context.Background(), logsFor("uid-1"))
	require.NoError(t, err)
	got := out.ResourceLogs().At(0).Resource().Attributes()
	for key, value := range snapshotPodDetails(snapshot)["attributes"].(map[string]string) {
		attr, ok := got.Get(key)
		require.True(t, ok, key)
		require.Equal(t, value, attr.Str(), key)
	}
	uuid, ok := uuidOf(out)
	require.True(t, ok)
	require.Equal(t, "pod-uuid", uuid)
	for key, value := range map[string]string{
		"k8s.replicaset.name": "rs", "k8s.replicaset.uid": "rs-uid", "k8s.replicaset.resourceUUID": "rs-uuid",
		"k8s.deployment.name": "deployment", "k8s.deployment.uid": "deployment-uid", "k8s.deployment.resourceUUID": "deployment-uuid",
		"k8s.workload.type": "ReplicaSet",
	} {
		attr, found := got.Get(key)
		require.True(t, found, key)
		require.Equal(t, value, attr.Str(), key)
	}

	// A node name must not cause unknown pods to inherit the node's resourceUUID.
	unknown := logsFor("new-pod")
	unknown.ResourceLogs().At(0).Resource().Attributes().PutStr("k8s.node.name", "node")
	out, err = p.processLogs(context.Background(), unknown)
	require.NoError(t, err)
	_, ok = out.ResourceLogs().At(0).Resource().Attributes().Get("resourceUUID")
	require.False(t, ok)

	nodeProcessor := newTestProcessor(t, srv.URL, func(c *Config) {
		c.NodeLogs = true
		c.CacheSize = 1
	})
	nodeProcessor.refresh(context.Background())
	require.Equal(t, []string{nodeKeyPrefix + "node"}, nodeProcessor.cache.Keys())
	nodeLogs := logsFor("")
	nodeLogs.ResourceLogs().At(0).Resource().Attributes().PutStr("k8s.node.name", "node")
	out, err = nodeProcessor.processLogs(context.Background(), nodeLogs)
	require.NoError(t, err)
	got = out.ResourceLogs().At(0).Resource().Attributes()
	nodeUUID, ok := got.Get("resourceUUID")
	require.True(t, ok)
	require.Equal(t, "node-uuid", nodeUUID.Str())
	_, ok = uuidOf(out)
	require.False(t, ok)
	_, ok = got.Get("k8s.deployment.name")
	require.False(t, ok)

	wrongCluster := logsFor("uid-1")
	wrongCluster.ResourceLogs().At(0).Resource().Attributes().PutStr("k8s.cluster.name", "other")
	out, err = p.processLogs(context.Background(), wrongCluster)
	require.NoError(t, err)
	_, ok = uuidOf(out)
	require.False(t, ok)
}

func TestMetadataWithoutCloudUUIDKeptUntilIdle(t *testing.T) {
	var mu sync.Mutex
	snapshot := metadataSnapshot()
	podAttrs := snapshotPodDetails(snapshot)["attributes"].(map[string]string)
	snapshot["nodes"].([]map[string]any)[0]["uuid"] = ""
	delete(podAttrs, "resourceUUID")
	delete(podAttrs, "k8s.pod.resourceUUID")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		require.NoError(t, json.NewEncoder(w).Encode(snapshot))
	}))
	defer srv.Close()
	p := newTestProcessor(t, srv.URL, nil)
	p.refresh(context.Background())
	out, err := p.processLogs(context.Background(), logsFor("uid-1"))
	require.NoError(t, err)
	name, ok := out.ResourceLogs().At(0).Resource().Attributes().Get("k8s.deployment.name")
	require.True(t, ok)
	require.Equal(t, "deployment", name.Str())
	_, ok = uuidOf(out)
	require.False(t, ok)
	require.True(t, p.missed.Load())

	mu.Lock()
	snapshot["nodes"] = []map[string]any{}
	mu.Unlock()
	p.refresh(context.Background())
	out, err = p.processLogs(context.Background(), logsFor("uid-1"))
	require.NoError(t, err)
	_, ok = out.ResourceLogs().At(0).Resource().Attributes().Get("k8s.deployment.name")
	require.True(t, ok, "absent pods are dropped by idle expiry, not by snapshots")
}

func TestMalformedMetadataPreservesCache(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"version", func(s map[string]any) { s["workloadDetails"].(map[string]any)["schemaVersion"] = 2 }},
		{"missing pods", func(s map[string]any) { delete(s, "nodes") }},
		{"missing context", func(s map[string]any) { delete(s, "workloadDetails") }},
		{"missing node", func(s map[string]any) { delete(s["workloadDetails"].(map[string]any), "node") }},
		{"wrong UID", func(s map[string]any) { s["nodes"].([]map[string]any)[0]["moid"] = "cluster_different" }},
		{"duplicate UID", func(s map[string]any) {
			pods := s["nodes"].([]map[string]any)
			s["nodes"] = append(pods, pods[0])
		}},
		{"wrong cloud UUID", func(s map[string]any) {
			snapshotPodDetails(s)["attributes"].(map[string]string)["resourceUUID"] = "wrong"
		}},
		{"wrong node UUID", func(s map[string]any) {
			s["workloadDetails"].(map[string]any)["node"].(map[string]any)["attributes"].(map[string]string)["resourceUUID"] = "wrong"
		}},
		{"owner UUID without UID", func(s map[string]any) {
			delete(snapshotPodDetails(s)["owners"].([]map[string]string)[0], "uid")
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var mu sync.Mutex
			snapshot := metadataSnapshot()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				require.NoError(t, json.NewEncoder(w).Encode(snapshot))
			}))
			defer srv.Close()
			p := newTestProcessor(t, srv.URL, nil)
			p.refresh(context.Background())
			mu.Lock()
			test.mutate(snapshot)
			mu.Unlock()
			p.refresh(context.Background())
			require.Equal(t, 1, p.failures)
			out, err := p.processLogs(context.Background(), logsFor("uid-1"))
			require.NoError(t, err)
			uuid, ok := uuidOf(out)
			require.True(t, ok)
			require.Equal(t, "pod-uuid", uuid)
		})
	}

}

func TestOptionalWorkloadDetailsAndDeletedPods(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(map[string]any)
		uuid   bool
		owner  bool
	}{
		{"enabled", func(map[string]any) {}, true, true},
		{"disabled", func(s map[string]any) {
			delete(s, "workloadDetails")
			delete(s["nodes"].([]map[string]any)[0], "workloadDetails")
		}, true, false},
		{"repository miss", func(s map[string]any) {
			delete(s["nodes"].([]map[string]any)[0], "workloadDetails")
		}, true, false},
		{"deleted", func(s map[string]any) {
			s["nodes"].([]map[string]any)[0]["objectState"] = "DELETED"
		}, false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			snapshot := metadataSnapshot()
			test.mutate(snapshot)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				require.NoError(t, json.NewEncoder(w).Encode(snapshot))
			}))
			defer srv.Close()
			p := newTestProcessor(t, srv.URL, nil)
			p.refresh(context.Background())
			require.Zero(t, p.failures)
			out, err := p.processLogs(context.Background(), logsFor("uid-1"))
			require.NoError(t, err)
			_, hasUUID := uuidOf(out)
			require.Equal(t, test.uuid, hasUUID)
			_, hasOwner := out.ResourceLogs().At(0).Resource().Attributes().Get("k8s.deployment.resourceUUID")
			require.Equal(t, test.owner, hasOwner)
		})
	}
}

func TestMetadataResponseLimits(t *testing.T) {
	for _, test := range []struct {
		name string
		body func() string
		want string
	}{
		{"bytes", func() string { return strings.Repeat(" ", maxResponseBytes+1) }, "exceeds 16777216 bytes"},
		{"pod count", func() string {
			snapshot := metadataSnapshot()
			pods := make([]map[string]any, 10001)
			for i := range pods {
				pods[i] = map[string]any{"moid": fmt.Sprintf("cluster_uid-%d", i), "uuid": "uuid", "kind": "Pod"}
			}

			snapshot["nodes"] = pods
			data, err := json.Marshal(snapshot)
			require.NoError(t, err)
			return string(data)
		}, "exceeds 10000 pods"},
		{"trailing JSON", func() string { return graphJSON + `{}` }, "decoding response"},
		{"empty object", func() string { return `{}` }, "no graph nodes"},
	} {
		t.Run(test.name, func(t *testing.T) {
			body := test.body()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(body))
			}))
			defer srv.Close()
			_, _, err := newPodClient(srv.URL, time.Second).fetch(context.Background())
			require.ErrorContains(t, err, test.want)
		})
	}
}
