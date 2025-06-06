// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !integration

package clickhouseexporter

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
