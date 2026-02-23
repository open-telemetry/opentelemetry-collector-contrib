// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lookupprocessor

type mockSourceConfig struct{}

func (*mockSourceConfig) Validate() error { return nil }
