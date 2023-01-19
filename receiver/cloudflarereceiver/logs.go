// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cloudflarereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudflarereceiver"

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/extension/experimental/storage"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	rcvr "go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudflarereceiver/internal/models"
)

const (
	storageCacheKey = "last_timestamp"
)

type logsReceiver struct {
	pollInterval  time.Duration
	nextStartTime string
	logger        *zap.Logger
	client        client
	cfg           *Config
	consumer      consumer.Logs
	wg            *sync.WaitGroup
	doneChan      chan bool
	id            component.ID  // ID of the receiver component
	storageID     *component.ID // ID of the storage extension component
	storageClient storage.Client
	record        *logRecord
}

func newLogsReceiver(params rcvr.CreateSettings, cfg *Config, consumer consumer.Logs) (*logsReceiver, error) {
	client, err := newCloudflareClient(cfg)
	if err != nil {
		return nil, err
	}

	return &logsReceiver{
		pollInterval:  cfg.PollInterval,
		cfg:           cfg,
		nextStartTime: time.Now().Add(-cfg.PollInterval).Format(time.RFC3339),
		consumer:      consumer,
		logger:        params.Logger,
		wg:            &sync.WaitGroup{},
		doneChan:      make(chan bool),
		id:            params.ID,
		storageID:     cfg.StorageID,
		client:        client,
	}, nil
}

func (l *logsReceiver) Start(ctx context.Context, host component.Host) error {
	l.logger.Debug("starting to poll for Cloudflare logs")
	storageClient, err := adapter.GetStorageClient(ctx, host, l.storageID, l.id)
	if err != nil {
		l.logger.Error("failed to set up storage: %w", zap.Error(err))
	}

	l.storageClient = storageClient
	go l.startPolling(ctx, host)
	return nil
}

func (l *logsReceiver) Shutdown(ctx context.Context) error {
	l.logger.Debug("shutting down logs receiver")
	close(l.doneChan)
	l.wg.Wait()

	return l.writeCheckpoint(ctx)
}

func (l *logsReceiver) startPolling(ctx context.Context, host component.Host) {
	l.logger.Debug("starting cloudflare receiver in retrieval mode")

	err := l.syncPersistence(ctx)
	if err != nil {
		l.logger.Error("there was an error syncing the receiver with checkpoint", zap.Error(err))
	}

	t := time.NewTicker(l.pollInterval)

	l.wg.Add(1)
	defer l.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case <-l.doneChan:
			return
		case <-t.C:
			err := l.poll(ctx)
			if err != nil {
				l.logger.Error("there was an error during the poll", zap.Error(err))
			}
		}
	}
}

func (l *logsReceiver) poll(ctx context.Context) error {
	var errs error
	startTime := l.nextStartTime
	if l.record != nil {
		startTime = l.record.LastRecordedTime.Format(time.RFC3339)
		l.record = nil
	}
	endTime := time.Now().Format(time.RFC3339)

	err := l.pollForLogs(ctx, startTime, endTime)
	if err != nil {
		return err
	}

	l.nextStartTime = endTime
	return errs
}

func (l *logsReceiver) pollForLogs(ctx context.Context, startTime, endTime string) error {
	resp, err := l.client.MakeRequest(ctx, defaultBaseURL, startTime, endTime)
	if err != nil {
		l.logger.Error("unable to retrieve logs from Cloudflare", zap.Error(err))
		return err
	}

	observedTime := pcommon.NewTimestampFromTime(time.Now())
	logs := l.processLogs(observedTime, resp)
	if logs.LogRecordCount() > 0 {
		if err = l.consumer.ConsumeLogs(ctx, logs); err != nil {
			l.logger.Error("unable to consume logs", zap.Error(err))
			return err
		}
	}

	return nil
}

func (l *logsReceiver) processLogs(now pcommon.Timestamp, logs []*models.Log) plog.Logs {
	pLogs := plog.NewLogs()

	for _, log := range logs {
		resourceLogs := pLogs.ResourceLogs().AppendEmpty()
		logRecord := resourceLogs.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
		resourceAttributes := resourceLogs.Resource().Attributes()
		attrs := logRecord.Attributes()

		logRecord.SetObservedTimestamp(now)
		resourceAttributes.PutStr("cloudflare.ZoneID", l.cfg.Zone)

		ts := time.Unix(log.EdgeEndTimestamp, 0)
		logRecord.SetTimestamp(pcommon.NewTimestampFromTime(ts))
		logRecord.SetSeverityNumber(severityFromAPI(log.EdgeResponseStatus))
		logRecord.SetSeverityText(severityFromAPI(log.EdgeResponseStatus).String())

		bodyBytes, err := json.Marshal(logs)
		if err != nil {
			l.logger.Warn("unable to marshal log into a body string")
			continue
		}
		logRecord.Body().SetStr(string(bodyBytes))

		// These attributes are optional and may not be present, depending on the selected fields.
		putStringToMapNotNil(attrs, "ClientIP", &log.ClientIP)
		putStringToMapNotNil(attrs, "ClientRequestHost", &log.ClientRequestHost)
		putStringToMapNotNil(attrs, "ClientRequestMethod", &log.ClientRequestMethod)
		putStringToMapNotNil(attrs, "ClientRequestURI", &log.ClientRequestURI)
		putIntToMapNotNil(attrs, "EdgeEndTimestamp", &log.EdgeEndTimestamp)
		putIntToMapNotNil(attrs, "EdgeResponseBytes", &log.EdgeResponseBytes)
		putIntToMapNotNil(attrs, "EdgeResponseStatus", &log.EdgeResponseStatus)
		putIntToMapNotNil(attrs, "EdgeStartTimestamp", &log.EdgeStartTimestamp)
		putStringToMapNotNil(attrs, "RayID", &log.RayID)
	}

	return pLogs
}

func putStringToMapNotNil(m pcommon.Map, k string, v *string) {
	if v != nil {
		m.PutStr(k, *v)
	}
}

func putIntToMapNotNil(m pcommon.Map, k string, v *int64) {
	if v != nil {
		m.PutInt(k, *v)
	}
}

// logRecord wraps a sync Map so it is goroutine safe as well as
// can have custom marshaling
type logRecord struct {
	sync.Mutex
	LastRecordedTime time.Time `mapstructure:"last_recorded"`
}

func (r *logRecord) SetLastRecorded(lastUpdated *time.Time) {
	r.Lock()
	r.LastRecordedTime = *lastUpdated
	r.Unlock()
}

func (l *logsReceiver) syncPersistence(ctx context.Context) error {
	if l.storageClient == nil {
		return nil
	}
	cBytes, err := l.storageClient.Get(ctx, storageCacheKey)
	if err != nil || cBytes == nil {
		l.record = &logRecord{}
		return nil
	}

	var cache logRecord
	if err = json.Unmarshal(cBytes, &cache); err != nil {
		return fmt.Errorf("unable to decode stored cache: %w", err)
	}
	l.record = &cache
	return nil

}

func (l *logsReceiver) writeCheckpoint(ctx context.Context) error {
	if l.storageClient == nil {
		l.logger.Error("unable to write checkpoint since no storage client was found")
		return errors.New("missing storage client")
	}
	marshalBytes, err := json.Marshal(&l.nextStartTime)
	if err != nil {
		return fmt.Errorf("unable to write checkpoint: %w", err)
	}
	err = l.storageClient.Set(ctx, storageCacheKey, marshalBytes)
	return err
}

// severityFromAPI is a workaround for shared types between the API and the model
func severityFromAPI(statusCode int64) plog.SeverityNumber {
	switch {
	case statusCode < 300:
		return plog.SeverityNumberInfo
	case statusCode < 400:
		return plog.SeverityNumberInfo2
	case statusCode < 500:
		return plog.SeverityNumberWarn
	case statusCode < 600:
		return plog.SeverityNumberError
	default:
		return plog.SeverityNumberUnspecified
	}
}
