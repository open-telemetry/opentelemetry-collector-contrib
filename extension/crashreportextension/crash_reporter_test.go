// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package crashreportextension

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"
)

func TestCrashReporter_Shutdown(t *testing.T) {
	reporter := &crashReportExtension{}
	err := reporter.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestCrashReporter_StartShutdownPanic(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.HTTPClientSettings.Endpoint = "https://example.com"
	reporter := &crashReportExtension{
		cfg:    cfg,
		logger: zap.NewNop(),
	}
	err := reporter.Start(context.Background(), componenttest.NewNopHost())
	assert.NoError(t, err)
	err = reporter.Shutdown(context.Background())
	assert.NoError(t, err)
	reporter.handlePanic(struct{}{})
}
