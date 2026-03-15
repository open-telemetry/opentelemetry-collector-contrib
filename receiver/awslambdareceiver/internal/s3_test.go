// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.uber.org/mock/gomock"
)

func TestS3ServiceProvider(t *testing.T) {
	provider := S3ServiceProvider{}

	service, err := provider.GetService(t.Context())
	require.NoError(t, err)
	require.NotNil(t, service)
}

func TestS3ServiceReadObject(t *testing.T) {
	gCtr := gomock.NewController(t)

	t.Run("Successful read", func(t *testing.T) {
		bucketName := "bucket"
		objectKey := "object"

		reader := bytes.NewReader([]byte("content"))
		readerCloser := io.NopCloser(reader)

		input := &s3.GetObjectInput{Bucket: &bucketName, Key: &objectKey}
		output := &s3.GetObjectOutput{Body: readerCloser}

		mockAPI := NewMocks3API(gCtr)
		mockAPI.EXPECT().GetObject(gomock.Any(), gomock.Eq(input), gomock.Any()).Return(output, nil).Times(1)

		client := s3ServiceClient{api: mockAPI}

		object, err := client.ReadObject(t.Context(), bucketName, objectKey)
		require.NoError(t, err)

		require.Equal(t, "content", string(object))
	})

	t.Run("Body read errors returns a retryable error", func(t *testing.T) {
		bucketName := "bucket"
		objectKey := "object"

		input := &s3.GetObjectInput{Bucket: &bucketName, Key: &objectKey}
		output := &s3.GetObjectOutput{Body: mockReaderCloser{err: errors.New("some network error")}}

		mockAPI := NewMocks3API(gCtr)
		mockAPI.EXPECT().GetObject(gomock.Any(), gomock.Eq(input), gomock.Any()).Return(output, nil).Times(1)

		client := s3ServiceClient{api: mockAPI}
		_, err := client.ReadObject(t.Context(), bucketName, objectKey)
		require.Error(t, err)
		var consumerErr *consumererror.Error
		require.ErrorAs(t, err, &consumerErr)
	})
}

func TestS3ServiceListObject(t *testing.T) {
	gCtr := gomock.NewController(t)

	t.Run("Successful listing - bucket name", func(t *testing.T) {
		bucketName := "bucket"

		input := &s3.ListObjectsV2Input{Bucket: &bucketName}
		output := &s3.ListObjectsV2Output{Contents: []types.Object{}}

		mockAPI := NewMocks3API(gCtr)
		mockAPI.EXPECT().ListObjectsV2(gomock.Any(), gomock.Eq(input)).Return(output, nil).Times(1)

		client := s3ServiceClient{api: mockAPI}
		out, err := client.ListObjects(t.Context(), bucketName, "", "")
		require.NoError(t, err)
		require.Equal(t, output, out)
	})

	t.Run("Successful listing - bucket name with continuation token", func(t *testing.T) {
		bucketName := "bucket"
		token := "token"

		input := &s3.ListObjectsV2Input{Bucket: &bucketName, ContinuationToken: &token}
		output := &s3.ListObjectsV2Output{Contents: []types.Object{}}

		mockAPI := NewMocks3API(gCtr)
		mockAPI.EXPECT().ListObjectsV2(gomock.Any(), gomock.Eq(input)).Return(output, nil).Times(1)

		client := s3ServiceClient{api: mockAPI}
		out, err := client.ListObjects(t.Context(), bucketName, token, "")
		require.NoError(t, err)
		require.Equal(t, output, out)
	})
}

func TestS3ServiceDeleteObject(t *testing.T) {
	gCtr := gomock.NewController(t)

	t.Run("Successful delete", func(t *testing.T) {
		bucketName := "bucket"
		objectKey := "object"

		input := &s3.DeleteObjectInput{Bucket: &bucketName, Key: &objectKey}
		output := &s3.DeleteObjectOutput{}

		mockAPI := NewMocks3API(gCtr)
		mockAPI.EXPECT().DeleteObject(gomock.Any(), gomock.Eq(input)).Return(output, nil).Times(1)

		client := s3ServiceClient{api: mockAPI}
		err := client.DeleteObject(t.Context(), bucketName, objectKey)
		require.NoError(t, err)
	})
}

type mockReaderCloser struct {
	err error
}

func (m mockReaderCloser) Close() error {
	return m.err
}

func (m mockReaderCloser) Read(p []byte) (n int, err error) {
	return len(p), m.err
}

func (m mockReaderCloser) Write(p []byte) (n int, err error) {
	return len(p), m.err
}
