// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !darwin && !freebsd && !dragonfly && !netbsd && !openbsd && !linux && !solaris && !windows
// +build !darwin,!freebsd,!dragonfly,!netbsd,!openbsd,!linux,!solaris,!windows

package filestatsreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filestatsreceiver"

import (
	"os"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filestatsreceiver/internal/metadata"
)

func collectStats(now pcommon.Timestamp, fileinfo os.FileInfo, metricsBuilder *metadata.MetricsBuilder, logger *zap.Logger) {
	logger.Warn("Cannot collect access and creation time for this arch")
}
