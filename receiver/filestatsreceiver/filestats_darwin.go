// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
