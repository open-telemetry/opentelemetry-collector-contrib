// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dorisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter"

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	_ "github.com/go-sql-driver/mysql" // for register database driver
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

const timeFormat = "2006-01-02 15:04:05.999999"

type commonExporter struct {
	component.TelemetrySettings

	client *http.Client

	logger   *zap.Logger
	cfg      *Config
	timeZone *time.Location
}

func newExporter(logger *zap.Logger, cfg *Config, set component.TelemetrySettings) *commonExporter {
	// There won't be an error because it's already been validated in the Config.Validate method.
	timeZone, _ := cfg.timeZone()

	return &commonExporter{
		TelemetrySettings: set,
		logger:            logger,
		cfg:               cfg,
		timeZone:          timeZone,
	}
}

func (e *commonExporter) formatTime(t time.Time) string {
	return t.In(e.timeZone).Format(timeFormat)
}

type streamLoadResponse struct {
	TxnID                  int64
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

func (r *streamLoadResponse) success() bool {
	return r.Status == "Success" || r.Status == "Publish Timeout"
}

func streamLoadURL(address string, db string, table string) string {
	return address + "/api/" + db + "/" + table + "/_stream_load"
}

func streamLoadRequest(ctx context.Context, cfg *Config, table string, data []byte) (*http.Request, error) {
	url := streamLoadURL(cfg.Endpoint, cfg.Database, table)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	req.Header.Set("format", "json")
	req.Header.Set("Expect", "100-continue")
	req.Header.Set("read_json_by_line", "true")
	if cfg.ClientConfig.Timeout != 0 {
		req.Header.Set("timeout", fmt.Sprintf("%d", cfg.ClientConfig.Timeout/time.Second))
	}
	req.SetBasicAuth(cfg.Username, string(cfg.Password))

	return req, nil
}

func createDorisHTTPClient(ctx context.Context, cfg *Config, host component.Host, settings component.TelemetrySettings) (*http.Client, error) {
	client, err := cfg.ClientConfig.ToClient(ctx, host, settings)
	if err != nil {
		return nil, err
	}

	client.CheckRedirect = func(req *http.Request, _ []*http.Request) error {
		req.SetBasicAuth(cfg.Username, string(cfg.Password))
		return nil
	}

	return client, nil
}

func createDorisMySQLClient(cfg *Config) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/mysql", cfg.Username, string(cfg.Password), cfg.MySQLEndpoint)
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

type metric interface {
	dMetricGauge | dMetricSum | dMetricHistogram | dMetricExponentialHistogram | dMetricSummary
}

func toJSONLines[T dLog | dTrace | metric](data []*T) ([]byte, error) {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	for _, d := range data {
		err := enc.Encode(d)
		if err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}
