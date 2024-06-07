// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsapplicationsignalsprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProcessorShutdown(t *testing.T) {
	// Dummy test method
	appsingalsProcessor := &awsapplicationsignalsprocessor{}
	assert.Nil(t, appsingalsProcessor.Shutdown(context.TODO()))
}
