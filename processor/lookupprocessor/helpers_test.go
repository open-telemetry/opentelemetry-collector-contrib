// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lookupprocessor

import "go.opentelemetry.io/collector/component"

type mockSourceConfig struct{}

func (*mockSourceConfig) Validate() error { return nil }

type testHost struct{}

func (*testHost) GetExtensions() map[component.ID]component.Component {
	return nil
}
