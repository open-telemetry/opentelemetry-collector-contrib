// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dorisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter"

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"net/http"

	_ "github.com/go-sql-driver/mysql"
)

const TimeFormat = "2006-01-02 15:04:05.999999-07:00"

type Trace struct {
	ServiceName        string         `json:"service_name"`
	Timestamp          string         `json:"timestamp"`
	TraceID            string         `json:"trace_id"`
	SpanID             string         `json:"span_id"`
	TraceState         string         `json:"trace_state"`
	ParentSpanID       string         `json:"parent_span_id"`
	SpanName           string         `json:"span_name"`
	SpanKind           string         `json:"span_kind"`
	EndTime            string         `json:"end_time"`
	Duration           int64          `json:"duration"`
	SpanAttributes     map[string]any `json:"span_attributes"`
	Events             []*Event       `json:"events"`
	Links              []*Link        `json:"links"`
	StatusMessage      string         `json:"status_message"`
	StatusCode         string         `json:"status_code"`
	ResourceAttributes map[string]any `json:"resource_attributes"`
	ScopeName          string         `json:"scope_name"`
	ScopeVersion       string         `json:"scope_version"`
}

type Event struct {
	Timestamp  string         `json:"timestamp"`
	Name       string         `json:"name"`
	Attributes map[string]any `json:"attributes"`
}

type Link struct {
	TraceID    string         `json:"trace_id"`
	SpanID     string         `json:"span_id"`
	TraceState string         `json:"trace_state"`
	Attributes map[string]any `json:"attributes"`
}

type StreamLoadResponse struct {
	TxnId                  int64
	Label                  string
	Status                 string
	ExistingJobStatus      string
	Message                string
	NumberTotalRows        int64
	NumberLoadedRows       int64
	NumberFilteredRows     int64
	NumberUnselectedRows   int64
	LoadBytes              int64
	LoadTimeMs             int64
	BeginTxnTimeMs         int64
	StreamLoadPutTimeMs    int64
	ReadDataTimeMs         int64
	WriteDataTimeMs        int64
	CommitAndPublishTimeMs int64
	ErrorURL               string
}

func (r *StreamLoadResponse) Success() bool {
	return r.Status == "Success" || r.ExistingJobStatus == "Publish Timeout"
}

func streamLoadUrl(address string, db string, table string) string {
	return address + "/api/" + db + "/" + table + "/_stream_load"
}

func streamLoadRequest(ctx context.Context, cfg *Config, table string, data []byte) (*http.Request, error) {
	url := streamLoadUrl(cfg.Endpoint.HTTP, cfg.Database, table)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	req.Header.Set("format", "json")
	req.Header.Set("Expect", "100-continue")
	req.Header.Set("strip_outer_array", "true")
	req.SetBasicAuth(cfg.Username, cfg.Password)

	return req, nil
}

func createMySQLClient(cfg *Config) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/mysql", cfg.Username, cfg.Password, cfg.Endpoint.TCP[6:])
	conn, err := sql.Open("mysql", dsn)
	return conn, err
}

func createAndUseDatabase(ctx context.Context, conn *sql.DB, cfg *Config) error {
	_, err := conn.ExecContext(ctx, "CREATE DATABASE IF NOT EXISTS "+cfg.Database)
	if err != nil {
		return err
	}
	_, err = conn.ExecContext(ctx, "USE "+cfg.Database)
	return err
}
