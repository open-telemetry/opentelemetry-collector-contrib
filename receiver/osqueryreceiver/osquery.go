// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package osqueryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/osqueryreceiver"

import (
	"context"
	"errors"
	"time"

	"github.com/osquery/osquery-go"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/osqueryreceiver/internal/collection"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/osqueryreceiver/internal/metadata"
)

const (
	defaultClientConnectRetries = 3
	defaultReconnectTimeout     = time.Millisecond * 200
	defaultQueryTimeout         = 30 * time.Second
)

type client interface {
	Close()
	QueryRowsContext(ctx context.Context, query string) ([]map[string]string, error)
}

var _ client = &osquery.ExtensionManagerClient{}

func makeOsQueryClient(socket string) (client, error) {
	client, err := osquery.NewClient(socket, defaultQueryTimeout)
	if err != nil {
		return nil, err
	}
	return client, nil
}

type osQueryReceiver struct {
	config       *Config
	logger       *zap.Logger
	createClient func(socket string) (client, error)
	collections  []collection.Collection
}

func newOsQueryReceiver(cfg *Config, set receiver.Settings) *osQueryReceiver {
	return &osQueryReceiver{
		config:       cfg,
		logger:       set.Logger,
		createClient: makeOsQueryClient,
		collections:  resolveCollections(cfg.Collections, set.Logger),
	}
}

// resolveCollections converts the configured collection names into their Collection
// implementations, logging and skipping any name that fails to resolve.
func resolveCollections(names []string, logger *zap.Logger) []collection.Collection {
	resolved := make([]collection.Collection, 0, len(names))
	for _, name := range names {
		c, err := collection.New(name)
		if err != nil {
			logger.Warn("Skipping unknown collection", zap.String("collection", name), zap.Error(err))
			continue
		}
		resolved = append(resolved, c)
	}
	return resolved
}

func (or *osQueryReceiver) connect(retries int) (client, error) {
	c, err := or.createClient(or.config.ExtensionsSocket)
	for err != nil && retries > 0 {
		or.logger.Error("Could not connect to osquery socket, retrying", zap.Error(err))
		time.Sleep(defaultReconnectTimeout)
		c, err = or.createClient(or.config.ExtensionsSocket)
		retries--
	}

	return c, err
}

func (or *osQueryReceiver) runQuery(ctx context.Context, ld plog.Logs, query, collectionName string) error {
	now := pcommon.NewTimestampFromTime(time.Now())

	or.logger.Debug("Running query", zap.String("query", query))

	// Use a separate connection for queries in order to be able to recover from timed out queries
	queryClient, err := or.connect(defaultClientConnectRetries)
	if err != nil {
		or.logger.Error("Could not connect to osquery socket", zap.Error(err))
		return err
	}
	defer queryClient.Close()

	rows, err := queryClient.QueryRowsContext(ctx, query)
	if err != nil {
		or.logger.Error("Error running query", zap.Error(err))
	}
	rl := ld.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().Attributes().PutStr(string(semconv.OTelScopeNameKey), metadata.Type.String())
	for _, row := range rows {
		lr := sl.LogRecords().AppendEmpty()
		lr.SetTimestamp(now)
		lr.SetSeverityNumber(plog.SeverityNumberInfo)
		lr.SetSeverityText("INFO")
		lr.Body().SetStr(query)
		if collectionName != "" {
			lr.Attributes().PutStr("collection", collectionName)
		}
		for k, v := range row {
			lr.Attributes().PutStr(k, v)
		}
	}
	return nil
}

func (or *osQueryReceiver) collect(ctx context.Context) (plog.Logs, error) {
	ld := plog.NewLogs()
	var errs []error
	for _, query := range or.config.Queries {
		errs = append(errs, or.runQuery(ctx, ld, query, ""))
	}
	for _, c := range or.collections {
		errs = append(errs, or.runQuery(ctx, ld, c.GetQuery(), c.GetName()))
	}
	return ld, errors.Join(errs...)
}
