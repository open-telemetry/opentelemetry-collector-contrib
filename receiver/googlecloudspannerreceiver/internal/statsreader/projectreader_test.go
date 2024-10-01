// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package statsreader

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"
)

type mockCompositeReader struct {
	mock.Mock
}

func (r *mockCompositeReader) Name() string {
	return "mockCompositeReader"
}

func (r *mockCompositeReader) Read(ctx context.Context) ([]*metadata.MetricsDataPoint, error) {
	args := r.Called(ctx)
	return args.Get(0).([]*metadata.MetricsDataPoint), args.Error(1)
}

func (r *mockCompositeReader) Shutdown() {
	// Do nothing
}

func TestNewProjectReader(t *testing.T) {
	logger := zaptest.NewLogger(t)
	var databaseReaders []CompositeReader

	reader := NewProjectReader(databaseReaders, logger)
	defer reader.Shutdown()

	assert.NotNil(t, reader)
	assert.Equal(t, logger, reader.logger)
	assert.Equal(t, databaseReaders, reader.databaseReaders)
}

func TestProjectReader_Shutdown(t *testing.T) {
	logger := zaptest.NewLogger(t)

	databaseReaders := []CompositeReader{&mockCompositeReader{}}

	reader := ProjectReader{
		databaseReaders: databaseReaders,
		logger:          logger,
	}

	reader.Shutdown()
}

func TestProjectReader_Read(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)
	testCases := map[string]struct {
		expectedError error
	}{
		"Happy path":     {nil},
		"Error occurred": {errors.New("read error")},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			compositeReader := &mockCompositeReader{}
			databaseReaders := []CompositeReader{compositeReader}
			reader := ProjectReader{
				databaseReaders: databaseReaders,
				logger:          logger,
			}
			defer reader.Shutdown()

			compositeReader.On("Read", ctx).Return([]*metadata.MetricsDataPoint{}, testCase.expectedError)

			_, err := reader.Read(ctx)

			compositeReader.AssertExpectations(t)

			if testCase.expectedError != nil {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestProjectReader_Name(t *testing.T) {
	logger := zaptest.NewLogger(t)

	databaseReader := &mockCompositeReader{}
	databaseReaders := []CompositeReader{databaseReader}

	reader := ProjectReader{
		databaseReaders: databaseReaders,
		logger:          logger,
	}
	defer reader.Shutdown()

	name := reader.Name()

	assert.Equal(t, "Project reader for: "+databaseReader.Name(), name)
}
