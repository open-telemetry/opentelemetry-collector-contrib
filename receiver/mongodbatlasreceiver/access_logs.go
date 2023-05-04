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

package mongodbatlasreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver"

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"go.mongodb.org/atlas/mongodbatlas"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/extension/experimental/storage"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	rcvr "go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal"
)

const (
	accessLogStorageKey           = "last_endtime_access_logs"
	defaultAccessLogsPollInterval = time.Minute

	defaultAccessLogsPageSize = 20000
	defaultAccessLogsMaxPages = 10
)

type accessLogClient interface {
	GetProject(ctx context.Context, groupID string) (*mongodbatlas.Project, error)
	GetClusters(ctx context.Context, groupID string) ([]mongodbatlas.Cluster, error)
	GetAccessLogs(ctx context.Context, groupID string, clusterName string, opts *internal.GetAccessLogsOptions) (ret []*mongodbatlas.AccessLogs, err error)
}

type accessLogsReceiver struct {
	client        accessLogClient
	logger        *zap.Logger
	storageClient storage.Client
	cfg           *Config
	consumer      consumer.Logs

	nextStartTime *time.Time
	pollInterval  time.Duration
	pageSize      int
	maxPages      int
	authResult    *bool
	wg            *sync.WaitGroup
	cancel        context.CancelFunc
}

func newAccessLogsReceiver(settings rcvr.CreateSettings, cfg *Config, consumer consumer.Logs) *accessLogsReceiver {
	r := &accessLogsReceiver{
		client:        internal.NewMongoDBAtlasClient(cfg.PublicKey, cfg.PrivateKey, cfg.RetrySettings, settings.Logger),
		cfg:           cfg,
		logger:        settings.Logger,
		consumer:      consumer,
		pollInterval:  cfg.AccessLogs.PollInterval,
		authResult:    cfg.AccessLogs.AuthResult,
		wg:            &sync.WaitGroup{},
		storageClient: storage.NewNopClient(),
	}

	if r.maxPages == 0 {
		r.maxPages = defaultAccessLogsMaxPages
	}

	if r.pageSize == 0 {
		r.pageSize = defaultAccessLogsPageSize
	}

	if r.pollInterval == 0 {
		r.pollInterval = defaultAccessLogsPollInterval
	}

	for _, p := range cfg.AccessLogs.Projects {
		p.populateIncludesAndExcludes()
	}

	return r
}

func (alr *accessLogsReceiver) Start(ctx context.Context, host component.Host, storageClient storage.Client) error {
	alr.logger.Debug("Starting up access log receiver")
	cancelCtx, cancel := context.WithCancel(ctx)
	alr.cancel = cancel
	alr.storageClient = storageClient
	alr.loadCheckpoint(cancelCtx)

	return alr.startPolling(cancelCtx)
}

func (alr *accessLogsReceiver) Shutdown(ctx context.Context) error {
	alr.logger.Debug("Shutting down accessLog receiver")
	alr.cancel()
	alr.wg.Wait()
	return alr.checkpoint(ctx)
}

func (alr *accessLogsReceiver) startPolling(ctx context.Context) error {
	t := time.NewTicker(alr.pollInterval)
	alr.wg.Add(1)
	go func() {
		defer alr.wg.Done()
		for {
			select {
			case <-t.C:
				if err := alr.pollAccessLogs(ctx); err != nil {
					alr.logger.Error("error while polling for accessLog", zap.Error(err))
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (alr *accessLogsReceiver) pollAccessLogs(ctx context.Context) error {
	st := pcommon.NewTimestampFromTime(time.Now().Add(-alr.pollInterval)).AsTime()
	if alr.nextStartTime != nil {
		st = *alr.nextStartTime
	}
	et := time.Now()

	for _, pc := range alr.cfg.AccessLogs.Projects {
		project, err := alr.client.GetProject(ctx, pc.Name)
		if err != nil {
			alr.logger.Error("error retrieving project information", zap.Error(err), zap.String("project", pc.Name))
			return err
		}
		clusters, err := alr.client.GetClusters(ctx, project.ID)
		if err != nil {
			alr.logger.Error("error retrieving cluster information", zap.Error(err), zap.String("project", pc.Name))
			return err
		}
		filteredClusters, err := filterClusters(clusters, pc)
		if err != nil {
			alr.logger.Error("error filtering clusters", zap.Error(err), zap.String("project", pc.Name))
			return err
		}
		for _, cluster := range filteredClusters {
			alr.pollCluster(ctx, project, cluster, st, et)
		}
	}

	alr.nextStartTime = &et
	return alr.checkpoint(ctx)
}

func (alr *accessLogsReceiver) pollCluster(ctx context.Context, project *mongodbatlas.Project, cluster mongodbatlas.Cluster, startTime, now time.Time) {
	nowTimestamp := pcommon.NewTimestampFromTime(now)

	opts := &internal.GetAccessLogsOptions{
		MaxDate:    now,
		MinDate:    startTime,
		AuthResult: alr.authResult,
		NLogs:      alr.pageSize,
	}

	pageCount := 0
	for {
		accessLogs, err := alr.client.GetAccessLogs(ctx, project.ID, cluster.ID, opts)
		pageCount++
		if err != nil {
			alr.logger.Error("unable to get access logs", zap.Error(err), zap.String("project", project.Name),
				zap.String("clusterID", cluster.ID), zap.String("clusterName", cluster.Name))
			return
		}

		logs := transformAccessLogs(nowTimestamp, accessLogs, project, cluster, alr.logger)

		if logs.LogRecordCount() > 0 {
			if err = alr.consumer.ConsumeLogs(ctx, logs); err != nil {
				alr.logger.Error("error consuming project cluster log", zap.Error(err), zap.String("project", project.Name),
					zap.String("clusterID", cluster.ID), zap.String("clusterName", cluster.Name))
				return
			}
		}

		// If we get back less than the maximum number of logs, we can assume that we've retrieved all of the logs for this
		// time period.
		if len(accessLogs) < alr.pageSize {
			break
		}

		if pageCount >= alr.maxPages {
			alr.logger.Warn(`reached maximum number of pages of access logs, increase 'max_pages' or 
			frequency of 'poll_interval' to ensure all access logs are retrieved`, zap.Int("maxPages", alr.maxPages))
			break
		}

		// If we get back the maximum number of logs, we need to re-query with a new start time. While undocumented, the API
		// returns the most recent logs first. If we get the maximum number of logs back, we can assume that
		// there are more logs to be retrieved. We'll re-query with the same start time, but the end
		// time set to just before the timestamp of the last log entry returned.
		lastLog := accessLogs[len(accessLogs)-1]
		body, err := parseLogMessage(lastLog)
		if err != nil {
			// If body couldn't be parsed, we'll still use the outer Timestamp field to determine the new max date.
			body = map[string]interface{}{}
		}
		lastLogTimestamp, err := getTimestamp(lastLog, body)
		if err != nil {
			alr.logger.Error("unable to parse last log timestamp", zap.Error(err), zap.String("project", project.Name),
				zap.String("clusterID", cluster.ID), zap.String("clusterName", cluster.Name))
			break
		}
		opts.MaxDate = lastLogTimestamp.Add(-1 * time.Millisecond)

		// If the new max date is before the min date, we've retrieved all of the logs for this time period
		// and receiving the maximum number of logs back is a coincidence.
		if opts.MaxDate.Before(opts.MinDate) {
			break
		}
	}
}

func getTimestamp(log *mongodbatlas.AccessLogs, body map[string]interface{}) (time.Time, error) {
	// If the log message has a timestamp, use that. When present, it has more precision than the timestamp from the access log entry.
	if tMap, ok := body["t"]; ok {
		if dateMap, ok := tMap.(map[string]interface{}); ok {
			if v, ok := dateMap["$date"]; ok {
				if dateStr, ok := v.(string); ok {
					return time.Parse(time.RFC3339, dateStr)
				}
			}
		}
	}

	// If the log message doesn't have a timestamp, use the timestamp from the outer access log entry.
	t, err := time.Parse(time.RFC3339, log.Timestamp)
	if err != nil {
		// The documentation claims ISO8601/RFC3339, but the API has been observed returning timestamps in UnixDate format
		// UnixDate looks like Wed Apr 26 02:38:56 GMT 2023
		unixDate, err2 := time.Parse(time.UnixDate, log.Timestamp)
		if err2 != nil {
			// Return the original error as the documentation claims ISO8601
			return time.Time{}, err
		}
		return unixDate, nil
	}
	return t, nil
}

func parseLogMessage(log *mongodbatlas.AccessLogs) (map[string]interface{}, error) {
	var body map[string]interface{}
	if err := json.Unmarshal([]byte(log.LogLine), &body); err != nil {
		return nil, err
	}
	return body, nil
}

func transformAccessLogs(now pcommon.Timestamp, accessLogs []*mongodbatlas.AccessLogs, p *mongodbatlas.Project, c mongodbatlas.Cluster, logger *zap.Logger) plog.Logs {
	logs := plog.NewLogs()
	resourceLogs := logs.ResourceLogs().AppendEmpty()
	ra := resourceLogs.Resource().Attributes()
	ra.PutStr("mongodbatlas.project.name", p.Name)
	ra.PutStr("mongodbatlas.project.id", p.ID)
	ra.PutStr("mongodbatlas.org.id", p.OrgID)
	ra.PutStr("mongodbatlas.cluster.name", c.Name)

	// Expected format documented https://www.mongodb.com/docs/atlas/reference/api-resources-spec/#tag/Access-Tracking/operation/listAccessLogsByClusterName
	logRecords := resourceLogs.ScopeLogs().AppendEmpty().LogRecords()
	for _, accessLog := range accessLogs {
		logRecord := logRecords.AppendEmpty()
		logBody, err := parseLogMessage(accessLog)
		if err != nil {
			logger.Error("unable to unmarshal access log into body string", zap.Error(err))
			continue
		}
		err = logRecord.Body().SetEmptyMap().FromRaw(logBody)
		if err != nil {
			logger.Error("unable to set log record body as map", zap.Error(err))
			logRecord.Body().SetStr(accessLog.LogLine)
		}

		ts, err := getTimestamp(accessLog, logBody)
		if err != nil {
			logger.Warn("unable to interpret when an access log event was recorded, timestamp not parsed", zap.Error(err), zap.String("timestamp", accessLog.Timestamp))
			logRecord.SetTimestamp(now)
		} else {
			logRecord.SetTimestamp(pcommon.NewTimestampFromTime(ts))
		}
		logRecord.SetObservedTimestamp(now)

		attrs := logRecord.Attributes()
		attrs.PutStr("event.domain", "mongodbatlas")
		logRecord.SetSeverityNumber(plog.SeverityNumberInfo)
		logRecord.SetSeverityText(plog.SeverityNumberInfo.String())
		if accessLog.AuthResult != nil {
			status := "success"
			if !*accessLog.AuthResult {
				logRecord.SetSeverityNumber(plog.SeverityNumberWarn)
				logRecord.SetSeverityText(plog.SeverityNumberWarn.String())
				status = "failure"
			}
			attrs.PutStr("auth.result", status)
		}
		if accessLog.FailureReason != "" {
			attrs.PutStr("auth.failure_reason", accessLog.FailureReason)
		}
		attrs.PutStr("auth.source", accessLog.AuthSource)
		attrs.PutStr("username", accessLog.Username)
		attrs.PutStr("hostname", accessLog.Hostname)
		attrs.PutStr("remote.ip", accessLog.IPAddress)
	}

	return logs
}

func (alr *accessLogsReceiver) checkpoint(ctx context.Context) error {
	if alr.nextStartTime == nil {
		return nil
	}

	return alr.storageClient.Set(ctx, accessLogStorageKey, []byte(alr.nextStartTime.Format(time.RFC3339)))
}

func (alr *accessLogsReceiver) loadCheckpoint(ctx context.Context) {
	cBytes, err := alr.storageClient.Get(ctx, accessLogStorageKey)
	if err != nil {
		alr.logger.Info("unable to load checkpoint from storage client, continuing without a previous checkpoint", zap.Error(err))
		return
	}

	if cBytes == nil {
		return
	}

	nextStartTime, err := time.Parse(time.RFC3339, string(cBytes))
	if err != nil {
		alr.logger.Error("unable to decode stored next start time for access log, continuing without a checkpoint", zap.Error(err))
		alr.nextStartTime = &nextStartTime
	}
}
