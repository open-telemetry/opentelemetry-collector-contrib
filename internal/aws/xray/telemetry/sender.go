// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray/telemetry"

import (
	"bytes"
	"context"
	"io"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/xray"
	"github.com/aws/aws-sdk-go-v2/service/xray/types"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
	awsxray "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"
)

const (
	envAWSHostname     = "AWS_HOSTNAME"
	envAWSInstanceID   = "AWS_INSTANCE_ID"
	metadataHostname   = "hostname"
	metadataInstanceID = "instance-id"

	defaultQueueSize = 30
	defaultBatchSize = 10
	defaultInterval  = time.Minute
)

// Sender wraps a Recorder and periodically sends the records.
type Sender interface {
	Recorder
	// Start send loop.
	Start(ctx context.Context)
	// Stop send loop.
	Stop()
}

type telemetrySender struct {
	// Recorder is the recorder wrapped by the sender.
	Recorder

	// logger is used to log dropped records.
	logger *zap.Logger
	// client is used to send the records.
	client awsxray.XRayClient

	resourceARN string
	instanceID  string
	hostname    string
	// interval is the amount of time between record rotation and sending attempts.
	interval time.Duration
	// queueSize is the capacity of the queue.
	queueSize int
	// batchSize is the max number of records sent in one request.
	batchSize int

	// queue is used to keep records that failed to send for retry during
	// the next period.
	queue []types.TelemetryRecord

	startOnce sync.Once
	stopWait  sync.WaitGroup
	stopOnce  sync.Once
	// stopCh is the channel used to stop the loop.
	stopCh chan struct{}
}

type Option interface {
	apply(ts *telemetrySender)
}

type optionFunc func(ts *telemetrySender)

func (o optionFunc) apply(ts *telemetrySender) {
	o(ts)
}

func WithResourceARN(resourceARN string) Option {
	return optionFunc(func(ts *telemetrySender) {
		ts.resourceARN = resourceARN
	})
}

func WithInstanceID(instanceID string) Option {
	return optionFunc(func(ts *telemetrySender) {
		ts.instanceID = instanceID
	})
}

func WithHostname(hostname string) Option {
	return optionFunc(func(ts *telemetrySender) {
		ts.hostname = hostname
	})
}

func WithLogger(logger *zap.Logger) Option {
	return optionFunc(func(ts *telemetrySender) {
		ts.logger = logger
	})
}

func WithInterval(interval time.Duration) Option {
	return optionFunc(func(ts *telemetrySender) {
		ts.interval = interval
	})
}

func WithQueueSize(queueSize int) Option {
	return optionFunc(func(ts *telemetrySender) {
		ts.queueSize = queueSize
	})
}

func WithBatchSize(batchSize int) Option {
	return optionFunc(func(ts *telemetrySender) {
		ts.batchSize = batchSize
	})
}

type metadataProvider interface {
	get(ctx context.Context) string
}

func getMetadata(ctx context.Context, providers ...metadataProvider) string {
	var metadata string
	for _, provider := range providers {
		if metadata = provider.get(ctx); metadata != "" {
			break
		}
	}
	return metadata
}

type simpleMetadataProvider struct {
	metadata string
}

func (p simpleMetadataProvider) get(_ context.Context) string {
	return p.metadata
}

type envMetadataProvider struct {
	envKey string
}

func (p envMetadataProvider) get(_ context.Context) string {
	return os.Getenv(p.envKey)
}

type ec2MetadataProvider struct {
	client      *imds.Client
	metadataKey string
}

func (p ec2MetadataProvider) get(ctx context.Context) string {
	resp, err := p.client.GetMetadata(ctx, &imds.GetMetadataInput{Path: p.metadataKey})
	if err != nil {
		return ""
	}

	var metadata bytes.Buffer
	if _, err := io.Copy(&metadata, resp.Content); err != nil {
		return ""
	}
	return metadata.String()
}

// ToOptions returns the metadata options if enabled by the config.
func ToOptions(ctx context.Context, cfg Config, awsConfig aws.Config, settings *awsutil.AWSSessionSettings) []Option {
	if !cfg.IncludeMetadata {
		return nil
	}
	metadataClient := imds.NewFromConfig(awsConfig)
	return []Option{
		WithHostname(getMetadata(
			ctx,
			simpleMetadataProvider{metadata: cfg.Hostname},
			envMetadataProvider{envKey: envAWSHostname},
			ec2MetadataProvider{client: metadataClient, metadataKey: metadataHostname},
		)),
		WithInstanceID(getMetadata(
			ctx,
			simpleMetadataProvider{metadata: cfg.InstanceID},
			envMetadataProvider{envKey: envAWSInstanceID},
			ec2MetadataProvider{client: metadataClient, metadataKey: metadataInstanceID},
		)),
		WithResourceARN(getMetadata(
			ctx,
			simpleMetadataProvider{metadata: cfg.ResourceARN},
			simpleMetadataProvider{metadata: settings.ResourceARN},
		)),
	}
}

// NewSender creates a new Sender with a default interval and queue size.
func NewSender(client awsxray.XRayClient, opts ...Option) Sender {
	return newSender(client, opts...)
}

func newSender(client awsxray.XRayClient, opts ...Option) *telemetrySender {
	sender := &telemetrySender{
		client:    client,
		interval:  defaultInterval,
		queueSize: defaultQueueSize,
		batchSize: defaultBatchSize,
		stopCh:    make(chan struct{}),
		Recorder:  NewRecorder(),
	}
	for _, opt := range opts {
		opt.apply(sender)
	}
	return sender
}

// Start starts the loop to send the records.
func (ts *telemetrySender) Start(ctx context.Context) {
	ts.startOnce.Do(func() {
		ts.stopWait.Go(func() {
			ts.run(ctx)
		})
	})
}

// Stop closes the stopCh channel to stop the loop.
func (ts *telemetrySender) Stop() {
	ts.stopOnce.Do(func() {
		close(ts.stopCh)
		ts.stopWait.Wait()
	})
}

// run sends the queued records once a minute if telemetry data was updated.
func (ts *telemetrySender) run(ctx context.Context) {
	ticker := time.NewTicker(ts.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ts.stopCh:
			return
		case <-ticker.C:
			if ts.HasRecording() {
				ts.enqueue(ts.Rotate())
				ts.send(ctx)
			}
		}
	}
}

// enqueue the record. If queue is full, drop the head of the queue and add.
func (ts *telemetrySender) enqueue(record types.TelemetryRecord) {
	for len(ts.queue) >= ts.queueSize {
		var dropped types.TelemetryRecord
		dropped, ts.queue = ts.queue[0], ts.queue[1:]
		if ts.logger != nil {
			ts.logger.Debug("queue full, dropping telemetry record", zap.Time("dropped_timestamp", *dropped.Timestamp))
		}
	}
	ts.queue = append(ts.queue, record)
}

// send the records in the queue in batches. Updates the queue.
func (ts *telemetrySender) send(ctx context.Context) {
	for i := len(ts.queue); i >= 0; i -= ts.batchSize {
		startIndex := max(i-ts.batchSize, 0)
		input := &xray.PutTelemetryRecordsInput{
			EC2InstanceId:    &ts.instanceID,
			Hostname:         &ts.hostname,
			ResourceARN:      &ts.resourceARN,
			TelemetryRecords: ts.queue[startIndex:i],
		}
		if _, err := ts.client.PutTelemetryRecords(ctx, input); err != nil {
			ts.RecordConnectionError(err)
			ts.queue = ts.queue[:i]
			return
		}
	}
	ts.queue = ts.queue[:0]
}
