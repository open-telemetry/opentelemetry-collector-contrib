// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver"

import (
	bytes "bytes"
	context "context"

	mock "github.com/stretchr/testify/mock"
)

type mockBlobClient struct {
	mock.Mock
}

// ReadBlob provides a mock function with given fields: ctx, containerName, blobName
func (_m *mockBlobClient) readBlob(ctx context.Context, containerName string, blobName string) (*bytes.Buffer, error) {
	ret := _m.Called(ctx, containerName, blobName)

	var r0 *bytes.Buffer
	if rf, ok := ret.Get(0).(func(context.Context, string, string) *bytes.Buffer); ok {
		r0 = rf(ctx, containerName, blobName)
	} else if ret.Get(0) != nil {
		r0 = ret.Get(0).(*bytes.Buffer)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, containerName, blobName)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

func newMockBlobClient() *mockBlobClient {
	blobClient := &mockBlobClient{}
	blobClient.On("readBlob", mock.Anything, mock.Anything, mock.Anything).Return(&bytes.Buffer{}, nil)
	return blobClient
}
