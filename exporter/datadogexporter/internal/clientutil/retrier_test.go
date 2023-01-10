// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package clientutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/clientutil"

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/scrub"
)

func TestDoWithRetries(t *testing.T) {
	scrubber := scrub.NewScrubber()
	retrier := NewRetrier(zap.NewNop(), exporterhelper.NewDefaultRetrySettings(), scrubber)
	ctx := context.Background()

	retryNum, err := retrier.DoWithRetries(ctx, func(context.Context) error { return nil })
	require.NoError(t, err)
	assert.Equal(t, retryNum, int64(0))

	retrier = NewRetrier(zap.NewNop(),
		exporterhelper.RetrySettings{
			Enabled:         true,
			InitialInterval: 5 * time.Millisecond,
			MaxInterval:     30 * time.Millisecond,
			MaxElapsedTime:  100 * time.Millisecond,
		},
		scrubber,
	)
	retryNum, err = retrier.DoWithRetries(ctx, func(context.Context) error { return errors.New("action failed") })
	require.Error(t, err)
	assert.Greater(t, retryNum, int64(0))
}

func TestNoRetriesOnPermanentError(t *testing.T) {
	scrubber := scrub.NewScrubber()
	retrier := NewRetrier(zap.NewNop(), exporterhelper.NewDefaultRetrySettings(), scrubber)
	ctx := context.Background()
	respNonRetriable := http.Response{StatusCode: 404}

	retryNum, err := retrier.DoWithRetries(ctx, func(context.Context) error {
		return WrapError(fmt.Errorf("test"), &respNonRetriable)
	})
	require.Error(t, err)
	assert.Equal(t, retryNum, int64(0))
}
