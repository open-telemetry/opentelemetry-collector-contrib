// Copyright The OpenTelemetry Authors
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

//go:build darwin || freebsd || dragonfly || netbsd || openbsd
// +build darwin freebsd dragonfly netbsd openbsd

package filestatsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filestatsreceiver"

import (
	"os"
	"syscall"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filestatsreceiver/internal/metadata"
)

func collectStats(now pcommon.Timestamp, fileinfo os.FileInfo, metricsBuilder *metadata.MetricsBuilder, logger *zap.Logger) {
	stat := fileinfo.Sys().(*syscall.Stat_t)
	atime := stat.Atimespec.Sec
	ctime := stat.Ctimespec.Sec
	metricsBuilder.RecordFileAtimeDataPoint(now, atime)
	metricsBuilder.RecordFileCtimeDataPoint(now, ctime, fileinfo.Mode().Perm().String())
}
