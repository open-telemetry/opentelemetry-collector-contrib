// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux || solaris
// +build linux solaris

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
	atime := stat.Atim.Sec
	ctime := stat.Ctim.Sec
	//nolint
	metricsBuilder.RecordFileAtimeDataPoint(now, int64(atime))
	//nolint
	metricsBuilder.RecordFileCtimeDataPoint(now, int64(ctime), fileinfo.Mode().Perm().String())
}
