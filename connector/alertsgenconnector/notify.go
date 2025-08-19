package alertsgenconnector

import (
    "bytes"
    "encoding/json"
    "net/http"
    "strconv"
    "time"
)

type AlertManager struct {
    urls []string
    timeout time.Duration
}

func NewAlertManager(urls []string, timeout time.Duration) *AlertManager {
    return &AlertManager{urls: urls, timeout: timeout}
}

type amAlert struct {
    Labels map[string]string `json:"labels"`
    Annotations map[string]string `json:"annotations"`
    StartsAt time.Time `json:"startsAt"`
    EndsAt   time.Time `json:"endsAt"`
    GeneratorURL string `json:"generatorURL"`
}

func (n *AlertManager) Notify(events []alertEvent) error {
    alerts := make([]amAlert, 0, len(events))
    now := time.Now()
    for _, e := range events {
        lab := map[string]string{
            "alertname": e.Rule,
            "severity": e.Severity,
        }
        for k,v := range e.Labels { lab[k]=v }
        ann := map[string]string{
            "summary": e.Rule + " " + e.State,
            "value":   strconv.FormatFloat(e.Value, 'f', -1, 64),
            "window":  e.Window,
            "for":     e.For,
        }
        al := amAlert{ Labels: lab, Annotations: ann, StartsAt: now, GeneratorURL: "otel/alertsgen" }
        if e.State == "resolved" { al.EndsAt = now }
        alerts = append(alerts, al)
    }
    if len(alerts) == 0 { return nil }
    payload, _ := json.Marshal(alerts)
    client := &http.Client{ Timeout: n.timeout }
    for _, u := range n.urls {
        req, _ := http.NewRequest("POST", u, bytes.NewReader(payload))
        req.Header.Set("Content-Type", "application/json")
        _, _ = client.Do(req) // best-effort
    }
    return nil
}
