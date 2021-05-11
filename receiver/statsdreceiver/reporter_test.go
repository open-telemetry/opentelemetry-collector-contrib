// Copyright 2020, OpenTelemetry Authors
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

package statsdreceiver

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/obsreport/obsreporttest"
	"go.uber.org/zap"
)

func TestReporterObservability(t *testing.T) {
	doneFn, err := obsreporttest.SetupRecordedMetricsTest()
	require.NoError(t, err)
	defer doneFn()

	receiverID := config.NewIDWithName(typeStr, "fake_receiver")
	reporter := newReporter(receiverID, zap.NewNop())

	ctx := reporter.OnDataReceived(context.Background())

	reporter.OnMetricsProcessed(ctx, 17, nil)

	obsreporttest.CheckReceiverMetrics(t, receiverID, "tcp", 17, 0)

	// Below just exercise the error paths.
	err = errors.New("fake error for tests")
	reporter.OnTranslationError(ctx, err)
	reporter.OnMetricsProcessed(ctx, 10, err)

	obsreporttest.CheckReceiverMetrics(t, receiverID, "tcp", 17, 10)
}
