// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// aix and solaris do not support modernc
//go:build !aix && !solaris

package dbstorage // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/dbstorage"

import (
	_ "modernc.org/sqlite" // SQLite driver
)
