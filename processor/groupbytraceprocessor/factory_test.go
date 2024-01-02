// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupbytraceprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/processor/processortest"
)

func TestDefaultConfiguration(t *testing.T) {
	// test
	c := createDefaultConfig().(*Config)

	// verify
	assert.Equal(t, defaultNumTraces, c.NumTraces)
	assert.Equal(t, defaultNumWorkers, c.NumWorkers)
	assert.Equal(t, defaultWaitDuration, c.WaitDuration)
	assert.Equal(t, defaultDiscardOrphans, c.DiscardOrphans)
	assert.Equal(t, defaultStoreOnDisk, c.StoreOnDisk)
}

func TestCreateTestProcessor(t *testing.T) {
	c := createDefaultConfig().(*Config)

	next := &mockProcessor{}

	// test
	p, err := createTracesProcessor(context.Background(), processortest.NewNopCreateSettings(), c, next)

	// verify
	assert.NoError(t, err)
	assert.NotNil(t, p)
}

func TestCreateTestProcessorWithNotImplementedOptions(t *testing.T) {
	// prepare
	f := NewFactory()
	next := &mockProcessor{}

	// test
	for _, tt := range []struct {
		config      *Config
		expectedErr error
	}{
		{
			&Config{
				DiscardOrphans: true,
			},
			errDiscardOrphansNotSupported,
		},
		{
			&Config{
				StoreOnDisk: true,
			},
			errDiskStorageNotSupported,
		},
	} {
		p, err := f.CreateTracesProcessor(context.Background(), processortest.NewNopCreateSettings(), tt.config, next)

		// verify
		assert.Error(t, tt.expectedErr, err)
		assert.Nil(t, p)
	}
}
