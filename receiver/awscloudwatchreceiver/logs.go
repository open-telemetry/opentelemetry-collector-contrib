// Copyright  The OpenTelemetry Authors
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

package awscloudwatchreceiver

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
	"go.uber.org/zap"
)

type logsReceiver struct {
	region        string
	profile       string
	groupPrefix   string
	groups        []LogGroupConfig
	eventLimit    *int64
	logGroupLimit int64
	pollInterval  time.Duration
	logger        *zap.Logger
	client        client
	consumer      consumer.Logs

	wg       *sync.WaitGroup
	doneChan chan bool
}

type client interface {
	DescribeLogGroupsWithContext(ctx context.Context, input *cloudwatchlogs.DescribeLogGroupsInput, opts ...request.Option) (*cloudwatchlogs.DescribeLogGroupsOutput, error)
	FilterLogEventsWithContext(ctx context.Context, input *cloudwatchlogs.FilterLogEventsInput, opts ...request.Option) (*cloudwatchlogs.FilterLogEventsOutput, error)
}

func newLogsReceiver(cfg *Config, logger *zap.Logger, consumer consumer.Logs) *logsReceiver {
	return &logsReceiver{
		region:        cfg.Region,
		profile:       cfg.Profile,
		consumer:      consumer,
		logGroupLimit: cfg.Logs.LogGroupLimit,
		groupPrefix:   cfg.Logs.LogGroupPrefix,
		groups:        cfg.Logs.LogGroups,
		pollInterval:  cfg.Logs.PollInterval,
		eventLimit:    &cfg.Logs.EventLimit,
		logger:        logger,
		wg:            &sync.WaitGroup{},
		doneChan:      make(chan bool),
	}
}

func (l *logsReceiver) Start(ctx context.Context, host component.Host) error {
	l.logger.Debug("starting to poll for Cloudwatch logs")
	if len(l.groups) == 0 {
		err := l.discoverGroups(ctx)
		if err != nil {
			l.logger.Error("unable to auto discover log groups using prefix", zap.String("prefix", l.groupPrefix), zap.Error(err))
		}
	}

	for _, lg := range l.groups {
		l.wg.Add(1)
		go l.poll(ctx, lg)
	}
	return nil
}

func (l *logsReceiver) Shutdown(ctx context.Context) error {
	l.logger.Debug("shutting down logs receiver")
	close(l.doneChan)
	l.wg.Wait()
	return nil
}

func (l *logsReceiver) poll(ctx context.Context, logGroup LogGroupConfig) {
	defer l.wg.Done()

	t := time.NewTicker(l.pollInterval)
	for {
		select {
		case <-ctx.Done():
			return
		case <-l.doneChan:
			return
		case <-t.C:
			err := l.pollForLogs(ctx, logGroup)
			if err != nil {
				l.logger.Error("there was an error during the poll", zap.String("log group", logGroup.Name), zap.Error(err))
			}
		}
	}
}

func (l *logsReceiver) pollForLogs(ctx context.Context, logGroup LogGroupConfig) error {
	err := l.ensureSession()
	if err != nil {
		return err
	}

	var token = ""
	nextToken := &token
	for nextToken != nil {
		observedTime := pcommon.NewTimestampFromTime(time.Now())
		input := l.logEventsRequest(nextToken, logGroup)
		resp, err := l.client.FilterLogEventsWithContext(ctx, input)
		if err != nil {
			l.logger.Error("unable to retrieve logs from cloudwatch", zap.Error(err))
			break
		}

		logs := l.processEvents(observedTime, logGroup, resp)
		if err != nil {
			l.logger.Error("unable to process events", zap.Error(err))
			break
		}

		if logs.LogRecordCount() > 0 {
			if err = l.consumer.ConsumeLogs(ctx, logs); err != nil {
				l.logger.Error("unable to consume logs", zap.Error(err))
				break
			}
		}

		nextToken = resp.NextToken
	}
	return nil
}

func (l *logsReceiver) logEventsRequest(token *string, lg LogGroupConfig) *cloudwatchlogs.FilterLogEventsInput {
	// this sliding window may need to be explored more
	st := time.Now().Add(-l.pollInterval).UnixMilli()
	et := time.Now().UnixMilli()

	baseReq := &cloudwatchlogs.FilterLogEventsInput{
		Limit:        l.eventLimit,
		LogGroupName: &lg.Name,
		StartTime:    &st,
		EndTime:      &et,
	}

	if token != nil && *token != "" {
		baseReq.NextToken = token
	}

	// no specification, collect all
	if len(lg.LogStreams) == 0 && lg.StreamPrefix == "" {
		return baseReq
	}

	if len(lg.LogStreams) > 0 {
		baseReq.LogStreamNames = lg.LogStreams
	}

	if lg.StreamPrefix != "" {
		baseReq.LogStreamNamePrefix = &lg.StreamPrefix
	}

	return baseReq
}

func (l *logsReceiver) processEvents(now pcommon.Timestamp, lc LogGroupConfig, output *cloudwatchlogs.FilterLogEventsOutput) plog.Logs {
	logs := plog.NewLogs()
	for _, e := range output.Events {
		if e.Timestamp == nil {
			l.logger.Error("unable to determine timestamp of event as the timestamp is nil")
			continue
		}

		if e.Message == nil {
			l.logger.Error("no message was present on the event", zap.Any("event.id", e.EventId))
			continue
		}

		rl := logs.ResourceLogs().AppendEmpty()
		resourceAttributes := rl.Resource().Attributes()

		safePutAttribute("aws.region", &l.region, resourceAttributes.PutString)
		safePutAttribute("cloudwatch.log.group.name", &lc.Name, resourceAttributes.PutString)
		safePutAttribute("cloudwatch.log.stream", e.LogStreamName, resourceAttributes.PutString)

		logRecord := rl.ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
		logRecord.SetObservedTimestamp(now)
		ts := time.UnixMilli(*e.Timestamp)
		logRecord.SetTimestamp(pcommon.NewTimestampFromTime(ts))

		logRecord.Body().SetStr(*e.Message)
		safePutAttribute("id", e.EventId, logRecord.Attributes().PutString)
	}
	return logs
}

func (l *logsReceiver) discoverGroups(ctx context.Context) error {
	err := l.ensureSession()
	if err != nil {
		return fmt.Errorf("unable to establish a session to auto discover log groups: %w", err)
	}
	var nextToken = aws.String("")
	newGroups := []LogGroupConfig{}
	for nextToken != nil || len(l.groups) >= int(l.logGroupLimit) {
		req := &cloudwatchlogs.DescribeLogGroupsInput{
			Limit: &l.logGroupLimit,
		}
		if l.groupPrefix != "" {
			req.LogGroupNamePrefix = &l.groupPrefix
		}

		dlgResults, err := l.client.DescribeLogGroupsWithContext(ctx, req)
		if err != nil {
			return fmt.Errorf("unable to list log groups: %w", err)
		}

		for _, lg := range dlgResults.LogGroups {
			l.logger.Debug("discovered log group", zap.String("log group", lg.GoString()))
			newGroups = append(l.groups, LogGroupConfig{
				Name:         *lg.LogGroupName,
				StreamPrefix: "",
				LogStreams:   make([]*string, 0),
			})
		}
		nextToken = dlgResults.NextToken
	}
	l.groups = newGroups
	return nil
}

func (l *logsReceiver) ensureSession() error {
	if l.client != nil {
		return nil
	}
	awsConfig := aws.NewConfig().WithRegion(l.region)
	if l.profile != "" {
		s, err := session.NewSessionWithOptions(session.Options{
			Config:  *awsConfig,
			Profile: l.profile,
		})
		l.client = cloudwatchlogs.New(s)
		return err
	}
	s, err := session.NewSession(awsConfig)
	l.client = cloudwatchlogs.New(s)
	return err
}

func safePutAttribute(key string, ptr *string, attributeAddFn func(string, string)) {
	if ptr != nil {
		attributeAddFn(key, *ptr)
	}
}
