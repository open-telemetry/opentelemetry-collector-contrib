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
// +build !windows

package system

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/metadata/valid"
)

const hostnamePath = "/bin/hostname"

func getSystemFQDN() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*1)
	defer cancel()

	// Go does not provide a way to get the full hostname
	// so we make a best-effort by running the hostname binary
	// if available
	if _, err := os.Stat(hostnamePath); err == nil {
		cmd := exec.CommandContext(ctx, hostnamePath, "-f")
		out, err := cmd.Output()
		return strings.TrimSpace(string(out)), err
	}

	// if stat failed for any reason, fail silently
	return "", nil
}

func (hi *HostInfo) GetHostname(logger *zap.Logger) string {
	if hi.FQDN == "" {
		// Don't report failure since FQDN was just not available
		return hi.OS
	} else if err := valid.Hostname(hi.FQDN); err != nil {
		logger.Info("FQDN is not valid", zap.Error(err))
		return hi.OS
	}

	return hi.FQDN
}
