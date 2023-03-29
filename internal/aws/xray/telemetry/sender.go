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

// Sender wraps a Recorder and periodically sends the records.
type Sender interface {
	Recorder
	// Start send loop.
	Start()
	// Stop send loop.
	Stop()
}

type telemetrySender struct {
	Recorder

	resourceARN string
	instanceID  string
	hostname    string

	// logger is used to log dropped records.
	logger *zap.Logger
	// client is used to send the records.
	client awsxray.XRayClient

	// queue is used to keep records that failed to send for retry during
	// the next period.
	queue chan *xray.TelemetryRecord
	// done is the channel used to stop the loop.
	done chan struct{}
	// interval is the amount of time between flushes.
	interval time.Duration
	// queueSize is the capacity of the queue.
	queueSize int
	// batchSize is the max number of records sent together.
	batchSize int

	startOnce sync.Once
	stopOnce  sync.Once
}

type Option interface {
	apply(r *telemetrySender)
}

type pusherOptionFunc func(r *telemetrySender)

func (o pusherOptionFunc) apply(r *telemetrySender) {
	o(r)
}

func WithResourceARN(resourceARN string) Option {
	return pusherOptionFunc(func(r *telemetrySender) {
		r.resourceARN = resourceARN
	})
}

func WithInstanceID(instanceID string) Option {
	return pusherOptionFunc(func(r *telemetrySender) {
		r.instanceID = instanceID
	})
}

func WithHostname(hostname string) Option {
	return pusherOptionFunc(func(r *telemetrySender) {
		r.hostname = hostname
	})
}

func WithLogger(logger *zap.Logger) Option {
	return pusherOptionFunc(func(r *telemetrySender) {
		r.logger = logger
	})
}

func WithInterval(interval time.Duration) Option {
	return pusherOptionFunc(func(r *telemetrySender) {
		r.interval = interval
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
		done:      make(chan struct{}),
		Recorder:  NewRecorder(),
	}
	for _, opt := range opts {
		opt.apply(sender)
	}
	sender.queue = make(chan *xray.TelemetryRecord, sender.queueSize)
	return sender
}

// Start starts the loop to send the records.
func (t *telemetrySender) Start() {
	t.startOnce.Do(func() {
		go t.start()
	})
}

// Stop closes the done channel to stop the loop.
func (t *telemetrySender) Stop() {
	t.stopOnce.Do(func() {
		close(t.done)
	})
}

// start flushes the record once a minute if telemetry data was updated.
func (t *telemetrySender) start() {
	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if t.HasRecording() {
				t.add(t.Rotate())
				if failedToSend, err := t.send(t.flush()); err != nil {
					for _, record := range failedToSend {
						t.add(record)
					}
				}
			}
		case <-t.done:
			return
		}
	}
}

// add to the queue. If queue is full, drop the head of the queue and try again.
func (t *telemetrySender) add(record *xray.TelemetryRecord) {
	select {
	case t.queue <- record:
	case <-t.done:
	default:
		dropped := <-t.queue
		if t.logger != nil {
			t.logger.Debug("queue full, dropping telemetry record", zap.Time("dropped_timestamp", *dropped.Timestamp))
		}
		t.queue <- record
	}
}

// flush the queue into a slice of records.
func (t *telemetrySender) flush() []*xray.TelemetryRecord {
	var records []*xray.TelemetryRecord
	for len(t.queue) > 0 {
		records = append(records, <-t.queue)
	}
	return records
}

// send the records in batches. Returns the error and unsuccessfully sent records
// if the PutTelemetryRecords call fails.
func (t *telemetrySender) send(records []*xray.TelemetryRecord) ([]*xray.TelemetryRecord, error) {
	if len(records) > 0 {
		for i := 0; i < len(records); i += t.batchSize {
			endIndex := i + t.batchSize
			if endIndex > len(records) {
				endIndex = len(records)
			}
			input := &xray.PutTelemetryRecordsInput{
				EC2InstanceId:    &t.instanceID,
				Hostname:         &t.hostname,
				ResourceARN:      &t.resourceARN,
				TelemetryRecords: records[i:endIndex],
			}
			_, err := t.client.PutTelemetryRecords(input)
			if err != nil {
				t.RecordConnectionError(err)
				return records[i:], err
			}
		}
	}
	return nil, nil
}
