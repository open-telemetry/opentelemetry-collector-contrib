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

package crashreportextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/crashreportextension"
import (
	"context"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

type crashReportExtension struct {
	cfg               *Config
	httpClient        *http.Client
	logger            *zap.Logger
	telemetrySettings component.TelemetrySettings
	started           bool
}

func (c *crashReportExtension) Start(_ context.Context, host component.Host) error {
	var err error
	c.httpClient, err = c.cfg.HTTPClientSettings.ToClient(host, c.telemetrySettings)
	if err != nil {
		return err
	}
	c.started = true
	defer func() {
		r := recover()
		if r != nil {
			c.handlePanic(r)
			// Make sure the process exits.
			os.Exit(1)
		}
	}()
	return nil
}

func (c *crashReportExtension) handlePanic(fatalErr any) {
	c.logger.Error("caught panic", zap.Any("err", fatalErr))
	if c.started {
		stacktraceBuffer := make([]byte, 2056)
		_ = runtime.Stack(stacktraceBuffer, true)
		msg := fmt.Sprintf("%v\n%s", fatalErr, stacktraceBuffer)
		resp, err := c.httpClient.Post(c.cfg.HTTPClientSettings.Endpoint, "application/octet-stream", strings.NewReader(msg))
		if err != nil {
			c.logger.Error("error sending crash report", zap.Error(err))
		} else if resp.StatusCode < 300 {
			c.logger.Info("crash report sent successfully", zap.Int("code", resp.StatusCode))
		}
	}
}

func (c *crashReportExtension) Shutdown(_ context.Context) error {
	c.started = false
	return nil
}
