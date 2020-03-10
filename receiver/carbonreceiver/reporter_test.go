// Copyright 2019 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package carbonreceiver

import (
	"context"
	"errors"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector/observability/observabilitytest"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestReporterObservability(t *testing.T) {
	doneFn := observabilitytest.SetupRecordedMetricsTest()
	defer doneFn()

	const receiverName = "fake_carbon_receiver"
	reporter := newReporter(receiverName, zap.NewNop())

	ctx := reporter.OnDataReceived(context.Background())

	reporter.OnMetricsProcessed(ctx, 17, 13, nil)

	err := observabilitytest.CheckValueViewReceiverReceivedTimeSeries(receiverName, 17)
	require.NoError(t, err)

	// Below just exercise the error paths.
	err = errors.New("fake error for tests")
	reporter.OnTranslationError(ctx, err)
	reporter.OnMetricsProcessed(ctx, 10, 10, err)

	err = observabilitytest.CheckValueViewReceiverDroppedTimeSeries(receiverName, 10)
	require.NoError(t, err)
}
