// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver/internal"

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"go.uber.org/zap"
)

// Types

// CustomTriggerHandler defines the generic contract for custom event handling
// Custom events are expected to be consumed by configured event handlers.
// Contract includes,
//   - Implements Iterator to iterate through available custom content
//   - IsDryRun to indicate if trigger is in dry-run mode
//   - PostProcess to perform any post-processing after content is consumed
type CustomTriggerHandler interface {
	Iterator[[]byte]
	IsDryRun() bool
	PostProcess(context.Context)
}

// Iterator expose a generic contract to iterate and obtain next available content
type Iterator[T any] interface {
	HasNext(context.Context) bool
	GetNext(context.Context) (T, error)
	Error() error
}

// errorReplayTrigger defines error replay custom trigger structure
type errorReplayTrigger struct {
	Config ReplayTriggerConfig `json:"replayFailedEvents"`
}

// newDefaultErrorTrigger generate a trigger event with default settings
func newDefaultErrorTrigger() errorReplayTrigger {
	return errorReplayTrigger{
		Config: ReplayTriggerConfig{
			Dryrun:          false,
			RemoveOnSuccess: true,
		},
	}
}

// ReplayTriggerConfig defines the configuration for error replay trigger
type ReplayTriggerConfig struct {
	Dryrun          bool `json:"dryrun,omitempty"`
	RemoveOnSuccess bool `json:"removeOnSuccess,omitempty"`
}

// errorEvent defines the structure of the error event stored in S3. Here, only required section is defined
// See https://docs.aws.amazon.com/lambda/latest/dg/invocation-async-retain-records.html#invocation-async-destinations
type errorEvent struct {
	RequestPayload json.RawMessage `json:"requestPayload"`
}

// Implementations

// ErrorReplayTriggerHandler implements CustomTriggerHandler for error replaying.
type ErrorReplayTriggerHandler struct {
	trigger    errorReplayTrigger
	bucket     string
	s3Service  S3Service
	s3Iterator Iterator[*types.Object]

	logger *zap.Logger

	currentKey string
}

func NewErrorReplayTriggerHandler(log *zap.Logger, event []byte, bucketName string, s3Service S3Service) (CustomTriggerHandler, error) {
	decoder := json.NewDecoder(bytes.NewBuffer(event))
	decoder.DisallowUnknownFields()

	trigger := newDefaultErrorTrigger()
	err := decoder.Decode(&trigger)
	if err != nil {
		return nil, fmt.Errorf("unable to parse the error replay event: %w", err)
	}

	iterator := newS3ListIterator(s3Service, bucketName, "")

	return &ErrorReplayTriggerHandler{
		trigger:    trigger,
		bucket:     bucketName,
		s3Service:  s3Service,
		s3Iterator: iterator,
		logger:     log,
	}, nil
}

func (m *ErrorReplayTriggerHandler) IsDryRun() bool {
	return m.trigger.Config.Dryrun
}

func (m *ErrorReplayTriggerHandler) HasNext(ctx context.Context) bool {
	return m.s3Iterator.HasNext(ctx)
}

// GetNext implementation retrieves the S3 content from the error S3 bucket.
func (m *ErrorReplayTriggerHandler) GetNext(ctx context.Context) ([]byte, error) {
	s3Entry, err := m.s3Iterator.GetNext(ctx)
	if err != nil {
		return nil, err
	}

	m.currentKey = *s3Entry.Key

	object, err := m.s3Service.ReadObject(ctx, m.bucket, *s3Entry.Key)
	if err != nil {
		return nil, err
	}

	var event errorEvent
	err = json.Unmarshal(object, &event)
	if err != nil {
		return nil, fmt.Errorf("unable to parse error event content: %w", err)
	}

	return event.RequestPayload, nil
}

func (m *ErrorReplayTriggerHandler) Error() error {
	return m.s3Iterator.Error()
}

func (m *ErrorReplayTriggerHandler) PostProcess(ctx context.Context) {
	if m.currentKey == "" {
		// nothing to do
		return
	}

	defer func() { m.currentKey = "" }()

	// skip if dry-run mode
	if m.IsDryRun() {
		m.logger.Info("Dry-run enabled, skipping object", zap.String("object", m.currentKey))
		return
	}

	if !m.trigger.Config.RemoveOnSuccess {
		m.logger.Info("Removal disabled, skipping object removal", zap.String("object", m.currentKey))
		return
	}

	err := m.s3Service.DeleteObject(ctx, m.bucket, m.currentKey)
	if err != nil {
		m.logger.Error("Removal failed", zap.String("object", m.currentKey), zap.Error(err))
	}
}

// s3ListIterator implements Iterator and isolates S3 object listing for consumers
type s3ListIterator struct {
	s3Service S3Service
	bucket    string
	prefix    string

	latest     *s3.ListObjectsV2Output
	currentObj *types.Object
	index      int
	done       bool

	err error
}

func newS3ListIterator(s3Service S3Service, bucket, prefix string) Iterator[*types.Object] {
	return &s3ListIterator{
		s3Service: s3Service,
		bucket:    bucket,
		prefix:    prefix,
	}
}

func (i *s3ListIterator) HasNext(ctx context.Context) bool {
	if i.done {
		return false
	}

	// check for initial fetch
	if i.latest == nil {
		var err error
		i.latest, err = i.s3Service.ListObjects(ctx, i.bucket, "", i.prefix)
		if err != nil {
			i.done = true
			i.err = err
			return false
		}
	}

	// check for limits and fetch next if available
	if len(i.latest.Contents) <= i.index {
		if !*i.latest.IsTruncated {
			i.done = true
			return false
		}

		var err error
		i.latest, err = i.s3Service.ListObjects(ctx, i.bucket, *i.latest.NextContinuationToken, i.prefix)
		if err != nil {
			i.done = true
			i.err = err
			return false
		}
		i.index = 0
	}

	// set current, advance and return
	i.currentObj = &i.latest.Contents[i.index]
	i.index++
	return true
}

func (i *s3ListIterator) GetNext(_ context.Context) (*types.Object, error) {
	if i.currentObj == nil {
		return nil, errors.New("payload not available, check HasNext before calling for GetNext")
	}

	return i.currentObj, nil
}

func (i *s3ListIterator) Error() error {
	return i.err
}
