// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"errors"
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
			name: "Valid event",
			event: `{
						"replayFailedEvents" : {}
					}`,
		},
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
	gMockCtr := gomock.NewController(t)

	bucketName := "someBucket"
	objectName := "someObject"
	mockData := []byte(`{"requestPayload":{"Records":"content in S3 trigger event, ignored here"}}`)

	tests := []struct {
		name          string
		trigger       errorReplayTrigger
		s3ServiceFunc func() S3Service
	}{
		{
			name:    "Event handling with defaults - one object end to end",
			trigger: newDefaultErrorTrigger(),
			s3ServiceFunc: func() S3Service {
				s3ServiceMock := NewMockS3Service(gMockCtr)
				s3ServiceMock.EXPECT().ReadObject(gomock.Any(), bucketName, objectName).Return(mockData, nil).Times(1)
				s3ServiceMock.EXPECT().DeleteObject(gomock.Any(), bucketName, objectName).Return(nil).Times(1)

				return s3ServiceMock
			},
		},
		{
			name: "Event handling in dry run mode - No calls for delete",
			trigger: errorReplayTrigger{Config: ReplayTriggerConfig{
				Dryrun: true,
			}},
			s3ServiceFunc: func() S3Service {
				s3ServiceMock := NewMockS3Service(gMockCtr)
				s3ServiceMock.EXPECT().ReadObject(gomock.Any(), bucketName, objectName).Return(mockData, nil).Times(1)

				// In dry run mode, DeleteObject should not be called
				s3ServiceMock.EXPECT().DeleteObject(gomock.Any(), bucketName, objectName).Return(nil).Times(0)

				return s3ServiceMock
			},
		},
		{
			name: "Event handling in disabled remove on success - No calls for delete",
			trigger: errorReplayTrigger{Config: ReplayTriggerConfig{
				Dryrun: true,
			}},
			s3ServiceFunc: func() S3Service {
				s3ServiceMock := NewMockS3Service(gMockCtr)
				s3ServiceMock.EXPECT().ReadObject(gomock.Any(), bucketName, objectName).Return(mockData, nil).Times(1)

				// With disabled delete on success, DeleteObject should not be called
				s3ServiceMock.EXPECT().DeleteObject(gomock.Any(), bucketName, objectName).Return(nil).Times(0)

				return s3ServiceMock
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			iterator := NewMockIterator[*types.Object](gMockCtr)
			iterator.EXPECT().HasNext(gomock.Any()).Return(true).Times(1)
			iterator.EXPECT().HasNext(gomock.Any()).Return(false).Times(1)
			iterator.EXPECT().GetNext(gomock.Any()).Return(&types.Object{Key: &objectName}, nil).Times(1)

			handler := ErrorReplayTriggerHandler{
				trigger:    tt.trigger,
				bucket:     bucketName,
				s3Service:  tt.s3ServiceFunc(),
				s3Iterator: iterator,
				logger:     zap.NewNop(),
			}

			require.True(t, handler.HasNext(t.Context()))
			content, err := handler.GetNext(t.Context())
			require.NoError(t, err)
			require.Equal(t, `{"Records":"content in S3 trigger event, ignored here"}`, string(content))

			handler.PostProcess(t.Context())
			require.False(t, handler.HasNext(t.Context()))
		})
	}
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
		require.ErrorContains(t, err, "payload not available")
		require.Nil(t, next)

		// next should return true
		require.True(t, iterator.HasNext(t.Context()))

		// next fetch should return correct values
		next, err = iterator.GetNext(t.Context())
		require.NoError(t, err)
		require.Equal(t, *next, cts[0])

		// next should be false going forward
		require.False(t, iterator.HasNext(t.Context()))
		require.False(t, iterator.HasNext(t.Context()))
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
		require.True(t, iterator.HasNext(t.Context()))
		next, err := iterator.GetNext(t.Context())
		require.NoError(t, err)
		require.Equal(t, *next, cts[0])

		// validate entry B
		require.True(t, iterator.HasNext(t.Context()))
		next, err = iterator.GetNext(t.Context())
		require.NoError(t, err)
		require.Equal(t, *next, cts[1])

		require.False(t, iterator.HasNext(t.Context()))
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
		require.True(t, iterator.HasNext(t.Context()))
		next, err := iterator.GetNext(t.Context())
		require.NoError(t, err)
		require.Equal(t, *next, ctsA[0])

		// validate list B entry B
		require.True(t, iterator.HasNext(t.Context()))
		next, err = iterator.GetNext(t.Context())
		require.NoError(t, err)
		require.Equal(t, *next, ctsB[0])

		require.False(t, iterator.HasNext(t.Context()))
	})

	t.Run("Iterate with listing error - error preserved and exposed for caller", func(t *testing.T) {
		gc := gomock.NewController(t)
		s3ServiceMock := NewMockS3Service(gc)

		errorText := "awslambdareceiver: error listing objects"
		bucketName := "bucket"
		prefix := "prefix"

		// Mock listing with error
		s3ServiceMock.EXPECT().
			ListObjects(gomock.Any(), bucketName, gomock.Any(), prefix).
			Return(&s3.ListObjectsV2Output{}, errors.New(errorText)).
			Times(1)

		iterator := newS3ListIterator(s3ServiceMock, bucketName, prefix)

		// validate iterator calls
		require.False(t, iterator.HasNext(t.Context()))
		require.EqualError(t, iterator.Error(), errorText)
	})
}
