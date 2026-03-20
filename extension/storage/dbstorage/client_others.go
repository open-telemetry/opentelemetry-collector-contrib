// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// aix does not support modernc
//go:build !aix

package dbstorage // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/dbstorage"

import (
	_ "modernc.org/sqlite" // SQLite driver
)
