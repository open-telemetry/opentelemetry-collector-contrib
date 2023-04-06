// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package telemetry // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray/telemetry"

import (
	"os"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/xray"
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
	Start()
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
	queue []*xray.TelemetryRecord

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
	get() string
}

func getMetadata(providers ...metadataProvider) string {
	var metadata string
	for _, provider := range providers {
		if metadata = provider.get(); metadata != "" {
			break
		}
	}
	return metadata
}

type simpleMetadataProvider struct {
	metadata string
}

func (p simpleMetadataProvider) get() string {
	return p.metadata
}

type envMetadataProvider struct {
	envKey string
}

func (p envMetadataProvider) get() string {
	return os.Getenv(p.envKey)
}

type ec2MetadataProvider struct {
	client      *ec2metadata.EC2Metadata
	metadataKey string
}

func (p ec2MetadataProvider) get() string {
	var metadata string
	if result, err := p.client.GetMetadata(p.metadataKey); err == nil {
		metadata = result
	}
	return metadata
}

// ToOptions returns the metadata options if enabled by the config.
func ToOptions(cfg Config, sess *session.Session, settings *awsutil.AWSSessionSettings) []Option {
	if !cfg.IncludeMetadata {
		return nil
	}
	metadataClient := ec2metadata.New(sess)
	return []Option{
		WithHostname(getMetadata(
			simpleMetadataProvider{metadata: cfg.Hostname},
			envMetadataProvider{envKey: envAWSHostname},
			ec2MetadataProvider{client: metadataClient, metadataKey: metadataHostname},
		)),
		WithInstanceID(getMetadata(
			simpleMetadataProvider{metadata: cfg.InstanceID},
			envMetadataProvider{envKey: envAWSInstanceID},
			ec2MetadataProvider{client: metadataClient, metadataKey: metadataInstanceID},
		)),
		WithResourceARN(getMetadata(
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
func (ts *telemetrySender) Start() {
	ts.startOnce.Do(func() {
		ts.stopWait.Add(1)
		go func() {
			defer ts.stopWait.Done()
			ts.run()
		}()
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
func (ts *telemetrySender) run() {
	ticker := time.NewTicker(ts.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ts.stopCh:
			return
		case <-ticker.C:
			if ts.HasRecording() {
				ts.enqueue(ts.Rotate())
				ts.send()
			}
		}
	}
}

// enqueue the record. If queue is full, drop the head of the queue and add.
func (ts *telemetrySender) enqueue(record *xray.TelemetryRecord) {
	for len(ts.queue) >= ts.queueSize {
		var dropped *xray.TelemetryRecord
		dropped, ts.queue = ts.queue[0], ts.queue[1:]
		if ts.logger != nil {
			ts.logger.Debug("queue full, dropping telemetry record", zap.Time("dropped_timestamp", *dropped.Timestamp))
		}
	}
	ts.queue = append(ts.queue, record)
}

// send the records in the queue in batches. Updates the queue.
func (ts *telemetrySender) send() {
	for i := len(ts.queue); i >= 0; i -= ts.batchSize {
		startIndex := i - ts.batchSize
		if startIndex < 0 {
			startIndex = 0
		}
		input := &xray.PutTelemetryRecordsInput{
			EC2InstanceId:    &ts.instanceID,
			Hostname:         &ts.hostname,
			ResourceARN:      &ts.resourceARN,
			TelemetryRecords: ts.queue[startIndex:i],
		}
		if _, err := ts.client.PutTelemetryRecords(input); err != nil {
			ts.RecordConnectionError(err)
			ts.queue = ts.queue[:i]
			return
		}
	}
	ts.queue = ts.queue[:0]
}
