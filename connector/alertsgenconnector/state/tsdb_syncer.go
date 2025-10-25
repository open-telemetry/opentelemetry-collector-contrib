// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package state // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/alertsgenconnector/state"

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"

	// Use the same gogo/protobuf that's already in the project
	"github.com/gogo/protobuf/proto"
)

// Entry is a snapshot of an alert's state stored in TSDB.
type Entry struct {
	Labels      map[string]string
	Active      bool
	ForDuration time.Duration
}

// AlertEvent represents an alert state change event
type AlertEvent struct {
	Rule        string            `json:"rule"`
	State       string            `json:"state"` // firing|resolved|no_data
	Severity    string            `json:"severity"`
	Labels      map[string]string `json:"labels"`
	Value       float64           `json:"value"`
	Window      string            `json:"window"`
	For         string            `json:"for"`
	Timestamp   time.Time         `json:"timestamp"`
	Fingerprint uint64            `json:"fingerprint"`
}

// TSDBSyncer does remote-read against Prometheus/VictoriaMetrics-compatible
// /api/v1/query endpoints to recover active alert state on startup,
// and publishes alert state via remote write API.
type TSDBSyncer struct {
	base        *url.URL
	client      *http.Client
	dedupWindow time.Duration
	remoteWrite *url.URL
	instanceID  string
	enableWrite bool
}

// TSDBConfig holds configuration for TSDB operations
type TSDBConfig struct {
	QueryURL       string        `mapstructure:"query_url"`
	RemoteWriteURL string        `mapstructure:"remote_write_url"`
	QueryTimeout   time.Duration `mapstructure:"query_timeout"`
	WriteTimeout   time.Duration `mapstructure:"write_timeout"`
	DedupWindow    time.Duration `mapstructure:"dedup_window"`
	InstanceID     string        `mapstructure:"instance_id"`
}

// NewTSDBSyncer builds a syncer for the given configuration
func NewTSDBSyncer(cfg TSDBConfig) (*TSDBSyncer, error) {
	if cfg.QueryURL == "" {
		return nil, errors.New("query_url required")
	}

	queryURL, err := url.Parse(cfg.QueryURL)
	if err != nil {
		return nil, fmt.Errorf("invalid query_url: %w", err)
	}

	var writeURL *url.URL
	enableWrite := false
	if cfg.RemoteWriteURL != "" {
		writeURL, err = url.Parse(cfg.RemoteWriteURL)
		if err != nil {
			return nil, fmt.Errorf("invalid remote_write_url: %w", err)
		}
		enableWrite = true
	}

	if cfg.QueryTimeout == 0 {
		cfg.QueryTimeout = 30 * time.Second
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = 10 * time.Second
	}
	if cfg.DedupWindow == 0 {
		cfg.DedupWindow = 30 * time.Second
	}
	if cfg.InstanceID == "" {
		cfg.InstanceID = "unknown"
	}

	cli := &http.Client{
		Timeout: cfg.QueryTimeout,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 20,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	return &TSDBSyncer{
		base:        queryURL,
		client:      cli,
		dedupWindow: cfg.DedupWindow,
		remoteWrite: writeURL,
		instanceID:  cfg.InstanceID,
		enableWrite: enableWrite,
	}, nil
}

// Legacy constructor for backward compatibility
func NewTSDBSyncerLegacy(baseURL string, queryTimeout, dedupWindow time.Duration) (*TSDBSyncer, error) {
	return NewTSDBSyncer(TSDBConfig{
		QueryURL:     baseURL,
		QueryTimeout: queryTimeout,
		DedupWindow:  dedupWindow,
		InstanceID:   "legacy",
	})
}

// QueryActive returns currently firing entries for a given rule_id by issuing
// an instant query: otel_alert_active{rule_id="<rule>"} != 0
func (s *TSDBSyncer) QueryActive(ruleID string) (map[uint64]Entry, error) {
	if s == nil || s.base == nil {
		return map[uint64]Entry{}, nil
	}

	q := fmt.Sprintf(`otel_alert_active{rule_id=%q} != 0`, ruleID)
	res, err := s.instantQuery(context.Background(), q)
	if err != nil {
		return nil, fmt.Errorf("failed to query active alerts: %w", err)
	}

	out := make(map[uint64]Entry, len(res))
	for _, r := range res {
		lbls := map[string]string{}
		var forDur time.Duration

		for k, v := range r.Metric {
			switch k {
			case "__name__", "rule_id", "severity", "instance":
				continue
			case "for_duration_seconds":
				if seconds, err := strconv.ParseFloat(v, 64); err == nil {
					forDur = time.Duration(seconds * float64(time.Second))
				}
			default:
				lbls[k] = v
			}
		}

		active := r.Value.Num() != "0"
		fp := fingerprint(ruleID, lbls)
		out[fp] = Entry{
			Labels:      lbls,
			Active:      active,
			ForDuration: forDur,
		}
	}
	return out, nil
}

// PublishEvents sends alert events to TSDB via remote write
func (s *TSDBSyncer) PublishEvents(events []interface{}) error {
	if s == nil || !s.enableWrite || len(events) == 0 {
		return nil
	}

	// Convert interface{} to AlertEvent
	alertEvents := make([]AlertEvent, 0, len(events))
	for _, e := range events {
		switch ae := e.(type) {
		case AlertEvent:
			alertEvents = append(alertEvents, ae)
		case map[string]interface{}:
			// Handle case where events come as generic maps
			if converted := s.convertMapToAlertEvent(ae); converted != nil {
				alertEvents = append(alertEvents, *converted)
			}
		default:
			// Skip unsupported event types
			continue
		}
	}

	if len(alertEvents) == 0 {
		return nil
	}

	return s.publishToRemoteWrite(alertEvents)
}

// convertMapToAlertEvent converts a generic map to AlertEvent
func (s *TSDBSyncer) convertMapToAlertEvent(m map[string]interface{}) *AlertEvent {
	ae := &AlertEvent{
		Timestamp: time.Now(),
		Labels:    make(map[string]string),
	}

	// Extract fields with type assertions
	if rule, ok := m["rule"].(string); ok {
		ae.Rule = rule
	}
	if state, ok := m["state"].(string); ok {
		ae.State = state
	}
	if severity, ok := m["severity"].(string); ok {
		ae.Severity = severity
	}
	if value, ok := m["value"].(float64); ok {
		ae.Value = value
	}
	if window, ok := m["window"].(string); ok {
		ae.Window = window
	}
	if forDur, ok := m["for"].(string); ok {
		ae.For = forDur
	}
	if fp, ok := m["fingerprint"].(uint64); ok {
		ae.Fingerprint = fp
	}

	// Extract labels
	if labels, ok := m["labels"].(map[string]string); ok {
		ae.Labels = labels
	} else if labels, ok := m["labels"].(map[string]interface{}); ok {
		for k, v := range labels {
			if str, ok := v.(string); ok {
				ae.Labels[k] = str
			}
		}
	}

	// Generate fingerprint if not provided
	if ae.Fingerprint == 0 && ae.Rule != "" {
		ae.Fingerprint = fingerprint(ae.Rule, ae.Labels)
	}

	return ae
}

// publishToRemoteWrite sends alert events via Prometheus remote write protocol
func (s *TSDBSyncer) publishToRemoteWrite(events []AlertEvent) error {
	if s.remoteWrite == nil {
		return errors.New("remote write URL not configured")
	}

	now := time.Now()
	timeseries := make([]prompb.TimeSeries, 0, len(events)*3) // 3 metrics per event

	for _, event := range events {
		// Create base labels for this alert
		baseLabels := []prompb.Label{
			{Name: "__name__", Value: "otel_alert_active"},
			{Name: "rule_id", Value: event.Rule},
			{Name: "severity", Value: event.Severity},
			{Name: "instance", Value: s.instanceID},
			{Name: "fingerprint", Value: fmt.Sprintf("%d", event.Fingerprint)},
		}

		// Add alert-specific labels
		for k, v := range event.Labels {
			baseLabels = append(baseLabels, prompb.Label{
				Name:  k,
				Value: v,
			})
		}

		// Sort labels for consistency
		sort.Slice(baseLabels, func(i, j int) bool {
			return baseLabels[i].Name < baseLabels[j].Name
		})

		// Determine metric value based on state
		var metricValue float64
		switch event.State {
		case "firing":
			metricValue = 1
		case "resolved", "no_data":
			metricValue = 0
		default:
			continue // Skip unknown states
		}

		// Create time series for alert state
		ts := prompb.TimeSeries{
			Labels: baseLabels,
			Samples: []prompb.Sample{
				{
					Value:     metricValue,
					Timestamp: now.UnixMilli(),
				},
			},
		}
		timeseries = append(timeseries, ts)

		// Create additional time series for alert value
		valueLabels := make([]prompb.Label, len(baseLabels))
		copy(valueLabels, baseLabels)
		// Change metric name for value series
		for i := range valueLabels {
			if valueLabels[i].Name == "__name__" {
				valueLabels[i].Value = "otel_alert_last_value"
				break
			}
		}

		valueSeries := prompb.TimeSeries{
			Labels: valueLabels,
			Samples: []prompb.Sample{
				{
					Value:     event.Value,
					Timestamp: now.UnixMilli(),
				},
			},
		}
		timeseries = append(timeseries, valueSeries)

		// Create time series for "for" duration tracking
		if event.For != "" {
			if forDuration, err := time.ParseDuration(event.For); err == nil {
				forLabels := make([]prompb.Label, len(baseLabels))
				copy(forLabels, baseLabels)
				// Change metric name for duration series
				for i := range forLabels {
					if forLabels[i].Name == "__name__" {
						forLabels[i].Value = "otel_alert_for_duration_seconds"
						break
					}
				}

				forSeries := prompb.TimeSeries{
					Labels: forLabels,
					Samples: []prompb.Sample{
						{
							Value:     forDuration.Seconds(),
							Timestamp: now.UnixMilli(),
						},
					},
				}
				timeseries = append(timeseries, forSeries)
			}
		}
	}

	if len(timeseries) == 0 {
		return nil
	}

	// Create write request using direct struct initialization (compatible with gogo/protobuf)
	writeReq := prompb.WriteRequest{
		Timeseries: timeseries,
	}

	// Marshal to protobuf
	data, err := proto.Marshal(&writeReq)
	if err != nil {
		return fmt.Errorf("failed to marshal write request: %w", err)
	}

	// Compress with snappy
	compressed := snappy.Encode(nil, data)

	// Create HTTP request
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", s.remoteWrite.String(), bytes.NewReader(compressed))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("Content-Encoding", "snappy")
	req.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")

	// Send request
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send remote write request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("remote write failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// ---- HTTP & response parsing ----

func (s *TSDBSyncer) instantQuery(ctx context.Context, query string) ([]vmResult, error) {
	u := *s.base
	u.Path = strings.TrimRight(u.Path, "/") + "/api/v1/query"
	v := url.Values{}
	v.Set("query", query)
	u.RawQuery = v.Encode()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out vmResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if out.Status != "success" {
		return nil, fmt.Errorf("tsdb query failed: %s", out.Error)
	}
	return out.Data.Result, nil
}

type vmResp struct {
	Status string      `json:"status"`
	Data   vmData      `json:"data"`
	Error  string      `json:"error,omitempty"`
	Warn   interface{} `json:"warnings,omitempty"`
}
type vmData struct {
	ResultType string     `json:"resultType"`
	Result     []vmResult `json:"result"`
}
type vmResult struct {
	Metric map[string]string `json:"metric"`
	Value  vmValue           `json:"value"`
}
type vmValue [2]any

func (v vmValue) Num() string {
	if len(v) != 2 {
		return "0"
	}
	if s, ok := v[1].(string); ok {
		return s
	}
	return "0"
}

// fingerprint matches the engine logic (FNV-1a).
func fingerprint(rule string, labels map[string]string) uint64 {
	h := fnv64a{}
	h.WriteString(rule)
	ks := make([]string, 0, len(labels))
	for k := range labels {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	for _, k := range ks {
		_ = h.WriteByte(0xff)
		h.WriteString(k)
		_ = h.WriteByte(0xff)
		h.WriteString(labels[k])
	}
	return h.Sum64()
}

// lightweight FNV-1a 64 implementation.
type fnv64a struct{ sum uint64 }

func (f *fnv64a) WriteByte(b byte) error {
	if f.sum == 0 {
		f.sum = 1469598103934665603
	}
	f.sum ^= uint64(b)
	f.sum *= 1099511628211
	return nil
}

func (f *fnv64a) WriteString(s string) {
	for i := 0; i < len(s); i++ {
		_ = f.WriteByte(s[i])
	}
}

func (f *fnv64a) Sum64() uint64 {
	if f.sum == 0 {
		return 1469598103934665603
	}
	return f.sum
}
