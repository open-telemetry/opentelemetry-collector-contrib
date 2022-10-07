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
	region              string
	profile             string
	pollInterval        time.Duration
	maxEventsPerRequest int64
	namedPolls          []namesPollConfig
	prefixedPolls       []prefixPollConfig
	autodiscover        *AutodiscoverConfig
	logger              *zap.Logger
	client              client
	consumer            consumer.Logs

	wg       *sync.WaitGroup
	doneChan chan bool
}

type client interface {
	DescribeLogGroupsWithContext(ctx context.Context, input *cloudwatchlogs.DescribeLogGroupsInput, opts ...request.Option) (*cloudwatchlogs.DescribeLogGroupsOutput, error)
	FilterLogEventsWithContext(ctx context.Context, input *cloudwatchlogs.FilterLogEventsInput, opts ...request.Option) (*cloudwatchlogs.FilterLogEventsOutput, error)
}

type namesPollConfig struct {
	logGroup    string
	streamNames []*string
}

func (npc *namesPollConfig) request(limit int64, st, et *time.Time) *cloudwatchlogs.FilterLogEventsInput {
	base := &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName: &npc.logGroup,
		StartTime:    aws.Int64(st.UnixMilli()),
		EndTime:      aws.Int64(et.UnixMilli()),
		Limit:        aws.Int64(limit),
	}
	if len(npc.streamNames) > 0 {
		base.LogStreamNames = npc.streamNames
	}
	return base
}

type prefixPollConfig struct {
	logGroup string
	prefix   *string
}

func (ppc *prefixPollConfig) request(limit int64, st, et *time.Time) *cloudwatchlogs.FilterLogEventsInput {
	return &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName:        &ppc.logGroup,
		StartTime:           aws.Int64(st.UnixMilli()),
		EndTime:             aws.Int64(et.UnixMilli()),
		Limit:               aws.Int64(limit),
		LogStreamNamePrefix: ppc.prefix,
	}
}

type pollConfig interface {
	request(limit int64, startTime, endTime *time.Time) *cloudwatchlogs.FilterLogEventsInput
}

func newLogsReceiver(cfg *Config, logger *zap.Logger, consumer consumer.Logs) *logsReceiver {
	namedPolls := []namesPollConfig{}
	prefixedPolls := []prefixPollConfig{}
	for logGroupName, sc := range cfg.Logs.Groups.NamedConfigs {
		for _, prefix := range sc.Prefixes {
			prefixedPolls = append(prefixedPolls, prefixPollConfig{
				logGroup: logGroupName,
				prefix:   prefix,
			})
		}

		namedPolls = append(namedPolls, namesPollConfig{
			logGroup:    logGroupName,
			streamNames: sc.Names,
		})
	}

	return &logsReceiver{
		region:              cfg.Region,
		profile:             cfg.Profile,
		consumer:            consumer,
		maxEventsPerRequest: cfg.Logs.MaxEventsPerRequest,
		autodiscover:        cfg.Logs.Groups.AutodiscoverConfig,
		pollInterval:        cfg.Logs.PollInterval,
		namedPolls:          namedPolls,
		prefixedPolls:       prefixedPolls,
		logger:              logger,
		wg:                  &sync.WaitGroup{},
		doneChan:            make(chan bool),
	}
}

func (l *logsReceiver) Start(ctx context.Context, host component.Host) error {
	l.logger.Debug("starting to poll for Cloudwatch logs")

	if l.autodiscover != nil {
		namedPolls, prefixedPolls, err := l.discoverGroups(ctx, l.autodiscover)
		if err != nil {
			return fmt.Errorf("unable to auto discover log groups using prefix  %s: %w", l.autodiscover.Prefix, err)
		}
		l.namedPolls = namedPolls
		l.prefixedPolls = prefixedPolls
	}

	for _, np := range l.namedPolls {
		l.wg.Add(1)
		go l.poll(ctx, np.logGroup, &np)
	}

	for _, pp := range l.prefixedPolls {
		l.wg.Add(1)
		go l.poll(ctx, pp.logGroup, &pp)
	}

	return nil
}

func (l *logsReceiver) Shutdown(ctx context.Context) error {
	l.logger.Debug("shutting down logs receiver")
	close(l.doneChan)
	l.wg.Wait()
	return nil
}

func (l *logsReceiver) poll(ctx context.Context, logGroup string, pc pollConfig) {
	defer l.wg.Done()

	t := time.NewTicker(l.pollInterval)
	for {
		select {
		case <-ctx.Done():
			return
		case <-l.doneChan:
			return
		case <-t.C:
			err := l.pollForLogs(ctx, logGroup, pc)
			if err != nil {
				l.logger.Error("there was an error during the poll", zap.String("log group", logGroup), zap.Error(err))
			}
		}
	}
}

func (l *logsReceiver) pollForLogs(ctx context.Context, logGroup string, pc pollConfig) error {
	err := l.ensureSession()
	if err != nil {
		return err
	}
	nextToken := aws.String("")
	for nextToken != nil {
		select {
		// if done, we want to stop processing paginated stream
		case _, ok := <-l.doneChan:
			if !ok {
				return nil
			}
		default:
			startTime := time.Now().Add(-l.pollInterval)
			endTime := time.Now()
			input := pc.request(l.maxEventsPerRequest, &startTime, &endTime)
			resp, err := l.client.FilterLogEventsWithContext(ctx, input)
			if err != nil {
				l.logger.Error("unable to retrieve logs from cloudwatch", zap.String("log group", logGroup), zap.Error(err))
				break
			}
			observedTime := pcommon.NewTimestampFromTime(time.Now())
			logs := l.processEvents(observedTime, logGroup, resp)
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
		safePutAttribute("cloudwatch.log.group.name", &logGroupName, resourceAttributes.PutString)
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

func (l *logsReceiver) discoverGroups(ctx context.Context, auto *AutodiscoverConfig) ([]namesPollConfig, []prefixPollConfig, error) {
	l.logger.Debug("attempting to discover log groups.", zap.Int64("limit", l.maxEventsPerRequest))

	newNamedPolls := []namesPollConfig{}
	newPrefixedPolls := []prefixPollConfig{}

	err := l.ensureSession()
	if err != nil {
		return newNamedPolls, newPrefixedPolls, fmt.Errorf("unable to establish a session to auto discover log groups: %w", err)
	}

	numGroups := 0
	var nextToken = aws.String("")
	for nextToken != nil {
		req := &cloudwatchlogs.DescribeLogGroupsInput{
			Limit: &auto.Limit,
		}

		if auto.Prefix != "" {
			req.LogGroupNamePrefix = &auto.Prefix
		}

		dlgResults, err := l.client.DescribeLogGroupsWithContext(ctx, req)
		if err != nil {
			return newNamedPolls, newPrefixedPolls, fmt.Errorf("unable to list log groups: %w", err)
		}

		for _, lg := range dlgResults.LogGroups {
			numGroups++
			l.logger.Debug("discovered log group", zap.String("log group", lg.GoString()))
			// default behavior is to collect all if not stream filtered
			if len(auto.Streams.Names) == 0 && len(auto.Streams.Prefixes) == 0 {
				newNamedPolls = append(newNamedPolls, namesPollConfig{
					logGroup: *lg.LogGroupName,
				})
				continue
			}

			for _, prefix := range auto.Streams.Prefixes {
				newPrefixedPolls = append(newPrefixedPolls, prefixPollConfig{
					logGroup: *lg.LogGroupName,
					prefix:   prefix,
				})
			}

			if len(auto.Streams.Names) > 0 {
				newNamedPolls = append(newNamedPolls,
					namesPollConfig{
						logGroup:    *lg.LogGroupName,
						streamNames: auto.Streams.Names,
					},
				)
			}
		}
		nextToken = dlgResults.NextToken
	}

	return newNamedPolls, newPrefixedPolls, nil
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
