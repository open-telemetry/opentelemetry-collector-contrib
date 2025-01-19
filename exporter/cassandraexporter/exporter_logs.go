// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cassandraexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/cassandraexporter"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gocql/gocql"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/traceutil"
)

type logsExporter struct {
	client *gocql.Session
	logger *zap.Logger
	cfg    *Config
}

func newLogsExporter(logger *zap.Logger, cfg *Config) *logsExporter {
	return &logsExporter{logger: logger, cfg: cfg}
}

func initializeLogKernel(cfg *Config) error {
	ctx := context.Background()
	cluster, err := newCluster(cfg)
	if err != nil {
		return err
	}
	cluster.Consistency = gocql.Quorum
	cluster.Port = cfg.Port
	cluster.Timeout = cfg.Timeout

	session, err := cluster.CreateSession()
	if err != nil {
		return err
	}

	defer session.Close()

	createDatabaseError := session.Query(parseCreateDatabaseSQL(cfg)).WithContext(ctx).Exec()
	if createDatabaseError != nil {
		return createDatabaseError
	}
	createLogTableError := session.Query(parseCreateLogTableSQL(cfg)).WithContext(ctx).Exec()
	if createLogTableError != nil {
		return createLogTableError
	}

	return nil
}

func newCluster(cfg *Config) (*gocql.ClusterConfig, error) {
	cluster := gocql.NewCluster(cfg.DSN)
	if cfg.Auth.UserName != "" && cfg.Auth.Password == "" {
		return nil, errors.New("empty auth.password")
	}
	if cfg.Auth.Password != "" && cfg.Auth.UserName == "" {
		return nil, errors.New("empty auth.username")
	}
	if cfg.Auth.UserName != "" && cfg.Auth.Password != "" {
		cluster.Authenticator = gocql.PasswordAuthenticator{
			Username: cfg.Auth.UserName,
			Password: string(cfg.Auth.Password),
		}
	}
	cluster.Consistency = gocql.Quorum
	cluster.Port = cfg.Port
	return cluster, nil
}

func (e *logsExporter) Start(_ context.Context, _ component.Host) error {
	cluster, err := newCluster(e.cfg)
	if err != nil {
		return err
	}
	cluster.Keyspace = e.cfg.Keyspace
	cluster.Consistency = gocql.Quorum
	cluster.Port = e.cfg.Port
	cluster.Timeout = e.cfg.Timeout

	session, err := cluster.CreateSession()
	if err != nil {
		return err
	}
	e.client = session
	initializeErr := initializeLogKernel(e.cfg)
	return initializeErr
}

func (e *logsExporter) Shutdown(_ context.Context) error {
	if e.client != nil {
		e.client.Close()
	}

	return nil
}

func parseCreateLogTableSQL(cfg *Config) string {
	return fmt.Sprintf(createLogTableSQL, cfg.Keyspace, cfg.LogsTable, cfg.Compression.Algorithm)
}

func (e *logsExporter) pushLogsData(ctx context.Context, ld plog.Logs) error {
	start := time.Now()

	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		logs := ld.ResourceLogs().At(i)
		res := logs.Resource()
		resAttr := attributesToMap(res.Attributes().AsRaw())

		for j := 0; j < logs.ScopeLogs().Len(); j++ {
			rs := logs.ScopeLogs().At(j).LogRecords()
			for k := 0; k < rs.Len(); k++ {
				r := rs.At(k)
				logAttr := attributesToMap(r.Attributes().AsRaw())
				bodyByte, err := json.Marshal(r.Body().AsRaw())
				if err != nil {
					return err
				}

				insertLogError := e.client.Query(fmt.Sprintf(insertLogTableSQL, e.cfg.Keyspace, e.cfg.LogsTable),
					r.Timestamp().AsTime(),
					traceutil.TraceIDToHexOrEmptyString(r.TraceID()),
					traceutil.SpanIDToHexOrEmptyString(r.SpanID()),
					uint32(r.Flags()),
					r.SeverityText(),
					int32(r.SeverityNumber()),
					string(bodyByte),
					resAttr,
					logAttr,
				).WithContext(ctx).Exec()

				if insertLogError != nil {
					e.logger.Error("insert log error", zap.Error(insertLogError))
				}
			}
		}
	}

	duration := time.Since(start)
	e.logger.Debug("insert logs", zap.Int("records", ld.LogRecordCount()),
		zap.String("cost", duration.String()))
	return nil
}
