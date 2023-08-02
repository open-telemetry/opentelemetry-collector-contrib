// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package filestatsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filestatsreceiver"

import (
	"os"
	"syscall"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filestatsreceiver/internal/metadata"
)

func collectStats(now pcommon.Timestamp, fileinfo os.FileInfo, metricsBuilder *metadata.MetricsBuilder, logger *zap.Logger) {
	stat := fileinfo.Sys().(*syscall.Win32FileAttributeData)
	atime := stat.LastAccessTime.Nanoseconds() / int64(time.Second)
	ctime := stat.LastWriteTime.Nanoseconds() / int64(time.Second)
	metricsBuilder.RecordFileAtimeDataPoint(now, atime)
	metricsBuilder.RecordFileCtimeDataPoint(now, ctime, fileinfo.Mode().Perm().String())
}
