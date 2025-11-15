// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
)

func TestErrorReplayTriggerHandler_Constructor(t *testing.T) {
	tests := []struct {
		name    string
		event   string
		wantErr string
	}{
		{
			name:    "Invalid event - empty",
			event:   "",
			wantErr: "unable to parse the error replay event",
		},
		{
			name: "Invalid event - unknown json",
			event: `{
						"customTrigger" : {}
					}`,
			wantErr: "unable to parse the error replay event",
		},
		{
			name: "Valid event",
			event: `{
						"replayFailedEvents" : {}
					}`,
		},
	}

	s3ServiceMock := NewMockS3Service(gomock.NewController(t))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewErrorReplayTriggerHandler(zap.NewNop(), []byte(tt.event), "someBucket", s3ServiceMock)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

func TestErrorReplayTriggerHandler(t *testing.T) {
	t.Run("Event handling with defaults", func(t *testing.T) {
		// setup
		gMockCtr := gomock.NewController(t)
		s3ServiceMock := NewMockS3Service(gMockCtr)
		iterator := NewMockIterator[*types.Object](gMockCtr)

		bucketName := "someBucket"
		objectName := "someObject"

		handler := ErrorReplayTriggerHandler{
			trigger:    newDefaultErrorTrigger(),
			bucket:     bucketName,
			s3Service:  s3ServiceMock,
			s3Iterator: iterator,
			logger:     zap.NewNop(),
		}

		iterator.EXPECT().HasNext().Return(true).Times(1)
		iterator.EXPECT().HasNext().Return(false).Times(1)
		iterator.EXPECT().GetNext(t.Context()).Return(&types.Object{Key: &objectName}, nil).Times(1)

		data := []byte(`{"requestPayload":{"Records":"content in S3 trigger event, ignored here"}}`)
		s3ServiceMock.EXPECT().ReadObject(gomock.Any(), bucketName, objectName).Return(data, nil).Times(1)
		s3ServiceMock.EXPECT().DeleteObject(gomock.Any(), bucketName, objectName).Return(nil).Times(1)

		// validations
		require.False(t, handler.IsDryRun())
		require.True(t, handler.HasNext())

		content, err := handler.GetNext(t.Context())
		require.NoError(t, err)
		require.Equal(t, `{"Records":"content in S3 trigger event, ignored here"}`, string(content.Content))

		handler.PostProcess(t.Context(), content)
		require.False(t, handler.HasNext())
	})

	t.Run("Event handling with DryRun Mode", func(t *testing.T) {
		gMockCtr := gomock.NewController(t)

		bucketName := "someBucket"
		objectName := "someObject"

		s3ServiceMock := NewMockS3Service(gMockCtr)
		s3ServiceMock.EXPECT().ReadObject(gomock.Any(), bucketName, objectName).Times(0)
		s3ServiceMock.EXPECT().DeleteObject(gomock.Any(), bucketName, objectName).Times(0)

		iterator := NewMockIterator[*types.Object](gMockCtr)
		iterator.EXPECT().HasNext().Return(true).Times(1)
		iterator.EXPECT().HasNext().Return(false).Times(1)

		iterator.EXPECT().GetNext(t.Context()).Return(&types.Object{Key: &objectName}, nil).Times(1)

		trigger := newDefaultErrorTrigger()
		trigger.Payload.Dryrun = true

		handler := ErrorReplayTriggerHandler{
			trigger:    trigger,
			bucket:     bucketName,
			s3Service:  s3ServiceMock,
			s3Iterator: iterator,
			logger:     zap.NewNop(),
		}

		// validations
		require.True(t, handler.HasNext())

		content, err := handler.GetNext(t.Context())
		require.NoError(t, err)
		require.Empty(t, string(content.Content))
		require.Equal(t, objectName, content.Key)

		handler.PostProcess(t.Context(), content)
		require.False(t, handler.HasNext())
	})

	t.Run("Event handling with disabled removeOnSuccess", func(t *testing.T) {
		gMockCtr := gomock.NewController(t)

		bucketName := "someBucket"
		objectName := "someObject"
		data := []byte(`{"requestPayload":{"Records":"content in S3 trigger event, ignored here"}}`)
		s3ServiceMock := NewMockS3Service(gMockCtr)
		s3ServiceMock.EXPECT().ReadObject(gomock.Any(), bucketName, objectName).Return(data, nil).Times(1)
		s3ServiceMock.EXPECT().DeleteObject(gomock.Any(), bucketName, objectName).Times(0)

		iterator := NewMockIterator[*types.Object](gMockCtr)
		iterator.EXPECT().HasNext().Return(true).Times(1)
		iterator.EXPECT().HasNext().Return(false).Times(1)

		iterator.EXPECT().GetNext(t.Context()).Return(&types.Object{Key: &objectName}, nil).Times(1)

		trigger := newDefaultErrorTrigger()
		trigger.Payload.RemoveOnSuccess = false

		handler := ErrorReplayTriggerHandler{
			trigger:    trigger,
			bucket:     bucketName,
			s3Service:  s3ServiceMock,
			s3Iterator: iterator,
			logger:     zap.NewNop(),
		}

		// validations
		require.True(t, handler.HasNext())

		content, err := handler.GetNext(t.Context())
		require.NoError(t, err)
		require.Equal(t, `{"Records":"content in S3 trigger event, ignored here"}`, string(content.Content))
		require.Equal(t, objectName, content.Key)

		handler.PostProcess(t.Context(), content)
		require.False(t, handler.HasNext())
	})
}

func TestS3ListIterator(t *testing.T) {
	tb := true
	fb := false

	t.Run("Iterate with one entry", func(t *testing.T) {
		gc := gomock.NewController(t)
		s3ServiceMock := NewMockS3Service(gc)

		bucketName := "bucket"
		prefix := "prefix"
		key := "entryA"

		cts := []types.Object{{Key: &key}}

		// List containing only one page with one object
		s3ServiceMock.EXPECT().
			ListObjects(gomock.Any(), bucketName, gomock.Any(), prefix).
			Return(&s3.ListObjectsV2Output{
				Contents:    cts,
				IsTruncated: &fb,
			}, nil).
			Times(1)

		iterator := newS3ListIterator(s3ServiceMock, bucketName, prefix)

		// first get should fail with error
		next, err := iterator.GetNext(t.Context())
		require.Error(t, err)
		require.Nil(t, next)

		// next should return true
		require.True(t, iterator.HasNext())

		// next fetch should return correct values
		next, err = iterator.GetNext(t.Context())
		require.NoError(t, err)
		require.Equal(t, *next, cts[0])

		// next should be false going forward
		require.False(t, iterator.HasNext())
		require.False(t, iterator.HasNext())
	})

	t.Run("Iterate with multiple entries", func(t *testing.T) {
		gc := gomock.NewController(t)
		s3ServiceMock := NewMockS3Service(gc)

		bucketName := "bucket"
		prefix := "prefix"

		keyA := "entryA"
		keyB := "entryB"

		cts := []types.Object{{Key: &keyA}, {Key: &keyB}}

		// List containing only one page with multiple entries
		s3ServiceMock.EXPECT().
			ListObjects(gomock.Any(), bucketName, gomock.Any(), prefix).
			Return(&s3.ListObjectsV2Output{
				Contents:    cts,
				IsTruncated: &fb,
			}, nil).
			Times(1)

		iterator := newS3ListIterator(s3ServiceMock, bucketName, prefix)

		// validate entry A
		require.True(t, iterator.HasNext())
		next, err := iterator.GetNext(t.Context())
		require.NoError(t, err)
		require.Equal(t, *next, cts[0])

		// validate entry B
		require.True(t, iterator.HasNext())
		next, err = iterator.GetNext(t.Context())
		require.NoError(t, err)
		require.Equal(t, *next, cts[1])

		require.False(t, iterator.HasNext())
	})

	t.Run("Iterate with multiple lists", func(t *testing.T) {
		gc := gomock.NewController(t)
		s3ServiceMock := NewMockS3Service(gc)

		bucketName := "bucket"
		prefix := "prefix"
		token := "token"

		keyA := "entryA"
		keyB := "entryB"

		ctsA := []types.Object{{Key: &keyA}}
		ctsB := []types.Object{{Key: &keyB}}

		// Mock multiple listings
		s3ServiceMock.EXPECT().
			ListObjects(gomock.Any(), bucketName, gomock.Any(), prefix).
			Return(&s3.ListObjectsV2Output{
				Contents:              ctsA,
				IsTruncated:           &tb,
				NextContinuationToken: &token,
			}, nil).
			Times(1)

		s3ServiceMock.EXPECT().
			ListObjects(gomock.Any(), bucketName, token, prefix).
			Return(&s3.ListObjectsV2Output{
				Contents:    ctsB,
				IsTruncated: &fb,
			}, nil).
			Times(1)

		iterator := newS3ListIterator(s3ServiceMock, bucketName, prefix)

		// validate list A entry A
		require.True(t, iterator.HasNext())
		next, err := iterator.GetNext(t.Context())
		require.NoError(t, err)
		require.Equal(t, *next, ctsA[0])

		// validate list B entry B
		require.True(t, iterator.HasNext())
		next, err = iterator.GetNext(t.Context())
		require.NoError(t, err)
		require.Equal(t, *next, ctsB[0])

		require.False(t, iterator.HasNext())
	})
}
