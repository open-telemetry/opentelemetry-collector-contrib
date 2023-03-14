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
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/xray"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil"
	awsxray "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"
)

const (
	envAWSHostname   = "AWS_HOSTNAME"
	envAWSInstanceID = "AWS_INSTANCE_ID"

	queueSize       = 30
	batchSize       = 10
	defaultInterval = time.Minute
)

type Recorder interface {
	// Start send loop.
	Start()
	// Stop send loop.
	Stop()
	// RecordSegmentsReceived adds the count to the record for the current period.
	RecordSegmentsReceived(count int)
	// RecordSegmentsSent adds the count to the record for the current period.
	RecordSegmentsSent(count int)
	// RecordSegmentsSpillover adds the count to the record for the current period.
	RecordSegmentsSpillover(count int)
	// RecordSegmentsRejected adds the count to the record for the current period.
	RecordSegmentsRejected(count int)
	// RecordConnectionError categorizes the error and increments the count by one for the current period.
	RecordConnectionError(err error)
}

type RecorderOption interface {
	apply(r *telemetryRecorder)
}

type recorderOptionFunc func(r *telemetryRecorder)

func (ro recorderOptionFunc) apply(r *telemetryRecorder) {
	ro(r)
}

func WithResourceARN(resourceARN string) RecorderOption {
	return recorderOptionFunc(func(r *telemetryRecorder) {
		r.resourceARN = resourceARN
	})
}

func WithInstanceID(instanceID string) RecorderOption {
	return recorderOptionFunc(func(r *telemetryRecorder) {
		r.instanceID = instanceID
	})
}

func WithHostname(hostname string) RecorderOption {
	return recorderOptionFunc(func(r *telemetryRecorder) {
		r.hostname = hostname
	})
}

func WithLogger(logger *zap.Logger) RecorderOption {
	return recorderOptionFunc(func(r *telemetryRecorder) {
		r.logger = logger
	})
}

// ToRecorderOptions gets the metadata recorder options if enabled by the config.
func ToRecorderOptions(cfg Config, sess *session.Session, settings *awsutil.AWSSessionSettings) []RecorderOption {
	if !cfg.IncludeMetadata {
		return nil
	}
	hostname := os.Getenv(envAWSHostname)
	instanceID := os.Getenv(envAWSInstanceID)
	if !settings.LocalMode {
		metadataClient := ec2metadata.New(sess)
		if hostname == "" {
			if result, err := metadataClient.GetMetadata("hostname"); err == nil {
				hostname = result
			}
		}
		if instanceID == "" {
			if result, err := metadataClient.GetMetadata("instance-id"); err == nil {
				instanceID = result
			}
		}
	}
	return []RecorderOption{WithResourceARN(settings.ResourceARN), WithHostname(hostname), WithInstanceID(instanceID)}
}

type telemetryRecorder struct {
	resourceARN string
	instanceID  string
	hostname    string

	logger *zap.Logger
	// client is used to send the records.
	client awsxray.XRayClient
	// record is the pointer to the count metrics for the current period.
	record *xray.TelemetryRecord
	// mu is the lock used when updating the record. Primarily exists to
	// prevent writes while swapping the record out in cutoff.
	mu sync.Mutex

	// queue is used to keep records that failed to send for retry during
	// the next period.
	queue chan *xray.TelemetryRecord
	// done is the channel used to stop the loop.
	done chan struct{}
	// interval is the amount of time between flushes.
	interval time.Duration

	// recordUpdated is set to true when any count is updated. Indicates
	// that telemetry data is available.
	recordUpdated *atomic.Bool

	startOnce sync.Once
	stopOnce  sync.Once
}

func newTelemetryRecorder(
	client awsxray.XRayClient,
	opts ...RecorderOption,
) *telemetryRecorder {
	recorder := &telemetryRecorder{
		client:        client,
		record:        newTelemetryRecord(),
		recordUpdated: &atomic.Bool{},
		interval:      defaultInterval,
		done:          make(chan struct{}),
		queue:         make(chan *xray.TelemetryRecord, queueSize),
	}
	for _, opt := range opts {
		opt.apply(recorder)
	}
	return recorder
}

func newTelemetryRecord() *xray.TelemetryRecord {
	return &xray.TelemetryRecord{
		SegmentsReceivedCount:  aws.Int64(0),
		SegmentsRejectedCount:  aws.Int64(0),
		SegmentsSentCount:      aws.Int64(0),
		SegmentsSpilloverCount: aws.Int64(0),
		BackendConnectionErrors: &xray.BackendConnectionErrors{
			HTTPCode4XXCount:       aws.Int64(0),
			HTTPCode5XXCount:       aws.Int64(0),
			ConnectionRefusedCount: aws.Int64(0),
			OtherCount:             aws.Int64(0),
			TimeoutCount:           aws.Int64(0),
			UnknownHostCount:       aws.Int64(0),
		},
	}
}

// Start starts the loop to send the records.
func (t *telemetryRecorder) Start() {
	t.startOnce.Do(func() {
		go t.start()
	})
}

// Stop closes the done channel to stop the loop.
func (t *telemetryRecorder) Stop() {
	t.stopOnce.Do(func() {
		close(t.done)
	})
}

// start flushes the record once a minute if telemetry
// data was recordUpdated.
func (t *telemetryRecorder) start() {
	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if t.recordUpdated.Load() {
				t.add(t.cutoff())
				t.recordUpdated.Store(false)
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

// cutoff the current record and swap it out with a new record.
// Sets the timestamp and returns the old record.
func (t *telemetryRecorder) cutoff() *xray.TelemetryRecord {
	t.mu.Lock()
	oldRecord := t.record
	t.record = newTelemetryRecord()
	t.mu.Unlock()
	oldRecord.SetTimestamp(time.Now())
	return oldRecord
}

// add to the queue. If queue is full, drop the head of the queue and try again.
func (t *telemetryRecorder) add(record *xray.TelemetryRecord) {
	for {
		select {
		case t.queue <- record:
			return
		case <-t.done:
			return
		default:
			dropped := <-t.queue
			if t.logger != nil {
				t.logger.Debug("queue full, dropping telemetry record", zap.Time("dropped_timestamp", *dropped.Timestamp))
			}
		}
	}
}

// flush the queue into a slice of records.
func (t *telemetryRecorder) flush() []*xray.TelemetryRecord {
	var records []*xray.TelemetryRecord
	for len(t.queue) > 0 {
		records = append(records, <-t.queue)
	}
	return records
}

// send the records in batches of batchSize. Returns the error and records it was unable to send
// if the PutTelemetryRecords call fails.
func (t *telemetryRecorder) send(records []*xray.TelemetryRecord) ([]*xray.TelemetryRecord, error) {
	if len(records) > 0 {
		for i := 0; i < len(records); i += batchSize {
			endIndex := i + batchSize
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

func (t *telemetryRecorder) RecordSegmentsReceived(count int) {
	t.mu.Lock()
	atomic.AddInt64(t.record.SegmentsReceivedCount, int64(count))
	t.mu.Unlock()
	t.recordUpdated.Store(true)
}

func (t *telemetryRecorder) RecordSegmentsSent(count int) {
	t.mu.Lock()
	atomic.AddInt64(t.record.SegmentsSentCount, int64(count))
	t.mu.Unlock()
	t.recordUpdated.Store(true)
}

func (t *telemetryRecorder) RecordSegmentsSpillover(count int) {
	t.mu.Lock()
	atomic.AddInt64(t.record.SegmentsSpilloverCount, int64(count))
	t.mu.Unlock()
	t.recordUpdated.Store(true)
}

func (t *telemetryRecorder) RecordSegmentsRejected(count int) {
	t.mu.Lock()
	atomic.AddInt64(t.record.SegmentsRejectedCount, int64(count))
	t.mu.Unlock()
	t.recordUpdated.Store(true)
}

func (t *telemetryRecorder) RecordConnectionError(err error) {
	if err == nil {
		return
	}
	var requestFailure awserr.RequestFailure
	if ok := errors.As(err, &requestFailure); ok {
		switch requestFailure.StatusCode() / 100 {
		case 5:
			t.recordConnectionHTTPCode5XX(1)
		case 4:
			t.recordConnectionHTTPCode4XX(1)
		default:
			t.recordConnectionOther(1)
		}
	} else {
		var awsError awserr.Error
		if ok = errors.As(err, &awsError); ok {
			switch awsError.Code() {
			case request.ErrCodeResponseTimeout:
				t.recordConnectionTimeout(1)
			case request.ErrCodeRequestError:
				t.recordConnectionUnknownHost(1)
			default:
				t.recordConnectionOther(1)
			}
		} else {
			t.recordConnectionOther(1)
		}
	}
}

func (t *telemetryRecorder) recordConnectionHTTPCode5XX(count int) {
	t.mu.Lock()
	atomic.AddInt64(t.record.BackendConnectionErrors.HTTPCode5XXCount, int64(count))
	t.mu.Unlock()
	t.recordUpdated.Store(true)
}

func (t *telemetryRecorder) recordConnectionHTTPCode4XX(count int) {
	t.mu.Lock()
	atomic.AddInt64(t.record.BackendConnectionErrors.HTTPCode4XXCount, int64(count))
	t.mu.Unlock()
	t.recordUpdated.Store(true)
}

func (t *telemetryRecorder) recordConnectionTimeout(count int) {
	t.mu.Lock()
	atomic.AddInt64(t.record.BackendConnectionErrors.TimeoutCount, int64(count))
	t.mu.Unlock()
	t.recordUpdated.Store(true)
}

func (t *telemetryRecorder) recordConnectionUnknownHost(count int) {
	t.mu.Lock()
	atomic.AddInt64(t.record.BackendConnectionErrors.UnknownHostCount, int64(count))
	t.mu.Unlock()
	t.recordUpdated.Store(true)
}

func (t *telemetryRecorder) recordConnectionOther(count int) {
	t.mu.Lock()
	atomic.AddInt64(t.record.BackendConnectionErrors.OtherCount, int64(count))
	t.mu.Unlock()
	t.recordUpdated.Store(true)
}
