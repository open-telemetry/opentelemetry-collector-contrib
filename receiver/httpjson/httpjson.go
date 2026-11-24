// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package httpjson // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/httpjson"

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

type httpjsonReceiver struct {
	cfg      *Config
	consumer consumer.Logs
	client   *http.Client
	logger   *zap.Logger

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func newHTTPJSONReceiver(cfg *Config, next consumer.Logs, logger *zap.Logger) *httpjsonReceiver {
	return &httpjsonReceiver{
		cfg:      cfg,
		consumer: next,
		client:   &http.Client{},
		logger:   logger,
	}
}

func (r *httpjsonReceiver) Start(ctx context.Context, _ component.Host) error {
	runCtx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel

	delay := r.cfg.InitialDelay
	if delay <= 0 {
		delay = r.cfg.Interval / 2
	}

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		t := time.NewTimer(delay)
		defer t.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-t.C:
				r.pollOnce(runCtx)
				t.Reset(r.cfg.Interval)
			}
		}
	}()
	return nil
}

func (r *httpjsonReceiver) Shutdown(_ context.Context) error {
	if r.cancel != nil {
		r.cancel()
	}
	r.wg.Wait()
	return nil
}

func (r *httpjsonReceiver) newRequest(ctx context.Context) (*http.Request, error) {
	full := r.cfg.Endpoint
	if len(r.cfg.Query) > 0 {
		u, err := url.Parse(r.cfg.Endpoint)
		if err != nil {
			return nil, fmt.Errorf("endpoint is not a valid URL: %w", err)
		}
		q := u.Query()
		for k, v := range r.cfg.Query {
			q.Set(k, v)
		}
		u.RawQuery = q.Encode()
		full = u.String()
	}

	var body io.Reader
	if r.cfg.Body != "" {
		body = bytes.NewReader([]byte(r.cfg.Body))
	}
	req, err := http.NewRequestWithContext(ctx, r.cfg.Method, full, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	for k, v := range r.cfg.Headers {
		req.Header.Set(k, v)
	}
	if r.cfg.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+r.cfg.BearerToken)
	}
	if r.cfg.BasicAuth.Username != "" || r.cfg.BasicAuth.Password != "" {
		req.SetBasicAuth(r.cfg.BasicAuth.Username, r.cfg.BasicAuth.Password)
	}
	return req, nil
}

func (r *httpjsonReceiver) pollOnce(ctx context.Context) {
	req, err := r.newRequest(ctx)
	if err != nil {
		r.logger.Error("httpjson: failed to build request", zap.Error(err))
		return
	}

	resp, err := r.client.Do(req)
	if err != nil {
		r.logger.Error("httpjson: request failed", zap.String("endpoint", r.cfg.Endpoint), zap.Error(err))
		return
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024))
	if err != nil {
		r.logger.Error("httpjson: failed to read response", zap.Error(err))
		return
	}
	if resp.StatusCode >= 300 {
		r.logger.Warn("httpjson: unexpected status",
			zap.Int("status", resp.StatusCode), zap.String("endpoint", r.cfg.Endpoint))
		return
	}

	var parsed interface{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		r.logger.Error("httpjson: response is not valid JSON", zap.Error(err))
		return
	}

	logs := r.buildLogs(parsed)
	if logs.LogRecordCount() == 0 {
		return
	}
	if err := r.consumer.ConsumeLogs(ctx, logs); err != nil {
		r.logger.Error("httpjson: failed to consume logs", zap.Error(err))
	}
}

// resolveArrayPath walks a dot-delimited path into a decoded JSON document
// and returns the array the path lands on. Empty path returns the document.
func (r *httpjsonReceiver) resolveArrayPath(doc interface{}) ([]interface{}, bool) {
	if r.cfg.ArrayPath == "" {
		if arr, ok := doc.([]interface{}); ok {
			return arr, true
		}
		return []interface{}{doc}, true
	}
	cur := doc
	for _, seg := range splitPath(r.cfg.ArrayPath) {
		m, ok := cur.(map[string]interface{})
		if !ok {
			return nil, false
		}
		cur, ok = m[seg]
		if !ok {
			return nil, false
		}
	}
	switch v := cur.(type) {
	case []interface{}:
		return v, true
	default:
		return []interface{}{v}, true
	}
}

func splitPath(p string) []string {
	var out []string
	cur := ""
	for _, ch := range p {
		switch ch {
		case '.':
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
		default:
			cur += string(ch)
		}
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

func (r *httpjsonReceiver) buildLogs(doc interface{}) plog.Logs {
	items, ok := r.resolveArrayPath(doc)
	if !ok {
		return plog.NewLogs()
	}

	logs := plog.NewLogs()
	rl := logs.ResourceLogs().AppendEmpty()
	rl.Resource().Attributes().PutStr("http.endpoint", r.cfg.Endpoint)
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().SetName("otelcol/httpjson")
	now := pcommon.NewTimestampFromTime(time.Now())

	for _, item := range items {
		rec := sl.LogRecords().AppendEmpty()
		rec.SetObservedTimestamp(now)
		for k, v := range r.cfg.Attributes {
			rec.Attributes().PutStr(k, toStr(v))
		}

		m, isMap := item.(map[string]interface{})
		if isMap {
			for k, v := range m {
				rec.Attributes().PutStr(k, toStr(v))
			}
			if r.cfg.BodyKey != "" {
				if v, ok := m[r.cfg.BodyKey]; ok {
					rec.Body().SetStr(toStr(v))
				} else {
					rec.Body().SetStr(stringify(item))
				}
			} else {
				rec.Body().SetStr(stringify(item))
			}
			if r.cfg.TimestampKey != "" {
				if tv, ok := m[r.cfg.TimestampKey]; ok {
					if r.cfg.Layout != "" {
						if t, err := time.Parse(r.cfg.Layout, toStr(tv)); err == nil {
							rec.SetTimestamp(pcommon.NewTimestampFromTime(t))
						}
					}
				}
			}
		} else {
			rec.Body().SetStr(stringify(item))
		}
		if rec.Timestamp() == 0 {
			rec.SetTimestamp(now)
		}
	}
	return logs
}

func stringify(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func toStr(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	default:
		b, _ := json.Marshal(t)
		return string(b)
	}
}
