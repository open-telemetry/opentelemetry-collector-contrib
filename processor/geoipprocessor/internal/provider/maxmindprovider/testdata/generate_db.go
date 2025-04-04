// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testdata

import (
	"testing"

	"github.com/maxmind/MaxMind-DB/pkg/writer"
)

// GenerateLocalDB generates *.mmdb databases files given a source directory data. It uses the writer functionality provided by MaxMind-Db/pkg/writer
func GenerateLocalDB(t *testing.T, sourceData string) string {
	tmpDir := t.TempDir()

	w, err := writer.New(sourceData, tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	err = w.WriteGeoIP2TestDB()
	if err != nil {
		t.Fatal(err)
	}

	return tmpDir
}
