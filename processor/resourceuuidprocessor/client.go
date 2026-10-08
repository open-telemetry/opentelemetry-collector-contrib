package resourceuuidprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceuuidprocessor"

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type graphResponse struct {
	Nodes []struct {
		Moid string `json:"moid"`
		UUID string `json:"uuid"`
		Kind string `json:"kind"`
	} `json:"nodes"`
}

type podClient struct {
	endpoint string
	http     *http.Client
}

func newPodClient(endpoint string, timeout time.Duration) *podClient {
	return &podClient{endpoint: endpoint, http: &http.Client{Timeout: timeout}}
}

// fetch returns podUid -> uuid for every pod that already has a uuid. A pod's moid is
// <clusterName>_<podUid> and a k8s uid never contains an underscore.
func (c *podClient) fetch(ctx context.Context) (pods map[string]string, unresolved int, err error) {
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
	if err = json.NewDecoder(resp.Body).Decode(&gr); err != nil {
		return nil, 0, fmt.Errorf("decoding response: %w", err)
	}

	pods = make(map[string]string, len(gr.Nodes))
	for _, n := range gr.Nodes {
		// kind is checked so the processor also works against an endpoint without the kind filter.
		if n.Kind != "" && n.Kind != "Pod" {
			continue
		}
		idx := strings.LastIndex(n.Moid, "_")
		if idx < 0 || idx == len(n.Moid)-1 {
			continue
		}
		if n.UUID == "" {
			unresolved++
			continue
		}
		pods[n.Moid[idx+1:]] = n.UUID
	}
	return pods, unresolved, nil
}
