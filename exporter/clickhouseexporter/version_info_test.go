// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter

// testCollectorVersionResolver will return a constant value for the collector version.
type testCollectorVersionResolver struct {
	version string
}

func newTestCollectorVersionResolver(version string) *testCollectorVersionResolver {
	return &testCollectorVersionResolver{version: version}
}

func newDefaultTestCollectorVersionResolver() *testCollectorVersionResolver {
	return &testCollectorVersionResolver{version: "test"}
}

func (r *testCollectorVersionResolver) GetVersion() string {
	return r.version
}
