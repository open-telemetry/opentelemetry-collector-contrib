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

// CustomEventHandler defines the generic contract for custom event handling
type CustomEventHandler[T any] interface {
	Iterator[T]
	IsDryRun() bool
	PostProcess(ctx context.Context, on T)
}

// Iterator expose a generic contract to iterate and obtain next available object
type Iterator[T any] interface {
	HasNext() bool
	GetNext(ctx context.Context) (T, error)
}

type errorReplayTrigger struct {
	Payload ReplayTrigger `json:"replayFailedEvents"`
}

// newDefaultErrorTrigger generate a trigger event with default settings
func newDefaultErrorTrigger() errorReplayTrigger {
	return errorReplayTrigger{
		Payload: ReplayTrigger{
			Dryrun:          false,
			RemoveOnSuccess: true,
		},
	}
}

type ReplayTrigger struct {
	Dryrun          bool `json:"dryrun,omitempty"`
	RemoveOnSuccess bool `json:"removeOnSuccess,omitempty"`
}

type errorEvent struct {
	RequestPayload json.RawMessage `json:"requestPayload"`
}

type ErrorReplayResponse struct {
	Content []byte
	Key     string
}

// Implementations

// ErrorReplayTriggerHandler implements Iterator for error replaying
type ErrorReplayTriggerHandler struct {
	trigger    errorReplayTrigger
	bucket     string
	s3Service  S3Service
	s3Iterator Iterator[*types.Object]

	logger *zap.Logger
}

func NewErrorReplayTriggerHandler(log *zap.Logger, event []byte, bucketName string, s3Service S3Service) (CustomEventHandler[*ErrorReplayResponse], error) {
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
	return m.trigger.Payload.Dryrun
}

func (m *ErrorReplayTriggerHandler) HasNext() bool {
	return m.s3Iterator.HasNext()
}

func (m *ErrorReplayTriggerHandler) GetNext(ctx context.Context) (*ErrorReplayResponse, error) {
	got, err := m.s3Iterator.GetNext(ctx)
	if err != nil {
		return nil, err
	}

	// check if this is a dryrun, if so skip download and send known details
	if m.IsDryRun() {
		return &ErrorReplayResponse{
			Content: nil,
			Key:     *got.Key,
		}, nil
	}

	object, err := m.s3Service.ReadObject(ctx, m.bucket, *got.Key)
	if err != nil {
		return nil, err
	}

	var event errorEvent
	err = json.Unmarshal(object, &event)
	if err != nil {
		return nil, fmt.Errorf("unable to parse error event content: %w", err)
	}

	return &ErrorReplayResponse{
		Content: event.RequestPayload,
		Key:     *got.Key,
	}, nil
}

func (m *ErrorReplayTriggerHandler) PostProcess(ctx context.Context, rsp *ErrorReplayResponse) {
	if m.IsDryRun() {
		// No processing for dry run. Avoid logging to reduce verbosity
		return
	}

	if !m.trigger.Payload.RemoveOnSuccess {
		m.logger.Info(fmt.Sprintf("Removal disabled, skipping object removal: %s", rsp.Key))
		return
	}

	err := m.s3Service.DeleteObject(ctx, m.bucket, rsp.Key)
	if err != nil {
		m.logger.Error("Removal failed", zap.String("object", rsp.Key), zap.String("cause", err.Error()))
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
}

func newS3ListIterator(s3Service S3Service, bucket, prefix string) Iterator[*types.Object] {
	return &s3ListIterator{
		s3Service: s3Service,
		bucket:    bucket,
		prefix:    prefix,
	}
}

func (i *s3ListIterator) HasNext() bool {
	if i.done {
		return false
	}

	// check for initial fetch
	if i.latest == nil {
		var err error
		i.latest, err = i.s3Service.ListObjects(context.Background(), i.bucket, "", i.prefix)
		if err != nil {
			i.done = true
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
		i.latest, err = i.s3Service.ListObjects(context.Background(), i.bucket, *i.latest.NextContinuationToken, i.prefix)
		if err != nil {
			i.done = true
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
		return nil, errors.New("payload not available")
	}

	return i.currentObj, nil
}
