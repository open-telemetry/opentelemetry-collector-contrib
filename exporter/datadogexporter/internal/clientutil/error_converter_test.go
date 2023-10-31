// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clientutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/clientutil"

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumererror"
)

func TestWrapError(t *testing.T) {
	respOK := http.Response{StatusCode: 200}
	respRetriable := http.Response{StatusCode: 402}
	respNonRetriable := http.Response{StatusCode: 404}
	err := fmt.Errorf("Test error")
	assert.False(t, consumererror.IsPermanent(WrapError(err, &respOK)))
	assert.False(t, consumererror.IsPermanent(WrapError(err, &respRetriable)))
	assert.True(t, consumererror.IsPermanent(WrapError(err, &respNonRetriable)))
	assert.False(t, consumererror.IsPermanent(WrapError(nil, &respNonRetriable)))
	assert.False(t, consumererror.IsPermanent(WrapError(err, nil)))
}
