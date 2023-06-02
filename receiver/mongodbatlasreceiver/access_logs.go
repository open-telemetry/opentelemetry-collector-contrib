// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbatlasreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver"

import (
	"context"
	"encoding/json"
	"fmt"
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
	accessLogStorageKey           = "last_endtime_access_logs_%s"
	defaultAccessLogsPollInterval = 5 * time.Minute
	defaultAccessLogsPageSize     = 20000
	defaultAccessLogsMaxPages     = 10
)

type accessLogStorageRecord struct {
	ClusterName       string    `json:"cluster_name"`
	NextPollStartTime time.Time `json:"next_poll_start_time"`
}

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

	record     map[string][]*accessLogStorageRecord
	authResult *bool
	wg         *sync.WaitGroup
	cancel     context.CancelFunc
}

func newAccessLogsReceiver(settings rcvr.CreateSettings, cfg *Config, consumer consumer.Logs) *accessLogsReceiver {
	r := &accessLogsReceiver{
		cancel:        func() {},
		client:        internal.NewMongoDBAtlasClient(cfg.PublicKey, cfg.PrivateKey, cfg.RetrySettings, settings.Logger),
		cfg:           cfg,
		logger:        settings.Logger,
		consumer:      consumer,
		wg:            &sync.WaitGroup{},
		storageClient: storage.NewNopClient(),
		record:        make(map[string][]*accessLogStorageRecord),
	}

	for _, p := range cfg.Logs.Projects {
		p.populateIncludesAndExcludes()
		if p.AccessLogs != nil && p.AccessLogs.IsEnabled() {
			if p.AccessLogs.PageSize <= 0 {
				p.AccessLogs.PageSize = defaultAccessLogsPageSize
			}
			if p.AccessLogs.MaxPages <= 0 {
				p.AccessLogs.MaxPages = defaultAccessLogsMaxPages
			}
			if p.AccessLogs.PollInterval == 0 {
				p.AccessLogs.PollInterval = defaultAccessLogsPollInterval
			}
		}
	}

	return r
}

func (alr *accessLogsReceiver) Start(ctx context.Context, _ component.Host, storageClient storage.Client) error {
	alr.logger.Debug("Starting up access log receiver")
	cancelCtx, cancel := context.WithCancel(ctx)
	alr.cancel = cancel
	alr.storageClient = storageClient

	return alr.startPolling(cancelCtx)
}

func (alr *accessLogsReceiver) Shutdown(_ context.Context) error {
	alr.logger.Debug("Shutting down accessLog receiver")
	alr.cancel()
	alr.wg.Wait()
	return nil
}

func (alr *accessLogsReceiver) startPolling(ctx context.Context) error {
	for _, pc := range alr.cfg.Logs.Projects {
		pc := pc
		if pc.AccessLogs == nil || !pc.AccessLogs.IsEnabled() {
			continue
		}

		t := time.NewTicker(pc.AccessLogs.PollInterval)
		alr.wg.Add(1)
		go func() {
			defer alr.wg.Done()
			for {
				select {
				case <-t.C:
					if err := alr.pollAccessLogs(ctx, pc); err != nil {
						alr.logger.Error("error while polling for accessLog", zap.Error(err))
					}
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	return nil
}

func (alr *accessLogsReceiver) pollAccessLogs(ctx context.Context, pc *LogsProjectConfig) error {
	st := pcommon.NewTimestampFromTime(time.Now().Add(-1 * pc.AccessLogs.PollInterval)).AsTime()
	et := time.Now()

	project, err := alr.client.GetProject(ctx, pc.Name)
	if err != nil {
		alr.logger.Error("error retrieving project information", zap.Error(err), zap.String("project", pc.Name))
		return err
	}

	alr.loadCheckpoint(ctx, project.ID)

	clusters, err := alr.client.GetClusters(ctx, project.ID)
	if err != nil {
		alr.logger.Error("error retrieving cluster information", zap.Error(err), zap.String("project", pc.Name))
		return err
	}
	filteredClusters, err := filterClusters(clusters, pc.ProjectConfig)
	if err != nil {
		alr.logger.Error("error filtering clusters", zap.Error(err), zap.String("project", pc.Name))
		return err
	}
	for _, cluster := range filteredClusters {
		clusterCheckpoint := alr.getClusterCheckpoint(project.ID, cluster.Name)

		if clusterCheckpoint == nil {
			clusterCheckpoint = &accessLogStorageRecord{
				ClusterName:       cluster.Name,
				NextPollStartTime: st,
			}
			alr.setClusterCheckpoint(project.ID, clusterCheckpoint)
		}
		clusterCheckpoint.NextPollStartTime = alr.pollCluster(ctx, pc, project, cluster, clusterCheckpoint.NextPollStartTime, et)
		if err = alr.checkpoint(ctx, project.ID); err != nil {
			alr.logger.Warn("error checkpointing", zap.Error(err), zap.String("project", pc.Name))
		}
	}

	return nil
}

func (alr *accessLogsReceiver) pollCluster(ctx context.Context, pc *LogsProjectConfig, project *mongodbatlas.Project, cluster mongodbatlas.Cluster, startTime, now time.Time) time.Time {
	nowTimestamp := pcommon.NewTimestampFromTime(now)

	opts := &internal.GetAccessLogsOptions{
		MaxDate:    now,
		MinDate:    startTime,
		AuthResult: alr.authResult,
		NLogs:      int(pc.AccessLogs.PageSize),
	}

	pageCount := 0
	// Assume failure, in which case we poll starting with the same startTime
	// unless we successfully make request(s) for access logs and they are successfully sent to the consumer
	nextPollStartTime := startTime
	for {
		accessLogs, err := alr.client.GetAccessLogs(ctx, project.ID, cluster.Name, opts)
		pageCount++
		if err != nil {
			alr.logger.Error("unable to get access logs", zap.Error(err), zap.String("project", project.Name),
				zap.String("clusterID", cluster.ID), zap.String("clusterName", cluster.Name))
			return nextPollStartTime
		}

		// No logs retrieved, try again on next interval with the same start time as the API may not have
		// all logs for the given time available to be queried yet (undocumented behavior)
		if len(accessLogs) == 0 {
			return nextPollStartTime
		}

		logs := transformAccessLogs(nowTimestamp, accessLogs, project, cluster, alr.logger)
		if err = alr.consumer.ConsumeLogs(ctx, logs); err != nil {
			alr.logger.Error("error consuming project cluster log", zap.Error(err), zap.String("project", project.Name),
				zap.String("clusterID", cluster.ID), zap.String("clusterName", cluster.Name))
			return nextPollStartTime
		}

		// The first page of results will have the latest data, so we want to update the nextPollStartTime
		// There is risk of data loss at this point if we are unable to then process the remaining pages
		// of data, but that is a limitation of the API that we can't work around.
		if pageCount == 1 {
			// This slice access is safe as we have previously confirmed that the slice is not empty
			mostRecentLogTimestamp, tsErr := getTimestamp(accessLogs[0])
			if tsErr != nil {
				alr.logger.Error("error getting latest log timestamp for calculating next poll timestamps", zap.Error(tsErr),
					zap.String("project", project.Name), zap.String("clusterName", cluster.Name))
				// If we are not able to get the latest log timestamp, we have to assume that we are collecting all
				// data and don't want to risk duplicated data by re-polling the same data again.
				nextPollStartTime = now
			} else {
				nextPollStartTime = mostRecentLogTimestamp.Add(100 * time.Millisecond)
			}
		}

		// If we get back less than the maximum number of logs, we can assume that we've retrieved all of the logs
		// that are currently available for this time period, though some logs may not be available in the API yet.
		if len(accessLogs) < int(pc.AccessLogs.PageSize) {
			return nextPollStartTime
		}

		if pageCount >= int(pc.AccessLogs.MaxPages) {
			alr.logger.Warn(`reached maximum number of pages of access logs, increase 'max_pages' or 
			frequency of 'poll_interval' to ensure all access logs are retrieved`, zap.Int("maxPages", int(pc.AccessLogs.MaxPages)))
			return nextPollStartTime
		}

		// If we get back the maximum number of logs, we need to re-query with a new end time. While undocumented, the API
		// returns the most recent logs first. If we get the maximum number of logs back, we can assume that
		// there are more logs to be retrieved. We'll re-query with the same start time, but the end
		// time set to just before the timestamp of the oldest log entry returned.
		oldestLogTimestampFromPage, err := getTimestamp(accessLogs[len(accessLogs)-1])
		if err != nil {
			alr.logger.Error("error getting oldest log timestamp for calculating next request timestamps", zap.Error(err),
				zap.String("project", project.Name), zap.String("clusterName", cluster.Name))
			return nextPollStartTime
		}

		opts.MaxDate = oldestLogTimestampFromPage.Add(-1 * time.Millisecond)

		// If the new max date is before the min date, we've retrieved all of the logs for this time period
		// and receiving the maximum number of logs back is a coincidence.
		if opts.MaxDate.Before(opts.MinDate) {
			break
		}
	}

	return now
}

func getTimestamp(log *mongodbatlas.AccessLogs) (time.Time, error) {
	body, err := parseLogMessage(log)
	if err != nil {
		// If body couldn't be parsed, we'll still use the outer Timestamp field to determine the new max date.
		body = map[string]interface{}{}
	}
	return getTimestampPreparsedBody(log, body)
}

func getTimestampPreparsedBody(log *mongodbatlas.AccessLogs, body map[string]interface{}) (time.Time, error) {
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

		ts, err := getTimestampPreparsedBody(accessLog, logBody)
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

func accessLogsCheckpointKey(groupID string) string {
	return fmt.Sprintf(accessLogStorageKey, groupID)
}

func (alr *accessLogsReceiver) checkpoint(ctx context.Context, groupID string) error {
	marshalBytes, err := json.Marshal(alr.record)
	if err != nil {
		return fmt.Errorf("unable to write checkpoint: %w", err)
	}
	return alr.storageClient.Set(ctx, accessLogsCheckpointKey(groupID), marshalBytes)
}

func (alr *accessLogsReceiver) loadCheckpoint(ctx context.Context, groupID string) {
	cBytes, err := alr.storageClient.Get(ctx, accessLogsCheckpointKey(groupID))
	if err != nil {
		alr.logger.Info("unable to load checkpoint from storage client, continuing without a previous checkpoint", zap.Error(err))
		if _, ok := alr.record[groupID]; !ok {
			alr.record[groupID] = []*accessLogStorageRecord{}
		}
		return
	}

	if cBytes == nil {
		if _, ok := alr.record[groupID]; !ok {
			alr.record[groupID] = []*accessLogStorageRecord{}
		}
		return
	}

	var record []*accessLogStorageRecord
	if err = json.Unmarshal(cBytes, &record); err != nil {
		alr.logger.Error("unable to decode stored record for access logs, continuing without a checkpoint", zap.Error(err))
		if _, ok := alr.record[groupID]; !ok {
			alr.record[groupID] = []*accessLogStorageRecord{}
		}
	}
}

func (alr *accessLogsReceiver) getClusterCheckpoint(groupID, clusterName string) *accessLogStorageRecord {
	for key, value := range alr.record {
		if key == groupID {
			for _, v := range value {
				if v.ClusterName == clusterName {
					return v
				}
			}
		}
	}
	return nil
}

func (alr *accessLogsReceiver) setClusterCheckpoint(groupID string, clusterCheckpoint *accessLogStorageRecord) {
	groupCheckpoints, ok := alr.record[groupID]
	if !ok {
		alr.record[groupID] = []*accessLogStorageRecord{clusterCheckpoint}
	}

	var found bool
	for idx, v := range groupCheckpoints {
		if v.ClusterName == clusterCheckpoint.ClusterName {
			found = true
			alr.record[groupID][idx] = clusterCheckpoint
		}
	}
	if !found {
		alr.record[groupID] = append(alr.record[groupID], clusterCheckpoint)
	}
}
