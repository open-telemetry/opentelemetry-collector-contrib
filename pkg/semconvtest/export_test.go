// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package semconvtest

// ValidateWeaverVersion exposes validateWeaverVersion to the external test
// package. The shared Makefile's integration-test discovery only works with
// a single *integration_test.go file per module root, so the version test
// lives in semconvtest_integration_test.go instead of an internal test file.
var ValidateWeaverVersion = validateWeaverVersion
