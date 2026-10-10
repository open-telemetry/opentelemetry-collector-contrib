package resourceuuidprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceuuidprocessor"

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type graphResponse struct {
	WorkloadDetails *struct {
		SchemaVersion int    `json:"schemaVersion"`
		ClusterName   string `json:"clusterName"`
		Node          *struct {
			Name       string            `json:"name"`
			Attributes map[string]string `json:"attributes"`
		} `json:"node"`
	} `json:"workloadDetails"`
	Nodes []struct {
		Moid            string `json:"moid"`
		UUID            string `json:"uuid"`
		Kind            string `json:"kind"`
		Name            string `json:"name"`
		Namespace       string `json:"namespace"`
		ObjectState     string `json:"objectState"`
		WorkloadDetails *struct {
			Attributes map[string]string `json:"attributes"`
			Owners     []struct {
				Kind         string `json:"kind"`
				Name         string `json:"name"`
				UID          string `json:"uid"`
				ResourceUUID string `json:"resourceUUID"`
			} `json:"owners"`
		} `json:"workloadDetails"`
	} `json:"nodes"`
}

type resourceMetadata struct {
	uuid       string
	attributes map[string]string
}

const nodeKeyPrefix = "\x00node:"
const maxResponseBytes = 16 << 20

type podClient struct {
	endpoint string
	http     *http.Client
}

func newPodClient(endpoint string, timeout time.Duration) *podClient {
	return &podClient{endpoint: endpoint, http: &http.Client{Timeout: timeout}}
}

// fetch reads the graph, optionally enriched with workloadDetails.
func (c *podClient) fetch(ctx context.Context) (pods map[string]resourceMetadata, unresolved int, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint, nil)
	if err != nil {
		return nil, 0, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, c.endpoint)
	}

	var gr graphResponse
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, 0, err
	}
	if len(data) > maxResponseBytes {
		return nil, 0, fmt.Errorf("metadata response exceeds %d bytes", maxResponseBytes)
	}
	if err = json.Unmarshal(data, &gr); err != nil {
		return nil, 0, fmt.Errorf("decoding response: %w", err)
	}

	if gr.Nodes == nil {
		return nil, 0, fmt.Errorf("response contains no graph nodes")
	}
	pods = make(map[string]resourceMetadata)
	details := gr.WorkloadDetails
	if details != nil {
		if details.SchemaVersion != 1 || details.ClusterName == "" || details.Node == nil || details.Node.Name == "" {
			return nil, 0, fmt.Errorf("invalid or unsupported workload details")
		}
		if details.Node.Attributes["k8s.node.name"] != details.Node.Name || details.Node.Attributes["k8s.cluster.name"] != details.ClusterName ||
			details.Node.Attributes["resourceUUID"] != details.Node.Attributes["k8s.node.resourceUUID"] {
			return nil, 0, fmt.Errorf("node metadata identity mismatch")
		}
		pods[nodeKeyPrefix+details.Node.Name] = resourceMetadata{uuid: details.Node.Attributes["resourceUUID"], attributes: details.Node.Attributes}
	}
	podCount := 0
	seenUIDs := make(map[string]bool)
	for _, n := range gr.Nodes {
		// kind is checked so the processor also works against an endpoint without the kind filter.
		if n.Kind != "" && n.Kind != "Pod" {
			continue
		}
		if details != nil && n.ObjectState == "DELETED" {
			continue
		}
		idx := strings.LastIndex(n.Moid, "_")
		if idx < 0 || idx == len(n.Moid)-1 {
			continue
		}
		uid := n.Moid[idx+1:]
		if strings.ContainsRune(uid, '\x00') {
			return nil, 0, fmt.Errorf("invalid pod UID")
		}
		podCount++
		if podCount > 10000 {
			return nil, 0, fmt.Errorf("metadata snapshot exceeds 10000 pods")
		}
		if seenUIDs[uid] {
			return nil, 0, fmt.Errorf("duplicate pod UID in graph")
		}
		seenUIDs[uid] = true
		var attributes map[string]string
		if n.WorkloadDetails != nil {
			if details == nil {
				return nil, 0, fmt.Errorf("pod workload details without response context")
			}
			attributes = n.WorkloadDetails.Attributes
			if attributes["k8s.pod.uid"] != uid || n.Moid != details.ClusterName+"_"+uid ||
				attributes["k8s.cluster.name"] != details.ClusterName || attributes["k8s.node.name"] != details.Node.Name ||
				n.Name == "" || attributes["k8s.pod.name"] != n.Name ||
				n.Namespace == "" || attributes["k8s.namespace.name"] != n.Namespace {
				return nil, 0, fmt.Errorf("pod metadata identity mismatch")
			}
			if attributes["resourceUUID"] != n.UUID || attributes["k8s.pod.resourceUUID"] != n.UUID {
				return nil, 0, fmt.Errorf("pod resource UUID mismatch")
			}
			seen := make(map[string]bool)
			for i, owner := range n.WorkloadDetails.Owners {
				if owner.Kind == "" || owner.Name == "" || (owner.UID == "" && owner.ResourceUUID != "") || seen[owner.Kind] {
					return nil, 0, fmt.Errorf("invalid workload owner identity")
				}
				seen[owner.Kind] = true
				if i == 0 {
					attributes["k8s.workload.type"] = owner.Kind
				}
				switch owner.Kind {
				case "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet", "Job", "CronJob", "ReplicationController", "Rollout":
					prefix := "k8s." + strings.ToLower(owner.Kind) + "."
					attributes[prefix+"name"] = owner.Name
					if owner.UID != "" {
						attributes[prefix+"uid"] = owner.UID
					}
					if owner.ResourceUUID != "" {
						attributes[prefix+"resourceUUID"] = owner.ResourceUUID
					}
				}
			}
		}
		if n.UUID == "" {
			unresolved++
		}
		if n.UUID != "" || attributes != nil {
			pods[uid] = resourceMetadata{uuid: n.UUID, attributes: attributes}
		}
	}
	return pods, unresolved, nil
}
