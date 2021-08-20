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

package system

import (
	"os"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metadata/valid"
)

type HostInfo struct {
	OS   string
	FQDN string
}

// GetHostInfo gets system information about the hostname
func GetHostInfo(logger *zap.Logger) (hostInfo *HostInfo) {
	hostInfo = &HostInfo{}

	if hostname, err := getSystemFQDN(); err == nil {
		hostInfo.FQDN = hostname
	} else {
		logger.Warn("Could not get FQDN Hostname", zap.Error(err))
	}

	if hostname, err := os.Hostname(); err == nil {
		hostInfo.OS = hostname
	} else {
		logger.Warn("Could not get OS Hostname", zap.Error(err))
	}

	return
}

// GetHostname gets the hostname provided by the system
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
