// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver"

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

const (
	noStreamName = "THIS IS INVALID STREAM"
)

type logsReceiver struct {
	region              string
	profile             string
	imdsEndpoint        string
	pollInterval        time.Duration
	maxEventsPerRequest int
	nextStartTime       time.Time
	groupRequests       []groupRequest
	autodiscover        *AutodiscoverConfig
	logger              *zap.Logger
	client              client
	consumer            consumer.Logs
	wg                  *sync.WaitGroup
	doneChan            chan bool
}

const maxLogGroupsPerDiscovery = int64(50)

type client interface {
	DescribeLogGroupsWithContext(ctx context.Context, input *cloudwatchlogs.DescribeLogGroupsInput, opts ...request.Option) (*cloudwatchlogs.DescribeLogGroupsOutput, error)
	FilterLogEventsWithContext(ctx context.Context, input *cloudwatchlogs.FilterLogEventsInput, opts ...request.Option) (*cloudwatchlogs.FilterLogEventsOutput, error)
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
		Limit:        aws.Int64(int64(limit)),
	}
	if len(sn.names) > 0 {
		base.LogStreamNames = sn.names
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
		Limit:               aws.Int64(int64(limit)),
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

func newLogsReceiver(cfg *Config, logger *zap.Logger, consumer consumer.Logs) *logsReceiver {
	groups := []groupRequest{}
	for logGroupName, sc := range cfg.Logs.Groups.NamedConfigs {
		for _, prefix := range sc.Prefixes {
			groups = append(groups, &streamPrefix{group: logGroupName, prefix: prefix})
		}
		if len(sc.Names) > 0 {
			groups = append(groups, &streamNames{group: logGroupName, names: sc.Names})
		}
	}

	// safeguard from using both
	autodiscover := cfg.Logs.Groups.AutodiscoverConfig
	if len(cfg.Logs.Groups.NamedConfigs) > 0 {
		autodiscover = nil
	}

	return &logsReceiver{
		region:              cfg.Region,
		profile:             cfg.Profile,
		consumer:            consumer,
		maxEventsPerRequest: cfg.Logs.MaxEventsPerRequest,
		imdsEndpoint:        cfg.IMDSEndpoint,
		autodiscover:        autodiscover,
		pollInterval:        cfg.Logs.PollInterval,
		nextStartTime:       time.Now().Add(-cfg.Logs.PollInterval),
		groupRequests:       groups,
		logger:              logger,
		wg:                  &sync.WaitGroup{},
		doneChan:            make(chan bool),
	}
}

func (l *logsReceiver) Start(ctx context.Context, _ component.Host) error {
	l.logger.Debug("starting to poll for Cloudwatch logs")
	l.wg.Add(1)
	go l.startPolling(ctx)
	return nil
}

func (l *logsReceiver) Shutdown(_ context.Context) error {
	l.logger.Debug("shutting down logs receiver")
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
					l.logger.Error("unable to perform discovery of log groups", zap.Error(err))
					continue
				}
				l.groupRequests = group
			}

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
	endTime := time.Now()
	for _, r := range l.groupRequests {
		if err := l.pollForLogs(ctx, r, startTime, endTime); err != nil {
			errs = multierr.Append(errs, err)
		}
	}
	l.nextStartTime = endTime
	return errs
}

func (l *logsReceiver) pollForLogs(ctx context.Context, pc groupRequest, startTime, endTime time.Time) error {
	err := l.ensureSession()
	if err != nil {
		return err
	}
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
			resp, err := l.client.FilterLogEventsWithContext(ctx, input)
			if err != nil {
				l.logger.Error("unable to retrieve logs from cloudwatch", zap.String("log group", pc.groupName()), zap.Error(err))
				break
			}
			observedTime := pcommon.NewTimestampFromTime(time.Now())
			logs := l.processEvents(observedTime, pc.groupName(), resp)
			if logs.LogRecordCount() > 0 {
				if err = l.consumer.ConsumeLogs(ctx, logs); err != nil {
					l.logger.Error("unable to consume logs", zap.Error(err))
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
			l.logger.Error("unable to determine timestamp of event as the timestamp is nil")
			continue
		}

		if e.EventId == nil {
			l.logger.Error("no event ID was present on the event, skipping entry")
			continue
		}

		if e.Message == nil {
			l.logger.Error("no message was present on the event", zap.String("event.id", *e.EventId))
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
			resourceAttributes.PutStr("cloudwatch.log.stream", logStreamName)
			group[logStreamName] = resourceLogs

			// Ensure one scopeLogs is initialized so we can handle in standardized way going forward.
			_ = resourceLogs.ScopeLogs().AppendEmpty()
		}

		// Now we know resourceLogs is initialized and has one scopeLogs so we don't have to handle any special cases.

		sl := resourceLogs.ScopeLogs()
		logRecord := sl.At(0).LogRecords().AppendEmpty()

		logRecord.SetObservedTimestamp(now)
		ts := time.UnixMilli(*e.Timestamp)
		logRecord.SetTimestamp(pcommon.NewTimestampFromTime(ts))
		logRecord.Body().SetStr(*e.Message)
		logRecord.Attributes().PutStr("id", *e.EventId)
	}
	return logs
}

func (l *logsReceiver) discoverGroups(ctx context.Context, auto *AutodiscoverConfig) ([]groupRequest, error) {
	l.logger.Debug("attempting to discover log groups.", zap.Int("limit", auto.Limit))
	groups := []groupRequest{}
	err := l.ensureSession()
	if err != nil {
		return groups, fmt.Errorf("unable to establish a session to auto discover log groups: %w", err)
	}

	numGroups := 0
	var nextToken = aws.String("")
	for nextToken != nil {
		if numGroups >= auto.Limit {
			break
		}

		req := &cloudwatchlogs.DescribeLogGroupsInput{
			Limit: aws.Int64(maxLogGroupsPerDiscovery),
		}

		if auto.Prefix != "" {
			req.LogGroupNamePrefix = &auto.Prefix
		}

		dlgResults, err := l.client.DescribeLogGroupsWithContext(ctx, req)
		if err != nil {
			return groups, fmt.Errorf("unable to list log groups: %w", err)
		}

		for _, lg := range dlgResults.LogGroups {
			if numGroups == auto.Limit {
				l.logger.Debug("reached limit of the number of log groups to discover."+
					"To increase the number of groups able to be discovered, please increase the autodiscover limit field.",
					zap.Int("groups_discovered", numGroups), zap.Int("limit", auto.Limit))
				break
			}

			numGroups++
			l.logger.Debug("discovered log group", zap.String("log group", lg.GoString()))
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
	awsConfig := aws.NewConfig().WithRegion(l.region)
	options := session.Options{
		Config: *awsConfig,
	}
	if l.imdsEndpoint != "" {
		options.EC2IMDSEndpoint = l.imdsEndpoint
	}
	if l.profile != "" {
		options.Profile = l.profile
	}
	s, err := session.NewSessionWithOptions(options)
	l.client = cloudwatchlogs.New(s)
	return err
}
