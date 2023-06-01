// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package key // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/key"

import (
	"github.com/google/uuid"
)

// Partition allows for switching our partitioning behavior
// when sending data to kinesis.
type Partition func(v interface{}) string

func Randomized(_ interface{}) string {
	return uuid.NewString()
}
