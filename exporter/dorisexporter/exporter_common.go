// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dorisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter"

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"go.uber.org/zap"
)

const timeFormat = "2006-01-02 15:04:05.999999"

type commonExporter struct {
	client *http.Client

	logger   *zap.Logger
	cfg      *Config
	timeZone *time.Location
}

func newExporter(logger *zap.Logger, cfg *Config) (*commonExporter, error) {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			req.SetBasicAuth(cfg.Username, cfg.Password)
			return nil
		},
	}

	timeZone, err := cfg.timeZone()
	if err != nil {
		return nil, err
	}

	return &commonExporter{
		logger:   logger,
		cfg:      cfg,
		client:   client,
		timeZone: timeZone,
	}, nil
}

func (e *commonExporter) formatTime(t time.Time) string {
	return t.In(e.timeZone).Format(timeFormat)
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
	url := streamLoadUrl(cfg.Endpoint, cfg.Database, table)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	req.Header.Set("format", "json")
	req.Header.Set("Expect", "100-continue")
	req.Header.Set("strip_outer_array", "true")
	req.Header.Set("timezone", cfg.TimeZone)
	req.SetBasicAuth(cfg.Username, cfg.Password)

	return req, nil
}

func createMySQLClient(cfg *Config) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/mysql", cfg.Username, cfg.Password, cfg.MySQLEndpoint)
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
