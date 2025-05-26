// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver"

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/adapter"
)

const (
	noStreamName             = "THIS IS INVALID STREAM"
	maxLogGroupsPerDiscovery = int32(50)
)

type logsReceiver struct {
	settings                      receiver.Settings
	region                        string
	profile                       string
	imdsEndpoint                  string
	pollInterval                  time.Duration
	maxEventsPerRequest           int
	nextStartTime                 time.Time
	groupRequests                 []groupRequest
	autodiscover                  *AutodiscoverConfig
	client                        client
	consumer                      consumer.Logs
	wg                            *sync.WaitGroup
	doneChan                      chan bool
	storageID                     *component.ID
	cloudwatchCheckpointPersister *cloudwatchCheckpointPersister
}

type client interface {
	DescribeLogGroups(ctx context.Context, input *cloudwatchlogs.DescribeLogGroupsInput, opts ...func(options *cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error)
	FilterLogEvents(ctx context.Context, input *cloudwatchlogs.FilterLogEventsInput, opts ...func(options *cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error)
}

type streamNames struct {
	group string
	names []*string
}

func (sn *streamNames) request(limit int, nextToken string, st, et *time.Time) *cloudwatchlogs.FilterLogEventsInput {
	base := &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName: &sn.group,
		StartTime:    aws.Int64(st.UnixMilli()),
		EndTime:      aws.Int64(et.UnixMilli()),
		Limit:        aws.Int32(int32(limit)),
	}
	if len(sn.names) > 0 {
		base.LogStreamNames = aws.ToStringSlice(sn.names)
	}
	if nextToken != "" {
		base.NextToken = aws.String(nextToken)
	}
	return base
}

func (sn *streamNames) groupName() string {
	return sn.group
}

type streamPrefix struct {
	group  string
	prefix *string
}

func (sp *streamPrefix) request(limit int, nextToken string, st, et *time.Time) *cloudwatchlogs.FilterLogEventsInput {
	base := &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName:        &sp.group,
		StartTime:           aws.Int64(st.UnixMilli()),
		EndTime:             aws.Int64(et.UnixMilli()),
		Limit:               aws.Int32(int32(limit)),
		LogStreamNamePrefix: sp.prefix,
	}
	if nextToken != "" {
		base.NextToken = aws.String(nextToken)
	}
	return base
}

func (sp *streamPrefix) groupName() string {
	return sp.group
}

type groupRequest interface {
	request(limit int, nextToken string, st, et *time.Time) *cloudwatchlogs.FilterLogEventsInput
	groupName() string
}

func newLogsReceiver(cfg *Config, settings receiver.Settings, consumer consumer.Logs) *logsReceiver {
	groups := []groupRequest{}
	for logGroupName, sc := range cfg.Logs.Groups.NamedConfigs {
		for _, prefix := range sc.Prefixes {
			groups = append(groups, &streamPrefix{group: logGroupName, prefix: prefix})
		}
		if len(sc.Names) > 0 {
			groups = append(groups, &streamNames{group: logGroupName, names: sc.Names})
		}
		if len(sc.Prefixes) == 0 && len(sc.Names) == 0 {
			groups = append(groups, &streamNames{group: logGroupName})
		}
	}

	// safeguard from using both
	autodiscover := cfg.Logs.Groups.AutodiscoverConfig
	if len(cfg.Logs.Groups.NamedConfigs) > 0 {
		autodiscover = nil
	}

	startTime := time.Unix(0, 0)
	if cfg.Logs.StartFrom != "" {
		parsedTime, err := time.Parse(time.RFC3339, cfg.Logs.StartFrom)
		if err != nil {
			settings.Logger.Error("Unable to parse start time", zap.Error(err))
		} else {
			startTime = parsedTime
		}
	}

	return &logsReceiver{
		settings:            settings,
		region:              cfg.Region,
		profile:             cfg.Profile,
		consumer:            consumer,
		maxEventsPerRequest: cfg.Logs.MaxEventsPerRequest,
		imdsEndpoint:        cfg.IMDSEndpoint,
		autodiscover:        autodiscover,
		pollInterval:        cfg.Logs.PollInterval,
		nextStartTime:       startTime,
		groupRequests:       groups,
		wg:                  &sync.WaitGroup{},
		doneChan:            make(chan bool),
		storageID:           cfg.StorageID,
	}
}

func (l *logsReceiver) Start(ctx context.Context, host component.Host) error {
	if l.cloudwatchCheckpointPersister == nil {
		storageClient, err := adapter.GetStorageClient(ctx, host, l.storageID, l.settings.ID)
		if err != nil {
			l.settings.Logger.Debug("Error connecting to Storage", zap.Error(err))
			return err
		}
		l.cloudwatchCheckpointPersister = newCloudwatchCheckpointPersister(storageClient, l.settings.Logger)
	}

	l.settings.Logger.Debug("starting to poll for Cloudwatch logs")
	l.wg.Add(1)
	go l.startPolling(ctx)
	return nil
}

func (l *logsReceiver) Shutdown(_ context.Context) error {
	l.settings.Logger.Debug("shutting down logs receiver")
	close(l.doneChan)
	l.wg.Wait()
	return nil
}

func (l *logsReceiver) startPolling(ctx context.Context) {
	defer l.wg.Done()

	t := time.NewTicker(l.pollInterval)
	for {
		select {
		case <-ctx.Done():
			return
		case <-l.doneChan:
			return
		case <-t.C:
			if l.autodiscover != nil {
				group, err := l.discoverGroups(ctx, l.autodiscover)
				if err != nil {
					l.settings.Logger.Error("unable to perform discovery of log groups", zap.Error(err))
					continue
				}
				l.groupRequests = group
			}

			err := l.poll(ctx)
			if err != nil {
				l.settings.Logger.Error("there was an error during the poll", zap.Error(err))
			}
		}
	}
}

func (l *logsReceiver) poll(ctx context.Context) error {
	var errs error
	endTime := time.Now()
	for _, r := range l.groupRequests {
		startTime := l.nextStartTime

		// Retrieve the last persisted timestamp for this log group if exists
		if l.cloudwatchCheckpointPersister != nil {
			logGroup := r.groupName()
			checkpoint, err := l.cloudwatchCheckpointPersister.GetCheckpoint(ctx, logGroup)
			if err == nil && checkpoint != "" {
				parsedTime, parseErr := time.Parse(time.RFC3339, checkpoint)
				if parseErr == nil && parsedTime.After(startTime) {
					startTime = parsedTime
					l.settings.Logger.Info("Resuming from previously known checkpoint(s)",
						zap.String("logGroup", logGroup),
						zap.Time("startTime", startTime))
				} else if parseErr != nil {
					l.settings.Logger.Warn("Failed to parse persisted timestamp, using default start time",
						zap.String("logGroup", logGroup),
						zap.String("checkpoint", checkpoint),
						zap.Error(parseErr))
					if err := l.cloudwatchCheckpointPersister.DeleteCheckpoint(ctx, logGroup); err != nil {
						l.settings.Logger.Error("Failed to delete invalid checkpoint",
							zap.String("logGroup", logGroup),
							zap.String("checkpoint", checkpoint),
							zap.Error(err))
					}
				}
			}
		}

		// Poll logs for the current log group
		if err := l.pollForLogs(ctx, r, startTime, endTime); err != nil {
			errs = errors.Join(errs, err)
		}

		// Persist the new end time as the checkpoint for this log group
		if l.cloudwatchCheckpointPersister != nil {
			logGroup := r.groupName()
			newCheckpoint := endTime.Format(time.RFC3339)
			err := l.cloudwatchCheckpointPersister.SetCheckpoint(ctx, logGroup, newCheckpoint)
			if err != nil {
				l.settings.Logger.Error("failed to persist timestamp checkpoint",
					zap.String("logGroup", logGroup),
					zap.String("checkpoint", newCheckpoint),
					zap.Error(err))
			}
		}
	}

	// Update the receiver's nextStartTime for the next poll cycle
	l.nextStartTime = endTime
	return errs
}

func (l *logsReceiver) pollForLogs(ctx context.Context, pc groupRequest, startTime, endTime time.Time) error {
	err := l.ensureSession()
	if err != nil {
		return err
	}
	logGroup := pc.groupName()
	nextToken := aws.String("")

	for nextToken != nil {
		select {
		// if done, we want to stop processing paginated stream of events
		case _, ok := <-l.doneChan:
			if !ok {
				return nil
			}
		default:
			input := pc.request(l.maxEventsPerRequest, *nextToken, &startTime, &endTime)
			resp, err := l.client.FilterLogEvents(ctx, input)
			if err != nil {
				l.settings.Logger.Error("unable to retrieve logs from cloudatch",
					zap.String("logGroup", logGroup),
					zap.Error(err))
				break
			}
			observedTime := pcommon.NewTimestampFromTime(time.Now())
			logs := l.processEvents(observedTime, logGroup, resp)
			if logs.LogRecordCount() > 0 {
				if err = l.consumer.ConsumeLogs(ctx, logs); err != nil {
					l.settings.Logger.Error("unable to consume logs", zap.Error(err))
					break
				}
			}
			nextToken = resp.NextToken
		}
	}
	return nil
}

func (l *logsReceiver) processEvents(now pcommon.Timestamp, logGroupName string, output *cloudwatchlogs.FilterLogEventsOutput) plog.Logs {
	logs := plog.NewLogs()

	resourceMap := map[string](map[string]*plog.ResourceLogs){}

	for _, e := range output.Events {
		if e.Timestamp == nil {
			l.settings.Logger.Error("unable to determine timestamp of event as the timestamp is nil")
			continue
		}

		if e.EventId == nil {
			l.settings.Logger.Error("no event ID was present on the event, skipping entry")
			continue
		}

		if e.Message == nil {
			l.settings.Logger.Error("no message was present on the event", zap.String("event.id", *e.EventId))
			continue
		}

		group, ok := resourceMap[logGroupName]
		if !ok {
			group = map[string]*plog.ResourceLogs{}
			resourceMap[logGroupName] = group
		}

		logStreamName := noStreamName
		if e.LogStreamName != nil {
			logStreamName = *e.LogStreamName
		}

		resourceLogs, ok := group[logStreamName]
		if !ok {
			rl := logs.ResourceLogs().AppendEmpty()
			resourceLogs = &rl
			resourceAttributes := resourceLogs.Resource().Attributes()
			resourceAttributes.PutStr("aws.region", l.region)
			resourceAttributes.PutStr("cloudwatch.log.group.name", logGroupName)
			if logStreamName != "" {
				resourceAttributes.PutStr("cloudwatch.log.stream", logStreamName)
			}
			group[logStreamName] = resourceLogs

			// Ensure one scopeLogs is initialized so we can handle in standardized way going forward.
			_ = resourceLogs.ScopeLogs().AppendEmpty()
		}

		// Now we know resourceLogs is initialized and has one scopeLogs so we don't have to handle any special cases.

		logRecord := resourceLogs.ScopeLogs().At(0).LogRecords().AppendEmpty()

		logRecord.SetObservedTimestamp(now)
		ts := time.UnixMilli(*e.Timestamp)
		logRecord.SetTimestamp(pcommon.NewTimestampFromTime(ts))
		logRecord.Body().SetStr(*e.Message)
		logRecord.Attributes().PutStr("id", *e.EventId)
	}
	return logs
}

func (l *logsReceiver) discoverGroups(ctx context.Context, auto *AutodiscoverConfig) ([]groupRequest, error) {
	l.settings.Logger.Debug("attempting to discover log groups.", zap.Int("limit", auto.Limit))
	groups := []groupRequest{}
	err := l.ensureSession()
	if err != nil {
		return groups, fmt.Errorf("unable to establish a session to auto discover log groups: %w", err)
	}

	numGroups := 0
	nextToken := aws.String("")
	for nextToken != nil {
		if numGroups >= auto.Limit {
			break
		}

		req := &cloudwatchlogs.DescribeLogGroupsInput{
			Limit: aws.Int32(maxLogGroupsPerDiscovery),
		}

		if len(*nextToken) > 0 {
			req.NextToken = nextToken
		}

		if auto.Prefix != "" {
			req.LogGroupNamePrefix = &auto.Prefix
		}

		dlgResults, err := l.client.DescribeLogGroups(ctx, req)
		if err != nil {
			return groups, fmt.Errorf("unable to list log groups: %w", err)
		}

		for _, lg := range dlgResults.LogGroups {
			if numGroups == auto.Limit {
				l.settings.Logger.Debug("reached limit of the number of log groups to discover."+
					"To increase the number of groups able to be discovered, please increase the autodiscover limit field.",
					zap.Int("groups_discovered", numGroups), zap.Int("limit", auto.Limit))
				break
			}

			numGroups++

			// default behavior is to collect all if not stream filtered
			if len(auto.Streams.Names) == 0 && len(auto.Streams.Prefixes) == 0 {
				groups = append(groups, &streamNames{group: *lg.LogGroupName})
				continue
			}

			for _, prefix := range auto.Streams.Prefixes {
				groups = append(groups, &streamPrefix{group: *lg.LogGroupName, prefix: prefix})
			}

			if len(auto.Streams.Names) > 0 {
				groups = append(groups, &streamNames{group: *lg.LogGroupName, names: auto.Streams.Names})
			}
		}
		nextToken = dlgResults.NextToken
	}
	return groups, nil
}

func (l *logsReceiver) ensureSession() error {
	if l.client != nil {
		return nil
	}

	cfgOptions := []func(*config.LoadOptions) error{
		config.WithRegion(l.region),
	}

	if l.imdsEndpoint != "" {
		cfgOptions = append(cfgOptions, config.WithEC2IMDSEndpoint(l.imdsEndpoint))
	}

	if l.profile != "" {
		cfgOptions = append(cfgOptions, config.WithSharedConfigProfile(l.profile))
	}

	cfg, err := config.LoadDefaultConfig(context.Background(), cfgOptions...)
	l.client = cloudwatchlogs.NewFromConfig(cfg)
	return err
}
