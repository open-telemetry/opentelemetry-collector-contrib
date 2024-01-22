// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package osqueryreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/osqueryreceiver"

import (
	"context"
	"sync"
	"time"

	osquery "github.com/osquery/osquery-go"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver"
	semconv "go.opentelemetry.io/collector/semconv/v1.21.0"
	"go.uber.org/zap"
)

const (
	defaultClientConnectRetries = 3
	defaultReconnectTimeout     = time.Millisecond * 200
)

type osQueryReceiver struct {
	config       *Config
	logger       *zap.Logger
	client       *osquery.ExtensionManagerClient
	logsConsumer consumer.Logs
	mx           sync.Mutex
	stopperChan  chan struct{}
	wg           sync.WaitGroup
	start        time.Time
	end          time.Time
}

func newOsQueryReceiver(cfg *Config, consumer consumer.Logs, set receiver.CreateSettings) (*osQueryReceiver, error) {
	client, err := cfg.getOsQueryClient()
	if err != nil {
		return nil, err
	}

	osqr := &osQueryReceiver{
		config:       cfg,
		logsConsumer: consumer,
		stopperChan:  make(chan struct{}),
		logger:       set.Logger,
		client:       client,
	}

	return osqr, nil
}

func newLog(ld plog.Logs, query string, row map[string]string) plog.Logs {
	rl := ld.ResourceLogs().AppendEmpty()
	resourceAttrs := rl.Resource().Attributes()
	for k, v := range row {
		resourceAttrs.PutStr(k, v)
	}

	sl := rl.ScopeLogs().AppendEmpty()
	sl.Scope().Attributes().PutStr(semconv.AttributeOTelScopeName, "otelcol/osqueryreceiver")

	lr := sl.LogRecords().AppendEmpty()
	lr.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	lr.SetSeverityNumber(plog.SeverityNumberInfo)
	lr.SetSeverityText("INFO")
	lr.Body().SetStr(query)
	return ld
}

func (or *osQueryReceiver) connect(retries int) (*osquery.ExtensionManagerClient, error) {
	client, err := or.config.getOsQueryClient()
	for err != nil && retries > 0 {
		or.logger.Error("Could not connect to osquery socket, retrying", zap.Error(err))
		time.Sleep(defaultReconnectTimeout)
		client, err = or.config.getOsQueryClient()
		retries--
	}

	return client, err
}

func (or *osQueryReceiver) runQuery(ctx context.Context, query string) {
	or.mx.Lock()
	defer or.mx.Unlock()

	if or.client == nil {
		return
	}
	or.logger.Debug("Running query", zap.String("query", query))

	// Use a separate connection for queries in order to be able to recover from timed out queries
	queryClient, err := or.connect(defaultClientConnectRetries)
	if err != nil {
		or.logger.Error("Could not connect to osquery socket", zap.Error(err))
		return
	}
	defer queryClient.Close()

	rows, err := queryClient.QueryRows(query)
	if err != nil {
		or.logger.Error("Error running query", zap.Error(err))
	}
	ld := plog.NewLogs()
	for _, row := range rows {
		newLog(ld, query, row)
	}

	err = or.logsConsumer.ConsumeLogs(ctx, ld)
	if err != nil {
		or.logger.Error("Error consuming logs", zap.Error(err))
	}
}

func (or *osQueryReceiver) collect(ctx context.Context) {
	for _, query := range or.config.Queries {
		go or.runQuery(ctx, query)
	}
}

func (or *osQueryReceiver) Start(ctx context.Context, _ component.Host) error {
	or.logger.Info("Starting osquery receiver", zap.Int("queries", len(or.config.Queries)))

	collectionInterval := or.config.ScraperControllerSettings.CollectionInterval

	or.wg.Add(1)
	go func() {
		defer or.wg.Done()
		or.start = time.Now().Add(-collectionInterval)
		or.end = time.Now()
		for {
			or.collect(ctx)
			// collection interval loop
			select {
			case <-ctx.Done():
				return
			case <-or.stopperChan:
				return
			case <-time.After(collectionInterval):
				or.start = or.end
				or.end = time.Now()
			}
		}
	}()
	return nil
}

func (or *osQueryReceiver) Shutdown(context.Context) error {
	close(or.stopperChan)
	or.wg.Wait()
	if or.client != nil {
		or.client.Close()
	}
	return nil
}
