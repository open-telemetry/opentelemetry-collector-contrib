// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3receiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver"

import (
	"context"
)

// s3ObjectCallback is a function that processes a single S3 object content
type s3ObjectCallback func(ctx context.Context, key string, content []byte) error

// s3Reader defines a common interface for components that read from S3
type s3Reader interface {
	// readAll processes all S3 objects matching specified criteria and passes content to callback
	readAll(ctx context.Context, telemetryType string, callback s3ObjectCallback) error
}
