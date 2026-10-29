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
	"github.com/google/uuid"
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
	reporter *progressReporter
}

func newExporter(logger *zap.Logger, cfg *Config, set component.TelemetrySettings, reporterName string) *commonExporter {
	return &commonExporter{
		TelemetrySettings: set,
		logger:            logger,
		cfg:               cfg,
		timeZone:          cfg.timeLocation,
		reporter:          newProgressReporter(reporterName, cfg.LogProgressInterval, logger),
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
	return r.Status == "Success" || r.Status == "Publish Timeout" || r.Status == "Label Already Exists"
}

func (r *streamLoadResponse) duplication() bool {
	return r.Status == "Label Already Exists"
}

func streamLoadURL(address, db, table string) string {
	return address + "/api/" + db + "/" + table + "/_stream_load"
}

func generateLabel(cfg *Config, table string) string {
	return fmt.Sprintf(
		"%s_%s_%s_%s_%s",
		cfg.LabelPrefix,
		cfg.Database,
		table,
		time.Now().In(cfg.timeLocation).Format("20060102150405"),
		uuid.New().String(),
	)
}

func streamLoadRequest(ctx context.Context, cfg *Config, table string, data []byte, label string) (*http.Request, error) {
	url := streamLoadURL(cfg.ClientConfig.Endpoint, cfg.Database, table)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}

	req.Header.Set("format", "json")
	req.Header.Set("Expect", "100-continue")
	req.Header.Set("read_json_by_line", "true")
	groupCommit, _ := cfg.ClientConfig.Headers.Get("group_commit")
	if groupCommit == "" || groupCommit == "off_mode" {
		req.Header.Set("label", label)
	}
	if cfg.ClientConfig.Timeout != 0 {
		req.Header.Set("timeout", fmt.Sprintf("%d", cfg.ClientConfig.Timeout/time.Second))
	}
	req.SetBasicAuth(cfg.Username, string(cfg.Password))

	return req, nil
}

func createDorisHTTPClient(ctx context.Context, cfg *Config, host component.Host, settings component.TelemetrySettings) (*http.Client, error) {
	client, err := cfg.ClientConfig.ToClient(ctx, host.GetExtensions(), settings)
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

const (
	partitionsReadyPollInterval = time.Second
	partitionsReadyTimeout      = 60 * time.Second
)

// waitForPartitionsReady polls SHOW PARTITIONS until the table has at least
// `expected` partitions. Dynamic-partition tables create their initial set of
// partitions asynchronously after CREATE TABLE returns; creating a
// materialized view before that is finished can corrupt Doris state.
func waitForPartitionsReady(ctx context.Context, conn *sql.DB, logger *zap.Logger, database, table string, expected int) error {
	query := fmt.Sprintf("SHOW PARTITIONS FROM `%s`.`%s`", database, table)
	deadline := time.Now().Add(partitionsReadyTimeout)

	for {
		count, err := countPartitions(ctx, conn, query)
		if err == nil && count >= expected {
			return nil
		}
		if err != nil {
			logger.Warn("failed to show partitions, will retry",
				zap.String("table", table), zap.Error(err))
		} else {
			logger.Debug("waiting for partitions to be ready",
				zap.String("table", table),
				zap.Int("got", count),
				zap.Int("expected", expected))
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for partitions of %s.%s (expected >= %d)", database, table, expected)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(partitionsReadyPollInterval):
		}
	}
}

func countPartitions(ctx context.Context, conn *sql.DB, query string) (int, error) {
	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
	}
	return count, rows.Err()
}

const (
	mvReadyPollInterval = time.Second
	mvReadyTimeout      = 5 * time.Minute
)

// waitForMaterializedViewReady polls SHOW ALTER TABLE MATERIALIZED VIEW until the
// named view on the table has finished building. Doris returns from CREATE
// MATERIALIZED VIEW immediately and builds it in the background; a second view on
// the same table can only be created once the first one is done.
func waitForMaterializedViewReady(ctx context.Context, conn *sql.DB, logger *zap.Logger, database, table, view string) error {
	query := fmt.Sprintf("SHOW ALTER TABLE MATERIALIZED VIEW FROM `%s` WHERE TableName = '%s'", database, table)
	deadline := time.Now().Add(mvReadyTimeout)

	for {
		state, err := materializedViewState(ctx, conn, query, view)
		switch {
		case err != nil:
			logger.Warn("failed to read materialized view state, will retry",
				zap.String("view", view), zap.Error(err))
		case state == "FINISHED" || state == "CANCELLED" || state == "":
			// empty means the job list no longer mentions it, i.e. it finished earlier
			return nil
		default:
			logger.Debug("waiting for materialized view",
				zap.String("view", view), zap.String("state", state))
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for materialized view %s on %s.%s", view, database, table)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(mvReadyPollInterval):
		}
	}
}

func materializedViewState(ctx context.Context, conn *sql.DB, query, view string) (string, error) {
	rows, err := conn.QueryContext(ctx, query)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return "", err
	}

	state := ""
	for rows.Next() {
		values := make([]sql.NullString, len(columns))
		pointers := make([]any, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return "", err
		}
		row := make(map[string]string, len(columns))
		for i, c := range columns {
			row[c] = values[i].String
		}
		if row["RollupIndexName"] == view {
			state = row["State"]
		}
	}
	return state, rows.Err()
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
