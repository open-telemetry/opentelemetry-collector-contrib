// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package race exists so that it can be possible to check if the
// race detector has been added into the build to allow for tests
// that provide no unit test value but perform concurrent actions
// where the race detector can identify issues.
package race // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor/internal/race"
