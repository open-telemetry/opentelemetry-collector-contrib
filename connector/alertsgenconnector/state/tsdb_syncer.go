package state

import (
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "time"
)

type TSDBSyncer struct {
    QueryURL string
    InstanceID string
    QueryInterval time.Duration
    TakeoverTimeout time.Duration
}

type ActiveEntry struct {
    Active bool
    Labels map[string]string
    ForDuration time.Duration
}

// QueryActive returns active states (best effort) keyed by a uint64 fingerprint represented via a string key in practice.
func (s *TSDBSyncer) QueryActive(rule string) (map[uint64]ActiveEntry, error) {
    // We expect a metric like otel_alerts_active{rule="...", severity="...", ...} 1
    q := fmt.Sprintf(`max by (rule,fingerprint) (otel_alerts_active{rule="%s"})`, rule)
    params := url.Values{ "query": { q } }
    u, _ := url.Parse(s.QueryURL)
    u.Path = "/api/v1/query"
    u.RawQuery = params.Encode()

    req, _ := http.NewRequest("GET", u.String(), nil)
    client := &http.Client{ Timeout: 3*time.Second }
    resp, err := client.Do(req)
    if err != nil { return map[uint64]ActiveEntry{}, nil }
    defer resp.Body.Close()
    body, _ := io.ReadAll(resp.Body)

    var pr promResp
    if err := json.Unmarshal(body, &pr); err != nil { return map[uint64]ActiveEntry{}, nil }
    out := map[uint64]ActiveEntry{}
    for _, r := range pr.Data.Result {
        labels := r.Metric
        active := r.Value[1] == "1"
        out[fpFromLabels(labels)] = ActiveEntry{ Active: active, Labels: labels, ForDuration: 0 }
    }
    return out, nil
}

type promResp struct {
    Status string `json:"status"`
    Data struct{
        ResultType string `json:"resultType"`
        Result []struct{
            Metric map[string]string `json:"metric"`
            Value [2]string `json:"value"`
        } `json:"result"`
    } `json:"data"`
}

func fpFromLabels(labels map[string]string) uint64 {
    // naive hash: we don't need exact match for restore, just grouping
    var h uint64 = 1469598103934665603
    keys := make([]string,0,len(labels))
    for k := range labels { keys = append(keys, k) }
    // do not import sort here; minimal best effort
    for _, k := range keys {
        for i:=0;i<len(k);i++ { h ^= uint64(k[i]); h *= 1099511628211 }
        v := labels[k]
        for i:=0;i<len(v);i++ { h ^= uint64(v[i]); h *= 1099511628211 }
    }
    return h
}
